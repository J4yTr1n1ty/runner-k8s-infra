package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/J4yTr1n1ty/runner-k8s-infra/pkg/config"
	"github.com/J4yTr1n1ty/runner-k8s-infra/pkg/deployment"
	"github.com/J4yTr1n1ty/runner-k8s-infra/pkg/github"
	"github.com/J4yTr1n1ty/runner-k8s-infra/pkg/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

// NewController creates a new controller instance
func NewController(cfg *config.Config) (*Controller, error) {
	// Create GitHub client
	githubClient := github.NewClient(cfg.GitHub.Token)

	// Create Kubernetes client
	kubeClient, err := kubernetes.NewClient(cfg.Kubernetes.KubeConfig, cfg.Kubernetes.Namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Get absolute template path
	templatePath, err := cfg.GetAbsoluteTemplatePath()
	if err != nil {
		return nil, fmt.Errorf("failed to get template path: %w", err)
	}

	// Create deployment manager
	deploymentManager := deployment.NewManager(kubeClient.Clientset, cfg.Kubernetes.Namespace, templatePath)

	// Initialize runner status map
	runnerStatus := make(map[string]*RunnerStatus)
	for _, org := range cfg.GitHub.Organizations {
		runnerStatus[org] = &RunnerStatus{
			Name:            org,
			CurrentReplicas: 0,
			PendingJobs:     0,
		}
	}

	for _, repo := range cfg.GitHub.PersonalRepos {
		runnerStatus[repo] = &RunnerStatus{
			Name:            repo,
			CurrentReplicas: 0,
			PendingJobs:     0,
		}
	}

	return &Controller{
		config:            cfg,
		githubClient:      githubClient.Client,
		kubeClient:        kubeClient.Clientset,
		deploymentManager: deploymentManager,
		runnerStatus:      runnerStatus,
	}, nil
}

// Start begins the controller's main loop
func (c *Controller) Start(ctx context.Context) error {
	klog.Info("Starting GitHub Runner Controller")

	// Initialize by getting current deployments state
	if err := c.syncRunnerState(ctx); err != nil {
		return fmt.Errorf("failed to sync initial runner state: %w", err)
	}

	pollInterval, err := c.config.GetPollInterval()
	if err != nil {
		return fmt.Errorf("invalid poll interval: %w", err)
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			klog.Info("Shutting down GitHub Runner Controller")
			return nil
		case <-ticker.C:
			if err := c.reconcile(ctx); err != nil {
				klog.Errorf("Error during reconciliation: %v", err)
			}
		}
	}
}

// syncRunnerState gets the current state of runner deployments
func (c *Controller) syncRunnerState(ctx context.Context) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	deployments, err := c.kubeClient.AppsV1().Deployments(c.config.Kubernetes.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=github-runner",
	})
	if err != nil {
		return fmt.Errorf("failed to list deployments: %w", err)
	}

	for _, deployment := range deployments.Items {
		org, ok := deployment.Labels["org"]
		if !ok {
			continue
		}

		if status, exists := c.runnerStatus[org]; exists {
			status.CurrentReplicas = *deployment.Spec.Replicas
		}
	}

	return nil
}

// reconcile is the main reconciliation loop
func (c *Controller) reconcile(ctx context.Context) error {
	// Get pending workflows for each organization/repo
	if err := c.updatePendingJobs(ctx); err != nil {
		return fmt.Errorf("failed to update pending jobs: %w", err)
	}

	// Decide on scaling actions
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for name, status := range c.runnerStatus {
		desiredReplicas := c.calculateDesiredReplicas(status)
		currentReplicas := status.CurrentReplicas

		if desiredReplicas != currentReplicas {
			if err := c.ensureDeploymentExists(ctx, name, desiredReplicas); err != nil {
				klog.Errorf("Failed to ensure deployment exists for %s: %v", name, err)
				continue
			}

			status.CurrentReplicas = desiredReplicas
			if desiredReplicas > currentReplicas {
				status.LastScaleUp = time.Now()
			} else {
				status.LastScaleDown = time.Now()
			}

			klog.Infof("Scaled deployment for %s from %d to %d replicas", name, currentReplicas, desiredReplicas)
		}
	}

	return nil
}

// updatePendingJobs gets the count of pending workflow runs
func (c *Controller) updatePendingJobs(ctx context.Context) error {
	ghClient := github.NewClient(c.config.GitHub.Token)

	// Check organizations
	for _, org := range c.config.GitHub.Organizations {
		status, ok := c.runnerStatus[org]
		if !ok {
			continue
		}

		pendingRuns, err := ghClient.GetPendingWorkflows(ctx, org, "")
		if err != nil {
			return fmt.Errorf("failed to get pending workflow runs for org %s: %w", org, err)
		}

		c.mutex.Lock()
		status.PendingJobs = pendingRuns
		c.mutex.Unlock()
	}

	// Check personal repos
	for _, repoFullName := range c.config.GitHub.PersonalRepos {
		parts := strings.Split(repoFullName, "/")
		if len(parts) != 2 {
			klog.Warningf("Invalid repo format: %s, expected owner/repo", repoFullName)
			continue
		}

		owner := parts[0]
		repo := parts[1]

		status, ok := c.runnerStatus[repoFullName]
		if !ok {
			continue
		}

		pendingRuns, err := ghClient.GetPendingWorkflows(ctx, owner, repo)
		if err != nil {
			return fmt.Errorf("failed to get pending workflow runs for repo %s: %w", repoFullName, err)
		}

		c.mutex.Lock()
		status.PendingJobs = pendingRuns
		c.mutex.Unlock()
	}

	return nil
}

// calculateDesiredReplicas determines how many runners are needed
func (c *Controller) calculateDesiredReplicas(status *RunnerStatus) int32 {
	// Basic calculation: one runner per job, respecting min/max limits
	desiredReplicas := int32(float64(status.PendingJobs) * c.config.Controller.RunnersPerJob)

	// Add at least one runner if there are any pending jobs
	if status.PendingJobs > 0 && desiredReplicas == 0 {
		desiredReplicas = 1
	}

	// Apply min/max constraints
	if desiredReplicas < c.config.Controller.MinRunners {
		desiredReplicas = c.config.Controller.MinRunners
	}
	if desiredReplicas > c.config.Controller.MaxRunners {
		desiredReplicas = c.config.Controller.MaxRunners
	}

	// Apply scale down delay
	if desiredReplicas < status.CurrentReplicas {
		scaleDownDelay, _ := c.config.GetScaleDownDelay()
		scaleDownInterval, _ := c.config.GetScaleDownInterval()

		timeSinceScaleUp := time.Since(status.LastScaleUp)
		if timeSinceScaleUp < scaleDownDelay {
			// Too soon to scale down after a scale up
			return status.CurrentReplicas
		}

		// Apply gradual scale down
		timeSinceScaleDown := time.Since(status.LastScaleDown)
		if timeSinceScaleDown < scaleDownInterval {
			// Too soon for another scale down, reduce by 1 at most
			if status.CurrentReplicas > 0 {
				return status.CurrentReplicas - 1
			}
			return 0
		}
	}

	return desiredReplicas
}

// ensureDeploymentExists creates or updates a deployment for an organization
func (c *Controller) ensureDeploymentExists(ctx context.Context, orgName string, replicas int32) error {
	deploymentConfig := &deployment.DeploymentConfig{
		OrgName:     orgName,
		GitHubToken: c.config.GitHub.Token,
		Replicas:    replicas,
	}

	return c.deploymentManager.CreateOrUpdateDeployment(ctx, deploymentConfig)
}
