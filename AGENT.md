# AGENT.md - AI Assistant Context

This document provides context and guidance for AI assistants working with this codebase.

## Project Overview

**external-dns-webhook-ipv64** is a Kubernetes webhook provider that bridges external-dns and ipv64.net DNS service. It enables automatic DNS record management for Kubernetes workloads by watching Services and Ingresses and synchronizing their DNS requirements to the ipv64.net cloud DNS service.

## Architecture

### High-Level Flow
1. **external-dns** (from kubernetes-sigs) watches Kubernetes resources
2. When it detects services/ingresses with DNS annotations, it calls our **webhook**
3. The **webhook** translates external-dns requests to ipv64 API calls
4. **ipv64.net DNS Service** manages the actual DNS records

### Key Components

#### 1. IPv64 API Client (`internal/ipv64/`)
- **client.go**: HTTP client for ipv64.net REST API
- **types.go**: Data structures matching ipv64's API schema
- Handles Bearer token authentication
- Manages domains and records (create, update, delete)

#### 2. Webhook Provider (`internal/webhook/`)
- **provider.go**: Implements external-dns provider interface
- **server.go**: HTTP server exposing webhook endpoints
- Transforms between external-dns and ipv64 data models
- Handles domain filtering and record type mapping

#### 3. Main Application (`cmd/webhook/`)
- **main.go**: Entry point with configuration and setup
- Signal handling for graceful shutdown
- Logging configuration
- Environment variable and flag parsing

## Key Design Decisions

### 1. API Client Implementation
- Uses standard `net/http` instead of generated clients
- Simple, maintainable code without external dependencies
- Bearer token authentication as specified by ipv64 API
- Automatic retry and error handling for common scenarios
- Rate limit awareness (64 calls per 24h default, 5 per 10 seconds)

### 2. Provider Interface
- Implements `sigs.k8s.io/external-dns/provider.Provider`
- Methods:
  - `Records()`: Fetch all DNS records from ipv64
  - `ApplyChanges()`: Apply create/update/delete operations
  - `AdjustEndpoints()`: Pre-process endpoints (currently no-op)

### 3. Supported Record Types
Currently supports:
- **A**: IPv4 addresses
- **AAAA**: IPv6 addresses
- **CNAME**: Canonical names
- **TXT**: Text records
- **MX**: Mail exchange records
- **NS**: Name server records
- **SRV**: Service records

### 4. Domain Management
- Automatically creates domains if they don't exist
- Extracts domain from DNS name by finding longest matching domain
- Filters domains based on domain filter configuration
- iPv64 uses "praefix" (their spelling) for subdomains - @ for apex

## Development Workflow

### Local Development Setup

1. **Prerequisites**: kubectl, docker, go 1.21+
2. **Build**: `make build` or `go build -o bin/webhook ./cmd/webhook`
3. **Development cycle**:
   - Edit code
   - `make build` (builds binary)
   - `make docker-build` (builds Docker image)
   - Deploy to test cluster
   - `kubectl logs -n external-dns -l app=external-dns-webhook-ipv64` (view logs)

### Testing Strategy

**Manual Testing**:
```bash
# Deploy to cluster
kubectl apply -f deploy/kubernetes/webhook-deployment.yaml
kubectl apply -f deploy/external-dns/deployment.yaml

# Create test service
kubectl apply -f docs/examples/test-service.yaml

# Check logs
kubectl logs -n external-dns -l app=external-dns-webhook-ipv64
kubectl logs -n external-dns -l app=external-dns

# Verify in ipv64.net dashboard
```

**Unit Tests**: Currently minimal, room for expansion
**Integration Tests**: TODO - automated end-to-end testing

### Debugging

**Common Issues**:
1. **API key invalid**: Verify IPV64_API_KEY secret is correct
2. **Rate limiting**: Check account limits on ipv64.net dashboard
3. **Records not created**: Check both external-dns and webhook logs
4. **Permission denied**: Verify RBAC configuration in deploy/external-dns/

**Debugging Commands**:
```bash
# Check webhook status
kubectl get pods -n external-dns -l app=external-dns-webhook-ipv64

# View logs
kubectl logs -n external-dns -l app=external-dns-webhook-ipv64
kubectl logs -n external-dns -l app=external-dns

# Describe pod for events
kubectl describe pod <pod-name> -n external-dns

# Test webhook health
kubectl port-forward -n external-dns svc/external-dns-webhook-ipv64 8888:8888
curl http://localhost:8888/healthz
```

## Code Patterns

### Error Handling
```go
if err != nil {
    log.Errorf("Operation failed: %v", err)
    return fmt.Errorf("context: %w", err)
}
```
- Always wrap errors with context
- Log errors before returning
- Use structured logging (logrus)

### Logging
```go
log.Infof("Action completed: %s", details)
log.Debugf("Debug info: %+v", data)
log.Warnf("Non-fatal issue: %v", warning)
log.Errorf("Error occurred: %v", err)
```
- Info: Important operations
- Debug: Detailed flow information
- Warn: Non-fatal issues
- Error: Failures

### Configuration
- Environment variables preferred
- Command-line flags as alternative
- Sensible defaults for development
- All configurable via Kubernetes ConfigMap/Secret

## Kubernetes Manifests

### Deployment Structure
```
deploy/
├── kubernetes/      # Webhook (Deployment + Service)
└── external-dns/    # external-dns controller
```

### Resource Organization
- **Namespace**: `external-dns` for all components
- **ServiceAccounts**: Separate for webhook and external-dns
- **RBAC**: ClusterRole for external-dns to watch resources
- **Secret**: IPV64_API_KEY for webhook configuration

## Makefile Commands

The `Makefile` provides task automation:

**Setup**:
- `make deps` - Download Go dependencies
- `make tidy` - Tidy Go modules

