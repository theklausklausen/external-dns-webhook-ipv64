# Quick Start Guide

This guide will help you get started with external-dns-webhook-ipv64 quickly.

## Prerequisites

1. A Kubernetes cluster (minikube, kind, or production cluster)
2. kubectl configured to access your cluster
3. An ipv64.net account with an API key
4. A domain registered with ipv64.net

## Step 1: Get Your IPv64 API Key

1. Log in to your [ipv64.net account](https://ipv64.net)
2. Navigate to the API settings
3. Copy your API key

## Step 2: Create Kubernetes Secret

Create a secret containing your ipv64 API key:

```bash
kubectl create namespace external-dns

kubectl create secret generic ipv64-api-key \
  --from-literal=api-key=YOUR_API_KEY_HERE \
  -n external-dns
```

## Step 3: Deploy the Webhook

```bash
# Clone the repository
git clone https://github.com/klausklausen/external-dns-webhook-ipv64.git
cd external-dns-webhook-ipv64

# Apply the webhook deployment
kubectl apply -f deploy/kubernetes/webhook-deployment.yaml

# Verify the webhook is running
kubectl get pods -n external-dns -l app=external-dns-webhook-ipv64
```

## Step 4: Deploy external-dns

```bash
# Apply the external-dns deployment
kubectl apply -f deploy/external-dns/deployment.yaml

# Verify external-dns is running
kubectl get pods -n external-dns -l app=external-dns
```

## Step 5: Test with a Sample Service

Create a test service with DNS annotation:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  name: test-service
  annotations:
    external-dns.alpha.kubernetes.io/hostname: test.your-domain.com
spec:
  type: LoadBalancer
  ports:
  - port: 80
    targetPort: 80
  selector:
    app: nginx
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:alpine
        ports:
        - containerPort: 80
EOF
```

## Step 6: Verify DNS Records

Check the logs to see if the DNS record was created:

```bash
# Check webhook logs
kubectl logs -n external-dns -l app=external-dns-webhook-ipv64

# Check external-dns logs
kubectl logs -n external-dns -l app=external-dns
```

You should see log entries indicating that the DNS record was created.

## Step 7: Verify in IPv64 Dashboard

1. Log in to your ipv64.net account
2. Navigate to your domains
3. You should see the DNS record for `test.your-domain.com`

## Troubleshooting

### Webhook pod not starting

Check the logs:
```bash
kubectl logs -n external-dns -l app=external-dns-webhook-ipv64
```

Common issues:
- Missing or invalid API key
- API key secret not created in the correct namespace

### DNS records not being created

1. Verify external-dns can reach the webhook:
```bash
kubectl logs -n external-dns -l app=external-dns | grep webhook
```

2. Check if the service has the correct annotation:
```bash
kubectl get service test-service -o yaml | grep external-dns
```

3. Verify domain filter (if configured):
```bash
kubectl get deployment -n external-dns external-dns-webhook-ipv64 -o yaml | grep DOMAIN_FILTER
```

### API Rate Limits

If you're hitting rate limits:
- Check your account limits in the ipv64.net dashboard
- Ensure `FREE_ACCOUNT` is set correctly (`true` for free accounts, `false` for paid accounts)
- Increase the external-dns sync interval
- Consider upgrading your ipv64.net account

## Configuration Options

### Domain Filter

To only manage specific domains, set the DOMAIN_FILTER environment variable:

```yaml
env:
  - name: DOMAIN_FILTER
    value: "example.com,another-domain.com"
```

### Dry Run Mode

To test without making actual changes:

```yaml
env:
  - name: DRY_RUN
    value: "true"
```

### Free vs Paid Account Rate Limiting

The webhook applies a built-in client-side limiter based on `FREE_ACCOUNT`:

```yaml
env:
  - name: FREE_ACCOUNT
    value: "true"
```

- `FREE_ACCOUNT=true` (default): 1 request every 3 minutes
- `FREE_ACCOUNT=false`: 1 request every 5 seconds

### Log Level

For more detailed logging:

```yaml
env:
  - name: LOG_LEVEL
    value: "debug"
```

## Next Steps

- Read the [API documentation](API.md)
- Configure domain filters for production use
- Set up monitoring and alerting
- Review security best practices in the main README

## Clean Up

To remove all resources:

```bash
kubectl delete -f deploy/external-dns/deployment.yaml
kubectl delete -f deploy/kubernetes/webhook-deployment.yaml
kubectl delete secret ipv64-api-key -n external-dns
kubectl delete namespace external-dns
```
