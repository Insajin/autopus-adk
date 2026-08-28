package cli

import (
	"errors"
	"net"
	"net/url"
	"strings"
)

func validatePipelineOMPActiveEndpoint(raw string) (string, error) {
	endpoint, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || endpoint == nil || endpoint.Scheme != "http" || endpoint.User != nil || endpoint.Port() == "" ||
		(endpoint.Path != "" && endpoint.Path != "/") || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return "", errors.New("invalid endpoint")
	}
	ip := net.ParseIP(endpoint.Hostname())
	if ip == nil || !ip.IsLoopback() {
		return "", errors.New("invalid endpoint")
	}
	return strings.TrimSuffix(endpoint.String(), "/"), nil
}

func pipelineOMPEnvironmentValue(environment []string, key string) (string, bool) {
	for _, entry := range environment {
		name, value, ok := strings.Cut(entry, "=")
		if ok && name == key {
			return value, true
		}
	}
	return "", false
}
