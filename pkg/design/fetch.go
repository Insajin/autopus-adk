package design

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// @AX:NOTE [AUTO]: Network timeout and redirect cap below are intentionally conservative for untrusted design imports.
const fetchTimeout = 10 * time.Second

// @AX:WARN [AUTO]: SSRF defense boundary with high branch count; URL, DNS, IP, and hostname checks must stay aligned.
// @AX:REASON: External design imports can target attacker-controlled URLs; weakening one branch may expose local network resources.
func ValidateExternalURL(ctx context.Context, rawURL string, resolver Resolver) *Diagnostic {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme == "" {
		return &Diagnostic{Path: rawURL, Category: CategoryUnsafeScheme, Message: "invalid or empty URL scheme"}
	}
	if parsed.Scheme != "https" {
		return &Diagnostic{Path: rawURL, Category: CategoryUnsafeScheme, Message: "only https URLs are allowed"}
	}
	host := parsed.Hostname()
	if host == "" {
		return &Diagnostic{Path: rawURL, Category: CategoryLocalHostname, Message: "URL host is required"}
	}
	if strings.EqualFold(strings.TrimSuffix(host, "."), "localhost") {
		return &Diagnostic{Path: rawURL, Category: CategoryPrivateAddress, Message: "localhost is not allowed"}
	}
	if isLocalOnlyHostname(host) {
		return &Diagnostic{Path: rawURL, Category: CategoryLocalHostname, Message: "local-only hostnames are not allowed"}
	}
	if ip := net.ParseIP(host); ip != nil {
		if isIPv4MappedHost(host, ip) {
			return &Diagnostic{Path: rawURL, Category: CategoryPrivateAddress, Message: "IPv4-mapped IPv6 addresses are not allowed"}
		}
		if isBlockedIP(ip) {
			return &Diagnostic{Path: rawURL, Category: CategoryPrivateAddress, Message: "blocked IP address"}
		}
		return nil
	}
	if resolver == nil {
		resolver = func(ctx context.Context, host string) ([]net.IP, error) {
			return net.DefaultResolver.LookupIP(ctx, "ip", host)
		}
	}
	ips, err := resolver(ctx, host)
	if err != nil {
		return &Diagnostic{Path: rawURL, Category: CategoryPrivateAddress, Message: "DNS validation failed"}
	}
	if len(ips) == 0 {
		return &Diagnostic{Path: rawURL, Category: CategoryPrivateAddress, Message: "DNS returned no addresses"}
	}
	for _, ip := range ips {
		if isBlockedIP(ip) {
			return &Diagnostic{Path: rawURL, Category: CategoryPrivateAddress, Message: "DNS resolved to blocked address"}
		}
	}
	return nil
}

// @AX:WARN [AUTO]: Manual HTTPS fetch loop combines redirect validation, status handling, and body-size limits.
// @AX:REASON: Each redirect must be revalidated before fetching; future edits can accidentally bypass the SSRF and size guards.
func fetchPublicHTTPS(ctx context.Context, rawURL string, opts ImportOptions) ([]byte, []string, error) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	current := rawURL
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{
			Transport: safePublicTransport(opts.Resolver),
			Timeout:   fetchTimeout,
		}
	}
	transport := client.Transport
	if transport == nil {
		transport = safePublicTransport(opts.Resolver)
	}
	for redirects := 0; ; redirects++ {
		if diag := ValidateExternalURL(ctx, current, opts.Resolver); diag != nil {
			return nil, []string{string(diag.Category)}, nil
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, current, nil)
		if err != nil {
			return nil, []string{string(CategoryFetchRejected)}, nil
		}
		resp, err := transport.RoundTrip(req)
		if err != nil {
			return nil, []string{"fetch_failed"}, nil
		}
		defer resp.Body.Close()
		if isRedirect(resp.StatusCode) {
			if redirects >= 3 {
				return nil, []string{"redirect_limit"}, nil
			}
			next, err := resolveRedirect(current, resp.Header.Get("Location"))
			if err != nil {
				return nil, []string{"unsafe_redirect"}, nil
			}
			current = next
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, []string{fmt.Sprintf("http_status_%d", resp.StatusCode)}, nil
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, MaxImportBodyBytes+1))
		if err != nil {
			return nil, []string{"read_failed"}, nil
		}
		if len(body) > MaxImportBodyBytes {
			return nil, []string{"body_too_large"}, nil
		}
		return body, nil, nil
	}
}

