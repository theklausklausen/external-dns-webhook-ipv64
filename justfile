# Justfile for external-dns-webhook-technitium

# Default recipe to display help
default:
    @just --list

# Variables
MINIKUBE_PROFILE := "external-dns"
DOCKER_IMAGE := "external-dns-webhook-technitium:latest"
NAMESPACE := "external-dns"

# Install prerequisites
install-deps:
    @echo "Checking prerequisites..."
    @command -v minikube >/dev/null 2>&1 || (echo "minikube not found. Install from https://minikube.sigs.k8s.io/" && exit 1)
    @command -v kubectl >/dev/null 2>&1 || (echo "kubectl not found. Install from https://kubernetes.io/docs/tasks/tools/" && exit 1)
    @command -v docker >/dev/null 2>&1 || (echo "docker not found. Install from https://docs.docker.com/get-docker/" && exit 1)
    @command -v go >/dev/null 2>&1 || (echo "go not found. Install from https://go.dev/doc/install" && exit 1)
    @echo "All prerequisites are installed!"

# Initialize Go modules
init:
    @echo "Initializing Go modules..."
    go mod tidy
    @echo "Go modules initialized!"

# Start minikube cluster
minikube-start:
    @echo "Starting minikube cluster..."
    minikube start --profile={{MINIKUBE_PROFILE}} --driver=docker --cpus=2 --memory=4096
    @echo "Enabling NGINX Ingress controller..."
    minikube addons enable ingress --profile={{MINIKUBE_PROFILE}}
    @echo "Minikube cluster started!"

# Stop minikube cluster
minikube-stop:
    @echo "Stopping minikube cluster..."
    minikube stop --profile={{MINIKUBE_PROFILE}}

# Delete minikube cluster
minikube-delete:
    @echo "Deleting minikube cluster..."
    minikube delete --profile={{MINIKUBE_PROFILE}}

# Build Go binary
build:
    @echo "Building webhook binary..."
    go build -o bin/webhook ./cmd/webhook
    @echo "Build complete!"

# Run tests
test:
    @echo "Running tests..."
    go test -v ./...

# Build Docker image
docker-build:
    @echo "Building Docker image..."
    eval $(minikube -p {{MINIKUBE_PROFILE}} docker-env) && \
    docker build -t {{DOCKER_IMAGE}} .
    @echo "Docker image built!"

# Load Docker image into minikube
docker-load:
    @echo "Loading Docker image into minikube..."
    minikube -p {{MINIKUBE_PROFILE}} image load {{DOCKER_IMAGE}}
    @echo "Image loaded!"

# Deploy Technitium DNS server
deploy-technitium:
    @echo "Deploying Technitium DNS server..."
    kubectl apply -f deploy/technitium/deployment.yaml
    @echo "Waiting for Technitium to be ready..."
    kubectl wait --for=condition=ready pod -l app=technitium-dns -n {{NAMESPACE}} --timeout=300s
    @echo "Technitium DNS server deployed!"

# Deploy webhook
deploy-webhook:
    @echo "Deploying webhook..."
    kubectl apply -f deploy/kubernetes/webhook-deployment.yaml
    @echo "Waiting for webhook to be ready..."
    kubectl wait --for=condition=ready pod -l app=external-dns-webhook-technitium -n {{NAMESPACE}} --timeout=120s
    @echo "Webhook deployed!"

# Deploy external-dns
deploy-external-dns:
    @echo "Deploying external-dns..."
    kubectl apply -f deploy/external-dns/deployment.yaml
    @echo "Waiting for external-dns to be ready..."
    kubectl wait --for=condition=ready pod -l app=external-dns -n {{NAMESPACE}} --timeout=120s
    @echo "External-DNS deployed!"

# Deploy Technitium Ingress
deploy-technitium-ingress:
    @echo "Deploying Technitium Ingress..."
    kubectl apply -f deploy/technitium/ingress.yaml
    @echo "Technitium Ingress deployed!"
    @echo "Access Technitium at: http://dns.192.168.49.2.nip.io"

# Deploy test ingress
deploy-test-ingress:
    @echo "Deploying test ingress..."
    kubectl apply -f deploy/test/test-ingress.yaml
    @echo "Test ingress deployed!"
    @echo "Access test app at: http://test.192.168.49.2.nip.io"

# Deploy all components
deploy-all: deploy-technitium deploy-webhook deploy-external-dns
    @echo "All components deployed!"

# Deploy all components including ingress
deploy-all-with-ingress: deploy-all deploy-technitium-ingress
    @echo "All components with ingress deployed!"

# Full setup: start minikube, build, and deploy everything
start: minikube-start docker-build deploy-all
    @echo "Environment is ready!"
    @echo ""
    @echo "Useful commands:"
    @echo "  just logs-webhook          - View webhook logs"
    @echo "  just logs-technitium       - View Technitium logs"
    @echo "  just logs-external-dns     - View external-dns logs"
    @echo "  just deploy-technitium-ingress - Deploy ingress for Technitium UI"
    @echo "  just port-forward-technitium - Access Technitium UI"
    @echo "  just status                - Check component status"

