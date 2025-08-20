# GitHub Actions Runners on Kubernetes

A sophisticated solution for deploying and auto-scaling self-hosted GitHub Actions runners on Kubernetes. This project provides both manual deployment tools and an intelligent controller that automatically scales runners based on workflow demand.

## Features

- **Automated Scaling**: Intelligent controller that monitors GitHub workflow queues and scales runners automatically
- **Multi-Organization Support**: Deploy runners for multiple GitHub organizations or personal repositories
- **Manual Deployment**: Simple script and CLI tools for manual runner deployment
- **Template-Based Configuration**: Flexible deployment templates for customization
- **Graceful Scaling**: Smart scale-down logic to prevent unnecessary churn

## Architecture

The project consists of three main components:

1. **Controller** (`cmd/controller`): Monitors GitHub workflows and automatically scales runners
2. **Deploy Runner CLI** (`cmd/deploy-runner`): Command-line tool for manual runner deployment
3. **Deploy Script** (`bin/deploy-runner.sh`): Bash script for quick manual deployments

## Quick Start

### Prerequisites

- Kubernetes cluster (tested with microk8s)
- `kubectl` configured to access your cluster
- GitHub personal access tokens with `admin:org` scope for each organization
- Go 1.24+ (for building from source)

### Option 1: Manual Deployment (Quick Setup)

For simple manual deployment of runners:

```bash
# Clone and setup
git clone https://github.com/J4yTr1n1ty/runner-k8s-infra.git
cd runner-k8s-infra

# Create namespace
kubectl apply -f deploy/namespace.yaml

# Deploy a runner for an organization
export GITHUB_TOKEN=your_github_token_here
./bin/deploy-runner.sh myorg
```

### Option 2: Automated Controller (Production Setup)

For automatic scaling based on workflow demand:

```bash
# 1. Configure the controller
cp config/config.yaml config/my-config.yaml
# Edit my-config.yaml with your settings

# 2. Build the controller
make build-controller

# 3. Create namespace and apply configuration
kubectl apply -f deploy/namespace.yaml
kubectl create secret generic github-token --from-literal=token=$GITHUB_TOKEN -n github-runners

# 4. Run the controller
./bin/controller -config config/my-config.yaml
```

## Configuration

### Controller Configuration

Create a `config.yaml` file based on the template in `config/config.yaml`:

```yaml
github:
  token: ""  # Set via GITHUB_TOKEN env var
  organizations: 
    - "myorg1"
    - "myorg2"
  personalRepos: 
    - "user/repo1"
    - "user/repo2"
  
kubernetes:
  namespace: "github-runners"
  
controller:
  pollInterval: "30s"
  minRunners: 0
  maxRunners: 10
  scaleDownDelay: "5m"
  runnersPerJob: 1.2
```

### Environment Variables

- `GITHUB_TOKEN`: GitHub personal access token (required)
- `KUBECONFIG`: Path to kubeconfig file (optional, uses in-cluster config if not set)

## Usage Examples

### Deploy a single runner manually

Using the CLI tool:
```bash
./bin/deploy-runner -org myorg -replicas 2
```

Using the bash script:
```bash
./bin/deploy-runner.sh myorg
```

### Scale existing deployment

```bash
kubectl scale deployment github-runner-myorg --replicas=5 -n github-runners
```

### Check runner status

```bash
# Check pods
kubectl get pods -l app=github-runner -n github-runners

# Check deployments
kubectl get deployments -n github-runners

# Check logs
kubectl logs -l org=myorg -n github-runners
```

### Remove runners

```bash
kubectl delete deployment github-runner-myorg -n github-runners
kubectl delete secret github-runner-token-myorg -n github-runners
```

## Building from Source

```bash
# Install dependencies
make install-deps

# Build all components
make build

# Build individual components
make build-controller
make build-deploy-runner

# Run tests
make test

# Format and vet code
make fmt vet
```

## How It Works

### Controller Logic

1. **Polling**: Regularly queries GitHub API for pending workflow runs
2. **Calculation**: Determines required runners using configurable ratios
3. **Scaling**: Creates/updates Kubernetes deployments as needed
4. **Protection**: Implements delays and limits to prevent thrashing

### Runner Configuration

Each runner deployment includes:
- Automatic GitHub registration/deregistration
- Custom labels for workflow targeting
- Resource limits and requests
- Docker socket access for containerized workflows
- Persistent work directories

### Deployment Template

The deployment template (`deploy/runner-deployment-template.yaml`) supports:
- Organization name substitution (`{{ORG_NAME}}`)
- Base64 token substitution (`{{ACCESS_TOKEN_B64}}`)
- Customizable resource limits
- Configurable labels and environment variables

## Monitoring and Troubleshooting

### Check Controller Status

```bash
# View controller logs
kubectl logs -l app=runner-controller -n github-runners

# Check controller configuration
kubectl get configmap controller-config -n github-runners -o yaml
```

### Debug Runner Issues

```bash
# Check runner pod status
kubectl describe pods -l app=github-runner -n github-runners

# View runner logs
kubectl logs -l org=myorg -n github-runners --tail=100

# Check GitHub registration status
kubectl exec -it $(kubectl get pods -l org=myorg -n github-runners -o jsonpath='{.items[0].metadata.name}') -- cat /home/runner/.runner
```

### Common Issues

1. **Runners not appearing in GitHub**: Check token permissions and network connectivity
2. **Pods stuck in Pending**: Check resource availability and node selectors  
3. **Scale events not working**: Verify GitHub API rate limits and webhook configuration

## Advanced Configuration

### Custom Runner Images

Modify the deployment template to use custom runner images:

```yaml
spec:
  template:
    spec:
      containers:
        - name: github-runner
          image: my-custom-runner:latest
```

### Multiple Runner Pools

Deploy different configurations for different use cases:

```bash
# CPU-intensive workloads
./bin/deploy-runner -org myorg-cpu -template deploy/cpu-intensive-template.yaml

# GPU workloads  
./bin/deploy-runner -org myorg-gpu -template deploy/gpu-template.yaml
```

### Resource Quotas

Apply resource quotas to prevent runaway scaling:

```yaml
apiVersion: v1
kind: ResourceQuota
metadata:
  name: github-runners-quota
  namespace: github-runners
spec:
  hard:
    requests.cpu: "20"
    requests.memory: 40Gi
    limits.cpu: "40" 
    limits.memory: 80Gi
```

## Security Considerations

- Store GitHub tokens in Kubernetes secrets, not config files
- Use least-privilege service accounts for controllers
- Regularly rotate GitHub tokens
- Monitor resource usage to prevent abuse
- Consider network policies to restrict runner access

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes with tests
4. Run `make test fmt vet`
5. Submit a pull request

## License

This project is licensed under the MIT License - see the LICENSE file for details.