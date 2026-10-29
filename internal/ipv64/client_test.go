package ipv64

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetDomainsDecodesSubdomainsResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"subdomains": {
				"example.com": {
					"records": [{"content": "192.0.2.1", "type": "A", "praefix": "www"}]
				}
			},
			"status": "200 OK"
		}`))
	}))
	defer server.Close()

	client := NewClientWithURL(server.URL, "test-key")
	domains, err := client.GetDomains()
	if err != nil {
		t.Fatalf("GetDomains() returned an error: %v", err)
	}

	if len(domains) != 1 {
		t.Fatalf("GetDomains() returned %d domains, want 1", len(domains))
	}
	if domains[0].Domain != "example.com" {
		t.Errorf("domain name = %q, want %q", domains[0].Domain, "example.com")
	}
	if len(domains[0].Records) != 1 {
		t.Fatalf("domain has %d records, want 1", len(domains[0].Records))
	}
}
