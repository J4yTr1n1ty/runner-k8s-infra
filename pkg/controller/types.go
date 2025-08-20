package controller

import (
	"sync"
	"time"

	"github.com/J4yTr1n1ty/runner-k8s-infra/pkg/config"
	"github.com/J4yTr1n1ty/runner-k8s-infra/pkg/deployment"
	"github.com/google/go-github/v57/github"
	"k8s.io/client-go/kubernetes"
)

// RunnerStatus tracks the status of runners for an organization/repo
type RunnerStatus struct {
	Name            string
	CurrentReplicas int32
	PendingJobs     int
	LastScaleUp     time.Time
	LastScaleDown   time.Time
}

// Controller manages GitHub runner scaling
type Controller struct {
	config            *config.Config
	githubClient      *github.Client
	kubeClient        kubernetes.Interface
	deploymentManager *deployment.Manager
	runnerStatus      map[string]*RunnerStatus
	mutex             sync.Mutex
}
