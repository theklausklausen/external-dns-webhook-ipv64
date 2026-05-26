package ipv64

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	defaultAPIURL = "https://ipv64.net/api.php"
)

// Client represents an ipv64 DNS API client
type Client struct {
	apiURL     string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new ipv64 DNS client with Bearer token authentication
func NewClient(apiKey string) *Client {
	return &Client{
		apiURL: defaultAPIURL,
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewClientWithURL creates a new client with custom API URL
func NewClientWithURL(apiURL, apiKey string) *Client {
	return &Client{
		apiURL: apiURL,
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// doRequest performs an HTTP request with Bearer token authentication
func (c *Client) doRequest(method, apiCall string, params url.Values) ([]byte, error) {
	var reqURL string
	var body io.Reader

	if method == http.MethodGet {
		// For GET requests, add parameters to URL
		if params == nil {
			params = url.Values{}
		}
		params.Set(apiCall, "")
		reqURL = fmt.Sprintf("%s?%s", c.apiURL, params.Encode())
	} else if method == http.MethodPost {
		// For POST requests, use form data
		if params == nil {
			params = url.Values{}
		}
		body = strings.NewReader(params.Encode())
		reqURL = c.apiURL
	} else if method == http.MethodDelete {
		// For DELETE requests, use form data
		if params == nil {
			params = url.Values{}
		}
		body = strings.NewReader(params.Encode())
		reqURL = c.apiURL
	}

	req, err := http.NewRequest(method, reqURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add Bearer token authentication
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	// Set content type for POST and DELETE requests
	if method == http.MethodPost || method == http.MethodDelete {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(data))
	}

	return data, nil
}

// GetAccountInfo retrieves account information
func (c *Client) GetAccountInfo() (*AccountInfo, error) {
	data, err := c.doRequest(http.MethodGet, "get_account_info", nil)
	if err != nil {
		return nil, err
	}

	var resp AccountInfoResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to decode account info response: %w", err)
	}

	if resp.Status != "success" {
		return nil, fmt.Errorf("API returned status: %s", resp.Status)
	}

	return &resp.Response, nil
}

// GetDomains retrieves all domains and their DNS records
func (c *Client) GetDomains() ([]Domain, error) {
	data, err := c.doRequest(http.MethodGet, "get_domains", nil)
	if err != nil {
		return nil, err
	}

	var resp DomainsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to decode domains response: %w", err)
	}

	if resp.Status != "success" {
		return nil, fmt.Errorf("API returned status: %s", resp.Status)
	}

	// Parse the response which can be either an array or an object
	var domains []Domain
	responseJSON, err := json.Marshal(resp.Response)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	// Try to unmarshal as array first
	if err := json.Unmarshal(responseJSON, &domains); err != nil {
		// If that fails, try as object with domain keys
		var domainsMap map[string]Domain
		if err := json.Unmarshal(responseJSON, &domainsMap); err != nil {
			return nil, fmt.Errorf("failed to decode domains: %w", err)
		}
		// Convert map to slice
		for domainName, domain := range domainsMap {
			domain.Domain = domainName
			domains = append(domains, domain)
		}
	}

	return domains, nil
}

// AddDomain creates a new domain
func (c *Client) AddDomain(domain string) error {
	params := url.Values{}
	params.Set("add_domain", domain)

	data, err := c.doRequest(http.MethodPost, "", params)
	if err != nil {
		return fmt.Errorf("failed to add domain: %w", err)
	}

	var resp APIResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("failed to decode add domain response: %w", err)
	}

	if resp.Status != "success" && !strings.Contains(strings.ToLower(resp.Response), "already exists") {
		return fmt.Errorf("failed to add domain: %s", resp.Response)
	}

	log.Infof("Successfully created domain: %s", domain)
	return nil
}

// DeleteDomain deletes a domain
func (c *Client) DeleteDomain(domain string) error {
	params := url.Values{}
	params.Set("del_domain", domain)

	data, err := c.doRequest(http.MethodDelete, "", params)
	if err != nil {
		return fmt.Errorf("failed to delete domain: %w", err)
	}

	var resp APIResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("failed to decode delete domain response: %w", err)
	}

	if resp.Status != "success" {
		return fmt.Errorf("failed to delete domain: %s", resp.Response)
	}

	log.Infof("Successfully deleted domain: %s", domain)
	return nil
}

// AddRecord adds a new DNS record
func (c *Client) AddRecord(domain, praefix, recordType, content string) error {
	params := url.Values{}
	params.Set("add_record", domain)
	params.Set("praefix", praefix)
	params.Set("type", recordType)
	params.Set("content", content)

	data, err := c.doRequest(http.MethodPost, "", params)
	if err != nil {
		return fmt.Errorf("failed to add record: %w", err)
	}

	var resp APIResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("failed to decode add record response: %w", err)
	}

	if resp.Status != "success" {
		return fmt.Errorf("failed to add record: %s", resp.Response)
	}

	log.Infof("Successfully added record: %s.%s %s %s", praefix, domain, recordType, content)
	return nil
}

// DeleteRecord deletes a DNS record by domain, praefix, type, and content
func (c *Client) DeleteRecord(domain, praefix, recordType, content string) error {
	params := url.Values{}
	params.Set("del_record", domain)
	params.Set("praefix", praefix)
	params.Set("type", recordType)
	params.Set("content", content)

	data, err := c.doRequest(http.MethodDelete, "", params)
	if err != nil {
		return fmt.Errorf("failed to delete record: %w", err)
	}

	var resp APIResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("failed to decode delete record response: %w", err)
	}

	if resp.Status != "success" {
		return fmt.Errorf("failed to delete record: %s", resp.Response)
	}

	log.Infof("Successfully deleted record: %s.%s %s %s", praefix, domain, recordType, content)
	return nil
}

// DeleteRecordByID deletes a DNS record by its ID
func (c *Client) DeleteRecordByID(recordID int) error {
	params := url.Values{}
	params.Set("del_record", fmt.Sprintf("%d", recordID))

	data, err := c.doRequest(http.MethodDelete, "", params)
	if err != nil {
		return fmt.Errorf("failed to delete record: %w", err)
	}

	var resp APIResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("failed to decode delete record response: %w", err)
	}

	if resp.Status != "success" {
		return fmt.Errorf("failed to delete record: %s", resp.Response)
	}

	log.Infof("Successfully deleted record with ID: %d", recordID)
	return nil
}

// HealthCheck performs a health check on the ipv64 API
func (c *Client) HealthCheck() error {
	_, err := c.GetAccountInfo()
	return err
}
