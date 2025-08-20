package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/J4yTr1n1ty/runner-k8s-infra/pkg/deployment"
	"github.com/J4yTr1n1ty/runner-k8s-infra/pkg/kubernetes"
	"k8s.io/klog/v2"
)

func main() {
	var (
		orgName      = flag.String("org", "", "Organization name (required)")
		githubToken  = flag.String("token", "", "GitHub token (defaults to GITHUB_TOKEN env var)")
		replicas     = flag.Int("replicas", 1, "Number of runner replicas")
		namespace    = flag.String("namespace", "github-runners", "Kubernetes namespace")
		templatePath = flag.String("template", "deploy/runner-deployment-template.yaml", "Path to deployment template")
		kubeconfig   = flag.String("kubeconfig", "", "Path to kubeconfig file")
	)
	flag.Parse()

	if *orgName == "" {
		fmt.Fprintf(os.Stderr, "Usage: %s -org <organization-name> [options]\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Get GitHub token from environment if not provided
	token := *githubToken
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
		if token == "" {
			fmt.Fprintf(os.Stderr, "GitHub token is required. Set GITHUB_TOKEN env var or use -token flag\n")
			os.Exit(1)
		}
	}

	// Create Kubernetes client
	kubeClient, err := kubernetes.NewClient(*kubeconfig, *namespace)
	if err != nil {
		klog.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	// Create deployment manager
	deploymentManager := deployment.NewManager(kubeClient.Clientset, *namespace, *templatePath)

	// Create deployment
	ctx := context.Background()
	deploymentConfig := &deployment.DeploymentConfig{
		OrgName:     *orgName,
		GitHubToken: token,
		Replicas:    int32(*replicas),
	}

	if err := deploymentManager.CreateOrUpdateDeployment(ctx, deploymentConfig); err != nil {
		klog.Fatalf("Failed to create/update deployment: %v", err)
	}

	fmt.Printf("Successfully deployed GitHub runner for organization '%s' with %d replicas\n", *orgName, *replicas)
	fmt.Printf("Check status with: kubectl get pods -l org=%s -n %s\n", *orgName, *namespace)
}
