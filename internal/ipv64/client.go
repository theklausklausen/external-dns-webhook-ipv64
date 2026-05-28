package ipv64

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

const (
	defaultAPIURL   = "https://ipv64.net/api.php"
	healthcheckURL  = "https://ipv64.net/"
	requestInterval = 5 * time.Second
	requestBurst    = 1
)

// Client represents an ipv64 DNS API client
//
//nolint:revive // apiURL naming follows existing project style.
type Client struct {
	apiURL     string
	apiKey     string
	httpClient *http.Client
	limiter    *rate.Limiter
}

// NewClient creates a new ipv64 DNS client with Bearer token authentication
func NewClient(apiKey string) *Client {
	return &Client{
		apiURL: defaultAPIURL,
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		limiter: rate.NewLimiter(rate.Every(requestInterval), requestBurst),
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
		limiter: rate.NewLimiter(rate.Every(requestInterval), requestBurst),
	}
}

// isSuccessStatus normalizes IPv64 status values to account for API variants.
func isSuccessStatus(status string) bool {
	s := strings.ToLower(strings.TrimSpace(status))
	if s == "success" || s == "ok" {
		return true
	}

	// Some IPv64 responses return HTTP-like status strings such as "200 OK".
	return strings.HasPrefix(s, "2") || strings.HasPrefix(s, "http 2")
}

// doRequest performs an HTTP request with Bearer token authentication
func (c *Client) doRequest(method, apiCall string, params url.Values) ([]byte, error) {
	requestStarted := time.Now()

	if err := c.limiter.Wait(context.Background()); err != nil {
		return nil, fmt.Errorf("rate limit wait failed: %w", err)
	}

	var reqURL string
	var body io.Reader

	if method == http.MethodGet {
		if params == nil {
			params = url.Values{}
		}
		params.Set(apiCall, "")
		reqURL = fmt.Sprintf("%s?%s", c.apiURL, params.Encode())
	} else if method == http.MethodPost || method == http.MethodDelete {
		if params == nil {
			params = url.Values{}
		}
		body = strings.NewReader(params.Encode())
		reqURL = c.apiURL
	} else {
		return nil, fmt.Errorf("unsupported HTTP method: %s", method)
	}

	log.Debugf("IPv64 request: method=%s url=%s apiCall=%s params=%q", method, reqURL, apiCall, params.Encode())

	req, err := http.NewRequest(method, reqURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	if method == http.MethodPost || method == http.MethodDelete {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Debugf("IPv64 request failed: method=%s url=%s error=%v", method, reqURL, err)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	log.Debugf("IPv64 response: method=%s url=%s status=%d duration=%s body=%q", method, reqURL, resp.StatusCode, time.Since(requestStarted), string(data))

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

	if !isSuccessStatus(resp.Status) {
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

	if !isSuccessStatus(resp.Status) {
		return nil, fmt.Errorf("API returned status: %s", resp.Status)
	}

	var domains []Domain
	responseJSON, err := json.Marshal(resp.Response)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	if err := json.Unmarshal(responseJSON, &domains); err != nil {
		var domainsMap map[string]Domain
		if err := json.Unmarshal(responseJSON, &domainsMap); err != nil {
			return nil, fmt.Errorf("failed to decode domains: %w", err)
		}
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

	if !isSuccessStatus(resp.Status) && !strings.Contains(strings.ToLower(resp.Response), "already exists") {
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

	if !isSuccessStatus(resp.Status) {
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

	if !isSuccessStatus(resp.Status) {
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

	if !isSuccessStatus(resp.Status) {
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

	if !isSuccessStatus(resp.Status) {
		return fmt.Errorf("failed to delete record: %s", resp.Response)
	}

	log.Infof("Successfully deleted record with ID: %d", recordID)
	return nil
}

// HealthCheck performs a health check on the ipv64 API
func (c *Client) HealthCheck() error {
	req, err := http.NewRequest(http.MethodGet, healthcheckURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}
	defer resp.Body.Close()

	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return fmt.Errorf("failed to read health check response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("health check failed with status: %s", resp.Status)
	}

	return nil
}
