package connect

import (
	"net/http"
	"time"
)

var connectHTTPTransport = http.DefaultTransport.(*http.Transport).Clone()

func newHTTPClient(timeout time.Duration) *http.Client {
	transport := connectHTTPTransport.Clone()
	return &http.Client{Transport: transport, Timeout: timeout}
}
