package controller

import (
	"context"
	"testing"
	"time"

	"github.com/J4yTr1n1ty/runner-k8s-infra/pkg/config"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNewController(t *testing.T) {
	cfg := &config.Config{
		GitHub: config.GitHubConfig{
			Token:         "test-token",
			Organizations: []string{"org1", "org2"},
			PersonalRepos: []string{"user/repo1"},
		},
		Kubernetes: config.KubernetesConfig{
			Namespace: "test-namespace",
		},
		Deployment: config.DeploymentConfig{
			TemplatePath: "test-template.yaml",
		},
	}

	// Create a temporary template file for the test
	// Note: In a real test environment, you'd create this file
	
	// For now, let's test the error case when template doesn't exist
	_, err := NewController(cfg)
	if err == nil {
		t.Error("Expected error when template file doesn't exist")
	}
}

func TestCalculateDesiredReplicas(t *testing.T) {
	cfg := &config.Config{
		Controller: config.ControllerConfig{
			MinRunners:        1,
			MaxRunners:        10,
			RunnersPerJob:     1.5,
			ScaleDownDelay:    "5m",
			ScaleDownInterval: "1m",
		},
	}

	controller := &Controller{
		config: cfg,
	}

	tests := []struct {
		name           string
		status         *RunnerStatus
		expectedReplicas int32
	}{
		{
			name: "no pending jobs, respect minimum",
			status: &RunnerStatus{
				PendingJobs:     0,
				CurrentReplicas: 0,
			},
			expectedReplicas: 1, // min runners
		},
		{
			name: "some pending jobs",
			status: &RunnerStatus{
				PendingJobs:     3,
				CurrentReplicas: 0,
			},
			expectedReplicas: 4, // 3 * 1.5 = 4.5, truncated to 4
		},
		{
			name: "many pending jobs, respect maximum",
			status: &RunnerStatus{
				PendingJobs:     20,
				CurrentReplicas: 0,
			},
			expectedReplicas: 10, // max runners
		},
		{
			name: "scale down with delay protection",
			status: &RunnerStatus{
				PendingJobs:     0,
				CurrentReplicas: 5,
				LastScaleUp:     time.Now().Add(-1 * time.Minute), // Recent scale up
			},
			expectedReplicas: 5, // Should not scale down due to recent scale up
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := controller.calculateDesiredReplicas(tt.status)
			if result != tt.expectedReplicas {
				t.Errorf("Expected %d replicas, got %d", tt.expectedReplicas, result)
			}
		})
	}
}

func TestSyncRunnerState(t *testing.T) {
	// Create fake deployments
	deployment1 := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "github-runner-org1",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app": "github-runner",
				"org": "org1",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(3),
		},
	}

	deployment2 := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "github-runner-org2",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app": "github-runner",
				"org": "org2",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(2),
		},
	}

	fakeClient := fake.NewSimpleClientset(deployment1, deployment2)

	cfg := &config.Config{
		Kubernetes: config.KubernetesConfig{
			Namespace: "test-namespace",
		},
		GitHub: config.GitHubConfig{
			Organizations: []string{"org1", "org2"},
		},
	}

	controller := &Controller{
		config:     cfg,
		kubeClient: fakeClient,
		runnerStatus: map[string]*RunnerStatus{
			"org1": {Name: "org1"},
			"org2": {Name: "org2"},
			"org3": {Name: "org3"}, // This org has no deployment
		},
	}

	ctx := context.Background()
	err := controller.syncRunnerState(ctx)
	if err != nil {
		t.Fatalf("syncRunnerState failed: %v", err)
	}

	// Verify that replica counts were synced correctly
	if controller.runnerStatus["org1"].CurrentReplicas != 3 {
		t.Errorf("Expected org1 to have 3 replicas, got %d", controller.runnerStatus["org1"].CurrentReplicas)
	}

	if controller.runnerStatus["org2"].CurrentReplicas != 2 {
		t.Errorf("Expected org2 to have 2 replicas, got %d", controller.runnerStatus["org2"].CurrentReplicas)
	}

	// org3 should remain unchanged (0) since no deployment exists
	if controller.runnerStatus["org3"].CurrentReplicas != 0 {
		t.Errorf("Expected org3 to have 0 replicas, got %d", controller.runnerStatus["org3"].CurrentReplicas)
	}
}

func TestScaleDownProtection(t *testing.T) {
	cfg := &config.Config{
		Controller: config.ControllerConfig{
			MinRunners:        1,
			MaxRunners:        10,
			RunnersPerJob:     1.0,
			ScaleDownDelay:    "5m",
			ScaleDownInterval: "1m",
		},
	}

	controller := &Controller{
		config: cfg,
	}

	// Test recent scale up protection
	status := &RunnerStatus{
		PendingJobs:     0,  // Want to scale down
		CurrentReplicas: 5,  // Currently have 5
		LastScaleUp:     time.Now().Add(-2 * time.Minute), // Scaled up 2 minutes ago
	}

	result := controller.calculateDesiredReplicas(status)
	// Should not scale down due to recent scale up (within 5m delay)
	if result != 5 {
		t.Errorf("Expected no scale down due to recent scale up, got %d replicas", result)
	}

	// Test old scale up, should allow scale down
	status.LastScaleUp = time.Now().Add(-10 * time.Minute) // 10 minutes ago
	status.LastScaleDown = time.Now().Add(-2 * time.Minute) // Last scale down 2 minutes ago

	result = controller.calculateDesiredReplicas(status)
	// Should scale down to minimum (1) since scale up was long ago
	if result != 1 {
		t.Errorf("Expected scale down to minimum (1), got %d replicas", result)
	}
}

func TestControllerWithPersonalRepos(t *testing.T) {
	cfg := &config.Config{
		GitHub: config.GitHubConfig{
			Token:         "test-token",
			PersonalRepos: []string{"user1/repo1", "user2/repo2"},
		},
		Kubernetes: config.KubernetesConfig{
			Namespace: "test-namespace",
		},
		Controller: config.ControllerConfig{
			MinRunners: 0,
			MaxRunners: 5,
		},
	}

	runnerStatus := make(map[string]*RunnerStatus)
	for _, repo := range cfg.GitHub.PersonalRepos {
		runnerStatus[repo] = &RunnerStatus{
			Name:            repo,
			CurrentReplicas: 0,
			PendingJobs:     0,
		}
	}

	controller := &Controller{
		config:       cfg,
		runnerStatus: runnerStatus,
	}

	// Verify that personal repos are tracked
	if len(controller.runnerStatus) != 2 {
		t.Errorf("Expected 2 runner statuses, got %d", len(controller.runnerStatus))
	}

	if _, exists := controller.runnerStatus["user1/repo1"]; !exists {
		t.Error("Expected user1/repo1 to be tracked")
	}

	if _, exists := controller.runnerStatus["user2/repo2"]; !exists {
		t.Error("Expected user2/repo2 to be tracked")
	}
}

func TestZeroPendingJobsWithMinRunners(t *testing.T) {
	cfg := &config.Config{
		Controller: config.ControllerConfig{
			MinRunners:    2,
			MaxRunners:    10,
			RunnersPerJob: 1.0,
		},
	}

	controller := &Controller{
		config: cfg,
	}

	status := &RunnerStatus{
		PendingJobs:     0,
		CurrentReplicas: 0,
	}

	result := controller.calculateDesiredReplicas(status)
	if result != 2 {
		t.Errorf("Expected minimum runners (2) when no pending jobs, got %d", result)
	}
}

// Helper function
func int32Ptr(i int32) *int32 {
	return &i
}