package connect

import (
	"net/http"
	"testing"
)

func TestNewClient_ClonesDefaultTransport(t *testing.T) {
	first := NewClient("first-token")
	firstTransport, ok := first.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("first client transport = %T, want *http.Transport", first.httpClient.Transport)
	}
	defaultTransport := http.DefaultTransport.(*http.Transport)
	if firstTransport == defaultTransport {
		t.Fatal("first client shares http.DefaultTransport")
	}

	second := NewClient("second-token")
	secondTransport, ok := second.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("second client transport = %T, want *http.Transport", second.httpClient.Transport)
	}
	if secondTransport == defaultTransport || secondTransport == firstTransport {
		t.Fatal("clients do not own independent HTTP transports")
	}
}