# Complete setup with ingress: start minikube, build, and deploy everything including ingress
start-all: minikube-start docker-build deploy-all-with-ingress
    @echo ""
    @echo "=========================================="
    @echo "🎉 Complete Environment is Ready!"
    @echo "=========================================="
    @echo ""
    @echo "Access Points:"
    @echo "  Technitium UI:  http://dns.192.168.49.2.nip.io"
    @echo "  (Credentials: admin / admin)"
    @echo ""
    @echo "Useful commands:"
    @echo "  just test-service-with-ingress - Create test service"
    @echo "  just logs-webhook              - View webhook logs"
    @echo "  just logs-external-dns         - View external-dns logs"
    @echo "  just show-urls                 - Show all ingress URLs"
    @echo "  just status                    - Check component status"
    @echo ""

# View webhook logs
logs-webhook:
    kubectl logs -f -l app=external-dns-webhook-technitium -n {{NAMESPACE}}

# View Technitium logs
logs-technitium:
    kubectl logs -f -l app=technitium-dns -n {{NAMESPACE}}

# View external-dns logs
logs-external-dns:
    kubectl logs -f -l app=external-dns -n {{NAMESPACE}}

# Port forward Technitium UI
port-forward-technitium:
    @echo "Port forwarding Technitium UI to http://localhost:5380"
    @echo "Default credentials: admin / admin"
    kubectl port-forward -n {{NAMESPACE}} service/technitium-dns 5380:5380

# Check status of all components
status:
    @echo "=== Minikube Status ==="
    @minikube status --profile={{MINIKUBE_PROFILE}} || echo "Minikube not running"
    @echo ""
    @echo "=== Namespace ==="
    @kubectl get namespace {{NAMESPACE}} 2>/dev/null || echo "Namespace not found"
    @echo ""
    @echo "=== Pods ==="
    @kubectl get pods -n {{NAMESPACE}} 2>/dev/null || echo "No pods found"
    @echo ""
    @echo "=== Services ==="
    @kubectl get services -n {{NAMESPACE}} 2>/dev/null || echo "No services found"

# Restart webhook
restart-webhook:
    @echo "Restarting webhook..."
    kubectl rollout restart deployment/external-dns-webhook-technitium -n {{NAMESPACE}}
    kubectl rollout status deployment/external-dns-webhook-technitium -n {{NAMESPACE}}

# Restart external-dns
restart-external-dns:
    @echo "Restarting external-dns..."
    kubectl rollout restart deployment/external-dns -n {{NAMESPACE}}
    kubectl rollout status deployment/external-dns -n {{NAMESPACE}}

# Restart all services
restart: restart-webhook restart-external-dns
    @echo "All services restarted!"

# Clean up all deployments
clean:
    @echo "Cleaning up deployments..."
    -kubectl delete -f deploy/external-dns/deployment.yaml
    -kubectl delete -f deploy/kubernetes/webhook-deployment.yaml
    -kubectl delete -f deploy/technitium/deployment.yaml
    @echo "Cleanup complete!"

# Full cleanup including minikube
clean-all: clean minikube-delete
    @echo "Full cleanup complete!"

# Rebuild and redeploy webhook
redeploy-webhook: docker-build
    @echo "Redeploying webhook..."
    kubectl delete pod -l app=external-dns-webhook-technitium -n {{NAMESPACE}}
    kubectl wait --for=condition=ready pod -l app=external-dns-webhook-technitium -n {{NAMESPACE}} --timeout=120s
    @echo "Webhook redeployed!"

# Create a test service to verify DNS records
test-service:
    @echo "Creating test service..."
    kubectl create namespace test-app 2>/dev/null || true
    kubectl apply -f deploy/test/test-service.yaml
    @echo "Test service created!"
    @echo ""
    @echo "Monitor DNS record creation with:"
    @echo "  just logs-webhook"
    @echo "  just logs-external-dns"

# Create test service with ingress
test-service-with-ingress: test-service deploy-test-ingress
    @echo ""
    @echo "Test service and ingress created!"
    @echo "Once DNS propagates, access at: http://test.192.168.49.2.nip.io"

# Delete test service
delete-test-service:
    @echo "Deleting test service..."
    kubectl delete namespace test-app --ignore-not-found=true
    @echo "Test service deleted!"

# Open Technitium UI in browser
open-technitium:
    @echo "Opening Technitium UI..."
    @echo "You may need to run 'just port-forward-technitium' first"
    xdg-open http://localhost:5380 2>/dev/null || open http://localhost:5380 2>/dev/null || echo "Please open http://localhost:5380 in your browser"

# Get minikube IP
minikube-ip:
    @minikube ip --profile={{MINIKUBE_PROFILE}}

# Show all ingress URLs
show-urls:
    @echo "=== Ingress URLs ==="
    @echo "Technitium DNS UI: http://dns.192.168.49.2.nip.io"
    @echo "Test Application:  http://test.192.168.49.2.nip.io"
    @echo ""
    @echo "Note: Replace 192.168.49.2 with your actual minikube IP if different"
    @echo "Get minikube IP with: just minikube-ip"

# Show environment info
info:
    @echo "=== Environment Information ==="
    @echo "Minikube Profile: {{MINIKUBE_PROFILE}}"
    @echo "Docker Image: {{DOCKER_IMAGE}}"
    @echo "Namespace: {{NAMESPACE}}"
    @echo ""
    @echo "=== Go Version ==="
    @go version
    @echo ""
    @echo "=== Docker Version ==="
    @docker --version
    @echo ""
    @echo "=== Kubectl Version ==="
    @kubectl version --client --short 2>/dev/null || kubectl version --client
    @echo ""
    @echo "=== Minikube Version ==="
    @minikube version --short 2>/dev/null || minikube version