**Development**:
- `make build` - Build binary
- `make docker-build` - Build Docker image
- `make test` - Run tests
- `make fmt` - Format code
- `make lint` - Run linter

**Operations**:
- `make run` - Run locally
- `make clean` - Clean build artifacts
- `make docker-push` - Push Docker image

## External Dependencies

### Go Modules
- `github.com/go-chi/chi/v5` - HTTP router
- `github.com/sirupsen/logrus` - Structured logging
- `sigs.k8s.io/external-dns` - external-dns types

### Container Images
- `golang:1.21-alpine` - Build stage
- `alpine:3.19` - Runtime stage
- `registry.k8s.io/external-dns/external-dns:v0.14.0` - Controller

## Common Modifications

### Adding a New Record Type

1. Update `internal/ipv64/client.go`:
   - Verify ipv64 API supports the record type
   
2. Update `internal/webhook/provider.go`:
   - Add to `isSupportedRecordType()`
   - Verify `convertToEndpoint()` handles it correctly

3. Test with appropriate Kubernetes resource

### Changing Default Configuration

1. Update environment variables in `deploy/kubernetes/webhook-deployment.yaml`
2. Update defaults in `cmd/webhook/main.go`
3. Update `README.md` configuration section
4. Redeploy webhook

### Handling IPv64 API Specifics

**Important Notes**:
- API uses "praefix" (not "prefix") for subdomain field
- Use "@" for apex/root domain records
- Bearer token authentication required
- Rate limits: 64 calls/24h (default), 5 calls/10s
- Domain creation automatically creates A or AAAA record

## Testing Considerations

### Unit Tests
- Mock ipv64 client for provider tests
- Test record type conversions
- Test domain extraction logic
- Test praefix handling (@ for apex)
- Test error handling paths

### Integration Tests
- Deploy to test cluster with real ipv64 account
- Create Kubernetes resources
- Verify DNS records in ipv64.net dashboard
- Test cleanup/deletion
- Test rate limit handling
- Test edge cases (invalid domains, etc.)

## Performance Considerations

- **API Rate Limits**: Must respect ipv64's limits (5 per 10s, 64 per 24h)
- **Caching**: Consider caching domain list to reduce API calls
- **Batch Operations**: Group operations where possible
- **Connection Pooling**: HTTP client reuses connections
- **Sync Interval**: external-dns interval should respect rate limits

## Security Considerations

- **API Key**: Stored in Kubernetes Secret, never in code or logs
- **RBAC**: Minimal permissions for each component
- **Non-root**: All containers run as non-root users (UID 1000)
- **Read-only FS**: Webhook runs with read-only root filesystem
- **Network Policies**: TODO - isolate components
- **TLS**: Webhook communicates over HTTP in-cluster, consider TLS for external access

## IPv64 API Specifics

### Authentication
- **Method**: Bearer Token in Authorization header
- **Format**: `Authorization: Bearer {api-key}`
- **Alternative methods**: API key supports multiple auth options, but Bearer is preferred

### API Endpoints
- **Base URL**: `https://ipv64.net/api.php` (or `https://ipv64.net/api`)
- **get_account_info**: GET - Account details and limits
- **get_domains**: GET - All domains and records
- **add_domain**: POST - Create domain (form-data)
- **del_domain**: DELETE - Delete domain (x-www-form-urlencoded)
- **add_record**: POST - Create DNS record (form-data)
- **del_record**: DELETE - Delete DNS record by details or ID

### Rate Limits
- **Account**: 64 calls per 24 hours (varies by account class)
- **Call Limit**: Maximum 5 API requests within 10 seconds
- **Handling**: Log warnings, let external-dns retry logic handle it

### Response Format
```json
{
  "response": "Success message or data",
  "status": "success",
  "api-call": "get_domains"
}
```

## Future Enhancements

1. **Metrics**: Prometheus metrics endpoint
2. **Helm Chart**: Package for easier deployment
3. **TLS**: Secure webhook endpoint
4. **HA**: Multiple webhook replicas
5. **Caching**: Domain and record caching with TTL
6. **Tests**: Comprehensive test suite
7. **CI/CD**: Automated builds and tests
8. **Rate Limit Handling**: Intelligent backoff for ipv64 limits

## Troubleshooting Guide for AI Assistants

When helping with issues:

1. **Check logs first**: 
   ```bash
   kubectl logs -n external-dns -l app=external-dns-webhook-ipv64
   kubectl logs -n external-dns -l app=external-dns
   ```

2. **Verify API key**: Check secret exists and is correctly mounted
   ```bash
   kubectl get secret ipv64-api-key -n external-dns
   ```

3. **Check connectivity**: Can webhook reach ipv64.net API?
   ```bash
   kubectl exec -n external-dns <pod> -- wget -O- https://ipv64.net/api.php?get_account_info
   ```

4. **Validate resources**: Do Services have correct annotations?
   ```bash
   kubectl get service -A -o yaml | grep external-dns
   ```

5. **Review RBAC**: Does external-dns have required permissions?
   ```bash
   kubectl get clusterrolebinding external-dns
   ```

6. **Check rate limits**: Review account usage in ipv64.net dashboard

7. **Test manually**: Use curl to test ipv64 API directly
   ```bash
   curl -X GET https://ipv64.net/api.php?get_account_info \
     -H "Authorization: Bearer your-api-key"
   ```

## References

- [external-dns Documentation](https://github.com/kubernetes-sigs/external-dns)
- [ipv64.net Website](https://ipv64.net)
- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [Go Best Practices](https://go.dev/doc/effective_go)
- [Project README](README.md)
- [API Documentation](docs/API.md)
- [Quick Start Guide](docs/QUICKSTART.md)