func isRedirect(status int) bool {
	return status == http.StatusMovedPermanently || status == http.StatusFound ||
		status == http.StatusSeeOther || status == http.StatusTemporaryRedirect ||
		status == http.StatusPermanentRedirect
}

func safePublicTransport(resolver Resolver) http.RoundTripper {
	dialer := &net.Dialer{Timeout: fetchTimeout}
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			ips, err := resolveHostForDial(ctx, host, resolver)
			if err != nil {
				return nil, err
			}
			for _, ip := range ips {
				if isBlockedIP(ip) {
					return nil, fmt.Errorf("blocked dial address: %s", ip.String())
				}
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
		},
		TLSHandshakeTimeout:   fetchTimeout,
		ResponseHeaderTimeout: fetchTimeout,
		ExpectContinueTimeout: time.Second,
	}
}

func resolveHostForDial(ctx context.Context, host string, resolver Resolver) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		if isIPv4MappedHost(host, ip) {
			return nil, fmt.Errorf("blocked IPv4-mapped address: %s", host)
		}
		return []net.IP{ip}, nil
	}
	if resolver == nil {
		resolver = func(ctx context.Context, host string) ([]net.IP, error) {
			return net.DefaultResolver.LookupIP(ctx, "ip", host)
		}
	}
	ips, err := resolver(ctx, host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no DNS addresses for %s", host)
	}
	return ips, nil
}

func resolveRedirect(current, location string) (string, error) {
	if strings.TrimSpace(location) == "" {
		return "", fmt.Errorf("empty redirect")
	}
	base, err := url.Parse(current)
	if err != nil {
		return "", err
	}
	next, err := url.Parse(location)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(next).String(), nil
}

func isLocalOnlyHostname(host string) bool {
	lower := strings.ToLower(strings.TrimSuffix(host, "."))
	return lower == "localhost" || !strings.Contains(lower, ".")
}

func isIPv4MappedHost(host string, ip net.IP) bool {
	return strings.Contains(host, ":") && ip.To4() != nil
}

func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() ||
		isUniqueLocal(ip) || isSpecialPurposeIP(ip)
}

func isUniqueLocal(ip net.IP) bool {
	ip16 := ip.To16()
	return ip16 != nil && (ip16[0]&0xfe) == 0xfc
}

func isSpecialPurposeIP(ip net.IP) bool {
	ip4 := ip.To4()
	for _, cidr := range blockedSpecialCIDRs {
		_, bits := cidr.Mask.Size()
		cidrIsIPv4 := bits == 32
		if ip4 != nil && cidrIsIPv4 && cidr.Contains(ip4) {
			return true
		}
		if ip4 == nil && !cidrIsIPv4 && cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// @AX:NOTE [AUTO]: Special-purpose CIDR deny-list is security policy; update alongside URL validation tests when IANA ranges change.
var blockedSpecialCIDRs = mustParseCIDRs([]string{
	"0.0.0.0/8",
	"10.0.0.0/8",
	"100.64.0.0/10",
	"127.0.0.0/8",
	"169.254.0.0/16",
	"172.16.0.0/12",
	"192.0.0.0/24",
	"192.0.2.0/24",
	"192.31.196.0/24",
	"192.52.193.0/24",
	"192.88.99.0/24",
	"192.168.0.0/16",
	"192.175.48.0/24",
	"198.18.0.0/15",
	"198.51.100.0/24",
	"203.0.113.0/24",
	"240.0.0.0/4",
	"255.255.255.255/32",
	"::/128",
	"::1/128",
	"::ffff:0:0/96",
	"64:ff9b::/96",
	"64:ff9b:1::/48",
	"100::/64",
	"100:0:0:1::/64",
	"2001::/23",
	"2001:db8::/32",
	"2002::/16",
	"2620:4f:8000::/48",
	"3fff::/20",
	"5f00::/16",
	"fc00::/7",
	"fe80::/10",
})

func mustParseCIDRs(values []string) []*net.IPNet {
	var cidrs []*net.IPNet
	for _, value := range values {
		_, cidr, err := net.ParseCIDR(value)
		if err != nil {
			panic(err)
		}
		cidrs = append(cidrs, cidr)
	}
	return cidrs
}
