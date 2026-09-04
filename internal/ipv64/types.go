package ipv64

import "time"

// Domain represents a domain in ipv64
type Domain struct {
	Domain  string   `json:"domain"`
	Records []Record `json:"records,omitempty"`
}

// Record represents a DNS record in ipv64
type Record struct {
	ID      int    `json:"id,omitempty"`
	Domain  string `json:"domain"`
	Praefix string `json:"praefix"` // Note: ipv64 API uses "praefix" spelling
	Type    string `json:"type"`
	Content string `json:"content"`
}

// AccountInfo represents account information
type AccountInfo struct {
	Email        string    `json:"email"`
	AccountClass string    `json:"account_class"`
	APILimit     int       `json:"api_limit"`
	APICalls     int       `json:"api_calls"`
	LastLogin    time.Time `json:"last_login,omitempty"`
}

// APIResponse represents a generic API response
type APIResponse struct {
	Response string `json:"response,omitempty"`
	Status   string `json:"status"`
	APICall  string `json:"api-call,omitempty"`
}

// DomainsResponse represents the response from get_domains API
type DomainsResponse struct {
	Response   interface{}       `json:"response"` // Can be array of domains or object with domains
	Subdomains map[string]Domain `json:"subdomains"`
	Status     string            `json:"status"`
	APICall    string            `json:"api-call,omitempty"`
}

// AccountInfoResponse represents the response from get_account_info API
type AccountInfoResponse struct {
	Response AccountInfo `json:"response"`
	Status   string      `json:"status"`
	APICall  string      `json:"api-call,omitempty"`
}

// LogEntry represents a log entry
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"`
	Details   string    `json:"details"`
}

// LogsResponse represents the response from get_logs API
type LogsResponse struct {
	Response []LogEntry `json:"response"`
	Status   string     `json:"status"`
	APICall  string     `json:"api-call,omitempty"`
}
