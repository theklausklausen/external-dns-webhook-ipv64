package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/klausklausen/external-dns-webhook-ipv64/internal/ipv64"
	"github.com/klausklausen/external-dns-webhook-ipv64/internal/webhook"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/external-dns/endpoint"
)

var (
	// IPv64 configuration
	ipv64APIKey = flag.String("ipv64-api-key", getEnv("IPV64_API_KEY", ""), "IPv64 API key (required)")
	ipv64APIURL = flag.String("ipv64-api-url", getEnv("IPV64_API_URL", "https://ipv64.net/api.php"), "IPv64 API URL")

	// Webhook configuration
	webhookAddr = flag.String("webhook-addr", getEnv("WEBHOOK_ADDR", ":8888"), "Webhook server listen address")

	// DNS configuration
	domainFilter      = flag.String("domain-filter", getEnv("DOMAIN_FILTER", ""), "Comma-separated list of domain filters")
	dryRun            = flag.Bool("dry-run", getEnv("DRY_RUN", "false") == "true", "Run in dry-run mode (no changes will be made)")
	createRecordTypes = flag.String(
		"create-record-types",
		getEnv("CREATE_RECORD_TYPES", "TXT,A,AAAA,CNAME"),
		"Comma-separated list of DNS record types external-dns is allowed to create",
	)

	// Logging
	logLevel  = flag.String("log-level", getEnv("LOG_LEVEL", "info"), "Log level (debug, info, warn, error)")
	logFormat = flag.String("log-format", getEnv("LOG_FORMAT", "text"), "Log format (text, json)")
)

func main() {
	flag.Parse()

	// Setup logging
	setupLogging(*logLevel, *logFormat)

	log.Info("Starting external-dns-webhook-ipv64")
	log.Infof("IPv64 API URL: %s", *ipv64APIURL)
	log.Infof("Webhook address: %s", *webhookAddr)
	log.Infof("Dry run: %v", *dryRun)

	// Validate API key
	if *ipv64APIKey == "" {
		log.Fatal("IPv64 API key is required (set IPV64_API_KEY environment variable or use --ipv64-api-key flag)")
	}

	// Create IPv64 client
	var client *ipv64.Client
	if *ipv64APIURL == "https://ipv64.net/api.php" {
		client = ipv64.NewClient(*ipv64APIKey)
	} else {
		client = ipv64.NewClientWithURL(*ipv64APIURL, *ipv64APIKey)
	}

	// Verify connection
	if err := client.HealthCheck(); err != nil {
		log.Fatalf("Failed to connect to IPv64 API: %v", err)
	}
	log.Info("Successfully connected to IPv64 API")

	// Get account info
	accountInfo, err := client.GetAccountInfo()
	if err != nil {
		log.Warnf("Failed to get account info: %v", err)
	} else {
		log.Infof("Account: %s (Class: %s, API Limit: %d, API Calls: %d)",
			accountInfo.Email, accountInfo.AccountClass, accountInfo.APILimit, accountInfo.APICalls)
	}

	// Create domain filter
	filter := createDomainFilter(*domainFilter)
	if len(filter.Filters) > 0 {
		log.Infof("Domain filter: %v", filter.Filters)
	} else {
		log.Info("No domain filter configured (all domains will be managed)")
	}

	allowedCreateRecordTypes := parseRecordTypes(*createRecordTypes)
	if len(allowedCreateRecordTypes) == 0 {
		log.Fatal("At least one create record type must be configured (CREATE_RECORD_TYPES or --create-record-types)")
	}
	log.Infof("Allowed create record types: %v", allowedCreateRecordTypes)

	// Create provider
	provider, err := webhook.NewIPv64Provider(client, filter, *dryRun, allowedCreateRecordTypes)
	if err != nil {
		log.Fatalf("Failed to create provider: %v", err)
	}

	// Create webhook server
	server := webhook.NewServer(provider, *webhookAddr)

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := server.Start(); err != nil {
			errChan <- err
		}
	}()

	// Wait for shutdown signal or error
	select {
	case sig := <-sigChan:
		log.Infof("Received signal: %v", sig)
	case err := <-errChan:
		log.Errorf("Server error: %v", err)
	}

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Errorf("Error during shutdown: %v", err)
		os.Exit(1)
	}

	log.Info("Shutdown complete")
}

// setupLogging configures the logging system
func setupLogging(level, format string) {
	// Set log level
	switch strings.ToLower(level) {
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "warn", "warning":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}

	// Set log format
	if strings.ToLower(format) == "json" {
		log.SetFormatter(&log.JSONFormatter{})
	} else {
		log.SetFormatter(&log.TextFormatter{
			FullTimestamp: true,
		})
	}
}

// createDomainFilter creates a domain filter from a comma-separated string
func createDomainFilter(filterStr string) endpoint.DomainFilter {
	if filterStr == "" {
		return endpoint.DomainFilter{}
	}

	filters := strings.Split(filterStr, ",")
	var trimmedFilters []string
	for _, f := range filters {
		if trimmed := strings.TrimSpace(f); trimmed != "" {
			trimmedFilters = append(trimmedFilters, trimmed)
		}
	}

	return endpoint.NewDomainFilter(trimmedFilters)
}

// parseRecordTypes parses a comma-separated record type list and returns normalized values.
func parseRecordTypes(typesStr string) []string {
	parts := strings.Split(typesStr, ",")
	result := make([]string, 0, len(parts))
	seen := make(map[string]bool, len(parts))

	for _, p := range parts {
		normalized := strings.ToUpper(strings.TrimSpace(p))
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		result = append(result, normalized)
	}

	return result
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
