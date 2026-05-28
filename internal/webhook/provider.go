package webhook

import (
	"context"
	"fmt"
	"strings"

	"github.com/klausklausen/external-dns-webhook-ipv64/internal/ipv64"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
	"sigs.k8s.io/external-dns/provider"
)

// IPv64Provider implements the external-dns provider interface
type IPv64Provider struct {
	client       *ipv64.Client
	domainFilter endpoint.DomainFilter
	dryRun       bool
}

// NewIPv64Provider creates a new IPv64Provider
func NewIPv64Provider(client *ipv64.Client, domainFilter endpoint.DomainFilter, dryRun bool) (*IPv64Provider, error) {
	return &IPv64Provider{
		client:       client,
		domainFilter: domainFilter,
		dryRun:       dryRun,
	}, nil
}

// Records returns the list of records in all managed domains
func (p *IPv64Provider) Records(ctx context.Context) ([]*endpoint.Endpoint, error) {
	log.Debug("Fetching DNS records from ipv64")

	domains, err := p.client.GetDomains()
	if err != nil {
		return nil, fmt.Errorf("failed to get domains: %w", err)
	}

	var endpoints []*endpoint.Endpoint

	for _, domain := range domains {
		// Skip domains that don't match the domain filter
		if !p.domainFilter.Match(domain.Domain) {
			log.Debugf("Skipping domain %s (doesn't match domain filter)", domain.Domain)
			continue
		}

		for _, record := range domain.Records {
			// Only process supported record types
			if !isSupportedRecordType(record.Type) {
				continue
			}

			ep := p.convertToEndpoint(record, domain.Domain)
			if ep != nil {
				endpoints = append(endpoints, ep)
			}
		}
	}

	log.Infof("Found %d DNS records", len(endpoints))
	return endpoints, nil
}

// ApplyChanges applies the changes to DNS records
func (p *IPv64Provider) ApplyChanges(ctx context.Context, changes *plan.Changes) error {
	if p.dryRun {
		log.Info("DRY RUN: Would apply the following changes:")
		log.Infof("Create: %d records", len(changes.Create))
		log.Infof("UpdateOld: %d records", len(changes.UpdateOld))
		log.Infof("UpdateNew: %d records", len(changes.UpdateNew))
		log.Infof("Delete: %d records", len(changes.Delete))
		return nil
	}

	// Process deletions
	for _, ep := range changes.Delete {
		if err := p.deleteEndpoint(ep); err != nil {
			log.Errorf("Failed to delete endpoint %s: %v", ep.DNSName, err)
		}
	}

	// Process updates (delete old, create new)
	for i := range changes.UpdateOld {
		oldEp := changes.UpdateOld[i]
		newEp := changes.UpdateNew[i]

		if err := p.deleteEndpoint(oldEp); err != nil {
			log.Errorf("Failed to delete old endpoint %s: %v", oldEp.DNSName, err)
			continue
		}

		if err := p.createEndpoint(newEp); err != nil {
			log.Errorf("Failed to create new endpoint %s: %v", newEp.DNSName, err)
		}
	}

	// Process creations
	for _, ep := range changes.Create {
		if err := p.createEndpoint(ep); err != nil {
			log.Errorf("Failed to create endpoint %s: %v", ep.DNSName, err)
		}
	}

	return nil
}

// AdjustEndpoints modifies endpoints before they are processed
func (p *IPv64Provider) AdjustEndpoints(endpoints []*endpoint.Endpoint) ([]*endpoint.Endpoint, error) {
	// No adjustments needed for ipv64
	return endpoints, nil
}

// GetDomainFilter returns the domain filter
func (p *IPv64Provider) GetDomainFilter() endpoint.DomainFilter {
	return p.domainFilter
}

