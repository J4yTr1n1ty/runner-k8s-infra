package kubernetes

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Client wraps Kubernetes API operations
type Client struct {
	Clientset *kubernetes.Clientset
	Namespace string
}

// NewClient creates a new Kubernetes client
func NewClient(kubeconfigPath, namespace string) (*Client, error) {
	var config *rest.Config
	var err error

	if kubeconfigPath == "" {
		// In-cluster configuration
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to create in-cluster config: %w", err)
		}
	} else {
		// Out-of-cluster configuration
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return nil, fmt.Errorf("failed to build config from kubeconfig: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	return &Client{
		Clientset: clientset,
		Namespace: namespace,
	}, nil
}

// GetDeployment retrieves a deployment by name
func (c *Client) GetDeployment(ctx context.Context, name string) (*appsv1.Deployment, error) {
	return c.Clientset.AppsV1().Deployments(c.Namespace).Get(ctx, name, metav1.GetOptions{})
}

// ListDeployments lists deployments with a label selector
func (c *Client) ListDeployments(ctx context.Context, labelSelector string) (*appsv1.DeploymentList, error) {
	return c.Clientset.AppsV1().Deployments(c.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
}

// ScaleDeployment changes the replica count of a deployment
func (c *Client) ScaleDeployment(ctx context.Context, name string, replicas int32) error {
	deployment, err := c.GetDeployment(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to get deployment %s: %w", name, err)
	}

	deployment.Spec.Replicas = &replicas

	_, err = c.Clientset.AppsV1().Deployments(c.Namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update deployment %s: %w", name, err)
	}

	return nil
}
