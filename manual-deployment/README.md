# GitHub Actions Runners on Kubernetes (microk8s)

A simple approach to deploying GitHub Actions runners on Kubernetes (microk8s) for multiple organizations.

## Overview

This repository contains Kubernetes manifests for deploying self-hosted GitHub Actions runners. It addresses the limitation that GitHub Actions runners cannot be shared between organizations by deploying separate runners for each organization.

## Prerequisites

- A Kubernetes cluster (microk8s) up and running
- `kubectl` configured to access your cluster
- GitHub personal access tokens for each organization (with `admin:org` scope)
- Docker installed on your Kubernetes nodes

## Setup Instructions

### 1. Clone the Repository

```bash
git clone https://github.com/J4yTr1n1ty/runner-k8s-infra.git
cd runner-k8s-infra
```

### 2. Make the Deployment Script Executable

```bash
chmod +x deploy-runner.sh
```

### 3. Deploy a Runner for an Organization

To deploy a runner for a specific GitHub organization:

```bash
./deploy-runner.sh <org-name> <github-token>
```

Example:

```bash
./deploy-runner.sh myorg ghp_1234567890abcdef
```

This will:

- Create a Kubernetes deployment for the GitHub runner
- Create a secret with your GitHub token
- Configure the runner to register with your organization

### 4. Verify the Deployment

Check that the runner pod is running:

```bash
kubectl get pods -l app=github-runner
```

You should also see the runner appear in your GitHub organization's settings under "Actions > Runners".

## Manually Scaling Runners

To scale the number of runners for an organization:

```bash
kubectl scale deployment github-runner-<org-name> --replicas=<number>
```

Example:

```bash
kubectl scale deployment github-runner-myorg --replicas=3
```

## Stopping Runners

To stop runners for an organization:

```bash
kubectl scale deployment github-runner-<org-name> --replicas=0
```

Or to completely remove the deployment:

```bash
kubectl delete deployment github-runner-<org-name>
kubectl delete secret github-runner-token-<org-name>
```

## Understanding the Runner Configuration

The GitHub runner deployment uses the [myoung34/github-runner](https://github.com/myoung34/docker-github-actions-runner) container image which:

1. Automatically registers with GitHub when started
2. Automatically de-registers when stopped
3. Uses the provided GitHub token for authentication
4. Applies custom labels for targeting workflows to this runner

## Troubleshooting

If the runners aren't appearing in GitHub:

1. Check the pod logs:

   ```bash
   kubectl logs -l app=github-runner,org=<org-name>
   ```

2. Verify your GitHub token has the correct permissions

3. Check if the pod can connect to GitHub's servers

## Next Steps

For a more sophisticated solution, consider:

1. Creating a controller to automatically scale runners based on workflow demand
2. Setting up runner pools with different resource configurations
3. Adding monitoring and alerting for runner health