// convertToEndpoint converts an ipv64 record to an external-dns endpoint
func (p *IPv64Provider) convertToEndpoint(record ipv64.Record, domainName string) *endpoint.Endpoint {
	if record.Content == "" {
		return nil
	}

	// Build full DNS name from praefix and domain
	var dnsName string
	if record.Praefix == "" || record.Praefix == "@" {
		dnsName = domainName
	} else {
		dnsName = fmt.Sprintf("%s.%s", record.Praefix, domainName)
	}

	return &endpoint.Endpoint{
		DNSName:    dnsName,
		RecordType: record.Type,
		Targets:    endpoint.Targets{record.Content},
		RecordTTL:  endpoint.TTL(300), // ipv64 API doesn't provide TTL, use default
	}
}

// createEndpoint creates a new DNS record from an endpoint
func (p *IPv64Provider) createEndpoint(ep *endpoint.Endpoint) error {
	domain := p.extractDomain(ep.DNSName)
	if domain == "" {
		return fmt.Errorf("could not determine domain for %s", ep.DNSName)
	}

	// Extract praefix from DNS name
	praefix := p.extractPraefix(ep.DNSName, domain)

	// Create records for each target
	for _, target := range ep.Targets {
		log.Debugf("Attempting to create record: domain=%s praefix=%s type=%s target=%s", domain, praefix, ep.RecordType, target)
		if err := p.client.AddRecord(domain, praefix, ep.RecordType, target); err != nil {
			// If record already exists, skip
			if strings.Contains(err.Error(), "already exists") {
				log.Infof("Record %s already exists, skipping", ep.DNSName)
				continue
			}

			if strings.Contains(strings.ToLower(err.Error()), "domain not found") {
				log.Errorf("Domain does not exist for record creation: domain=%s praefix=%s type=%s target=%s error=%v", domain, praefix, ep.RecordType, target, err)
				return err
			}

			log.Errorf("Failed to create record: domain=%s praefix=%s type=%s target=%s error=%v", domain, praefix, ep.RecordType, target, err)
			return err
		}
		log.Debugf("Successfully created record: domain=%s praefix=%s type=%s target=%s", domain, praefix, ep.RecordType, target)
	}

	return nil
}

// deleteEndpoint deletes a DNS record from an endpoint
func (p *IPv64Provider) deleteEndpoint(ep *endpoint.Endpoint) error {
	domain := p.extractDomain(ep.DNSName)
	if domain == "" {
		return fmt.Errorf("could not determine domain for %s", ep.DNSName)
	}

	// Extract praefix from DNS name
	praefix := p.extractPraefix(ep.DNSName, domain)

	// Delete records for each target
	for _, target := range ep.Targets {
		if err := p.client.DeleteRecord(domain, praefix, ep.RecordType, target); err != nil {
			// Ignore "not found" errors
			if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "does not exist") {
				log.Infof("Record %s not found, skipping deletion", ep.DNSName)
				continue
			}
			return err
		}
	}

	return nil
}

// extractDomain extracts the base domain from a DNS name
func (p *IPv64Provider) extractDomain(dnsName string) string {
	dnsName = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(dnsName), "."))
	if dnsName == "" {
		return ""
	}

	parts := strings.Split(dnsName, ".")
	if len(parts) >= 3 {
		return strings.Join(parts[len(parts)-3:], ".")
	}
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], ".")
	}

	return dnsName
}

// extractPraefix extracts the subdomain praefix from a DNS name
func (p *IPv64Provider) extractPraefix(dnsName, domain string) string {
	dnsName = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(dnsName), "."))
	domain = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domain), "."))

	if dnsName == domain {
		return "@"
	}

	// Remove the domain part from the DNS name
	praefix := strings.TrimSuffix(dnsName, "."+domain)
	if praefix == "" {
		return "@"
	}

	if praefix == dnsName {
		// If nothing was trimmed, the dnsName is the same as domain
		return "@"
	}

	return praefix
}

// isSupportedRecordType checks if a record type is supported
func isSupportedRecordType(recordType string) bool {
	supportedTypes := map[string]bool{
		"A":     true,
		"AAAA":  true,
		"CNAME": true,
		"TXT":   true,
		"MX":    true,
		"NS":    true,
		"SRV":   true,
	}
	return supportedTypes[recordType]
}

// Ensure IPv64Provider implements the provider.Provider interface
var _ provider.Provider = &IPv64Provider{}
