# external-dns-webhook-ipv64

[![Go Version](https://img.shields.io/badge/go-1.21-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

A webhook provider for [external-dns](https://github.com/kubernetes-sigs/external-dns) that integrates with [ipv64.net](https://ipv64.net) DNS service, enabling automatic DNS record management for Kubernetes services and ingresses.

## Features

- 🔄 Automatic DNS record synchronization from Kubernetes to ipv64.net
- 🎯 Support for A, AAAA, CNAME, TXT, MX, NS, and SRV records
- 🔐 Bearer token authentication with ipv64 API
- 🌐 Domain filtering for selective DNS management
- 📊 Health checks and readiness probes
- 🐳 Docker and Kubernetes-ready
- 🚀 Lightweight and efficient API client

## Quick Start

### Prerequisites

- [Go 1.21+](https://go.dev/doc/install) (for building from source)
- [Docker](https://docs.docker.com/get-docker/) (for containerized deployment)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- An [ipv64.net](https://ipv64.net) account with API key

### Installation

#### Option 1: Using Docker

```bash
docker pull your-registry/external-dns-webhook-ipv64:latest
docker run -e IPV64_API_KEY=your-api-key -p 8888:8888 external-dns-webhook-ipv64:latest
```

#### Option 2: Building from Source

1. **Clone the repository:**
   ```bash
   git clone https://github.com/klausklausen/external-dns-webhook-ipv64.git
   cd external-dns-webhook-ipv64
   ```

2. **Initialize Go modules:**
   ```bash
   go mod download
   ```

3. **Build the binary:**
   ```bash
   go build -o webhook ./cmd/webhook
   ```

4. **Run the webhook:**
   ```bash
   export IPV64_API_KEY=your-api-key
   ./webhook
   ```

## Configuration

### Environment Variables

The webhook can be configured using the following environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `IPV64_API_KEY` | - | IPv64 API key (required) |
| `IPV64_API_URL` | `https://ipv64.net/api.php` | IPv64 API URL |
| `WEBHOOK_ADDR` | `:8888` | Webhook server listen address |
| `DOMAIN_FILTER` | - | Comma-separated list of domains to manage |
| `DRY_RUN` | `false` | Enable dry-run mode (no changes made) |
| `LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `LOG_FORMAT` | `text` | Log format (text, json) |

### Command Line Flags

All environment variables can also be specified as command line flags:

```bash
./webhook \
  --ipv64-api-key=your-api-key \
  --ipv64-api-url=https://ipv64.net/api.php \
  --domain-filter=example.com,test.com \
  --log-level=debug
```

## Architecture

```
┌─────────────────┐
│   Kubernetes    │
│   (Services/    │
│   Ingresses)    │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  external-dns   │
│   (watches K8s  │
│    resources)   │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│     Webhook     │
│   (this project)│
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   ipv64.net     │
│   DNS Service   │
└─────────────────┘
```

1. **external-dns** watches Kubernetes resources (Services, Ingresses) for DNS annotations
2. **Webhook** receives requests from external-dns and translates them to ipv64 API calls
3. **ipv64.net** stores and serves the DNS records

## Deployment

### Kubernetes Deployment

1. **Create a secret with your ipv64 API key:**
   ```bash
   kubectl create secret generic ipv64-api-key \
     --from-literal=api-key=your-api-key-here \
     -n external-dns
   ```

2. **Deploy the webhook:**
   ```bash
   kubectl apply -f deploy/kubernetes/webhook-deployment.yaml
   ```

3. **Deploy external-dns:**
   ```bash
   kubectl apply -f deploy/external-dns/deployment.yaml
   ```

### Using with Helm (external-dns chart)

```yaml
apiVersion: helm.cattle.io/v1
kind: HelmChart
metadata:
  name: external-dns
  namespace: kube-system
spec:
  chart: external-dns
  repo: https://kubernetes-sigs.github.io/external-dns/
  targetNamespace: external-dns
  createNamespace: true
  version: 1.19.0
  valuesContent: |-
    provider:
      name: webhook
      webhook:
        image:
          repository: your-registry/external-dns-webhook-ipv64
          tag: latest
        env:
          - name: IPV64_API_KEY
            valueFrom:
              secretKeyRef:
                name: ipv64-api-key
                key: api-key
          - name: LOG_LEVEL
            value: "info"
          - name: DOMAIN_FILTER
            value: "example.com"
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8888
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8888
```

## Usage with Kubernetes

### Service Annotation

To create DNS records for a Kubernetes Service, add the `external-dns.alpha.kubernetes.io/hostname` annotation:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-service
  annotations:
    external-dns.alpha.kubernetes.io/hostname: my-app.example.com
spec:
  type: LoadBalancer
  ports:
  - port: 80
    targetPort: 8080
  selector:
    app: my-app
```

### Ingress Annotation

For Ingress resources, external-dns automatically uses the hostname from the Ingress spec:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-ingress
spec:
  rules:
  - host: my-app.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: my-service
            port:
              number: 80
```

### Custom TTL

Set a custom TTL for DNS records (note: ipv64 may have its own TTL handling):

```yaml
metadata:
  annotations:
    external-dns.alpha.kubernetes.io/hostname: my-app.example.com
    external-dns.alpha.kubernetes.io/ttl: "300"
```

## API Endpoints

The webhook exposes the following endpoints:

- `GET /` - Service information and domain filter (for external-dns negotiation)
- `GET /healthz` - Health check
- `GET /readyz` - Readiness check
- `GET /records` - List all DNS records
- `POST /records` - Apply DNS record changes
- `POST /adjustendpoints` - Adjust endpoints before processing

## IPv64 API Integration

This webhook uses the ipv64.net API with the following features:

- **Authentication**: Bearer token authentication
- **API Calls**: 
  - `GET get_account_info` - Retrieve account information
  - `GET get_domains` - Get all domains and records
  - `POST add_domain` - Create a new domain
  - `DELETE del_domain` - Delete a domain
  - `POST add_record` - Add a DNS record
  - `DELETE del_record` - Delete a DNS record

### API Rate Limits

- **Account Limit**: Depends on account class (default: 64 requests per 24 hours)
- **Call Limit**: Maximum 5 API requests within 10 seconds

The webhook is designed to minimize API calls by batching operations and caching domain information.

## Development

### Project Structure

```
.
├── cmd/
│   └── webhook/          # Main application entry point
├── internal/
│   ├── ipv64/           # IPv64 API client
│   └── webhook/         # Webhook server and provider
├── deploy/
│   ├── kubernetes/      # Webhook deployment manifests
│   └── external-dns/    # external-dns deployment manifests
├── docs/                # Additional documentation
├── Dockerfile           # Multi-stage Docker build
├── go.mod               # Go module definition
└── README.md            # This file
```

### Building

```bash
# Build binary
go build -o webhook ./cmd/webhook

# Build Docker image
docker build -t external-dns-webhook-ipv64:latest .

# Run tests (if implemented)
go test ./...
```

## Troubleshooting

### Webhook not connecting to ipv64

Check the webhook logs:
```bash
kubectl logs -n external-dns deployment/external-dns-webhook-ipv64
```

Verify your API key is correct and has sufficient permissions.

### DNS records not being created

1. Check external-dns logs:
   ```bash
   kubectl logs -n external-dns deployment/external-dns
   ```

2. Verify the service has the correct annotation:
   ```bash
   kubectl get service <service-name> -o yaml | grep external-dns
   ```

3. Check domain filter configuration matches your domains.

### API Rate Limiting

If you encounter rate limiting errors:
- Review your account class and API limits at ipv64.net
- Consider reducing the external-dns sync interval
- Enable dry-run mode to test without making actual API calls

## Security Best Practices

1. **Never commit API keys** - Always use environment variables or secrets
2. **Use read-only filesystem** - The webhook runs with a read-only root filesystem
3. **Run as non-root user** - Container runs as user ID 1000
4. **Network policies** - Restrict webhook access to only external-dns
5. **RBAC** - Grant minimal permissions required for external-dns

## Contributing

Contributions are welcome! Please follow these steps:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [external-dns](https://github.com/kubernetes-sigs/external-dns) - The external-dns project
- [ipv64.net](https://ipv64.net) - DNS service provider
- [kubernetes-sigs](https://github.com/kubernetes-sigs) - Kubernetes Special Interest Groups

## Support

- 📖 [Documentation](docs/)
- 🐛 [Issue Tracker](https://github.com/klausklausen/external-dns-webhook-ipv64/issues)
- 💬 [Discussions](https://github.com/klausklausen/external-dns-webhook-ipv64/discussions)

## Roadmap

- [ ] Comprehensive integration tests
- [ ] Metrics endpoint for Prometheus
- [ ] Support for more DNS record types
- [ ] Improved error handling and retry logic
- [ ] Performance optimizations for large deployments
