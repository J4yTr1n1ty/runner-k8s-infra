package deployment

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
)

type Manager struct {
	kubeClient   kubernetes.Interface
	namespace    string
	templatePath string
}

type DeploymentConfig struct {
	OrgName     string
	GitHubToken string
	Replicas    int32
}

func NewManager(kubeClient kubernetes.Interface, namespace, templatePath string) *Manager {
	return &Manager{
		kubeClient:   kubeClient,
		namespace:    namespace,
		templatePath: templatePath,
	}
}

func (m *Manager) CreateOrUpdateDeployment(ctx context.Context, config *DeploymentConfig) error {
	deploymentName := fmt.Sprintf("github-runner-%s", config.OrgName)

	// Check if deployment already exists
	_, err := m.kubeClient.AppsV1().Deployments(m.namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err == nil {
		// Deployment exists, just update replicas if different
		return m.updateDeploymentReplicas(ctx, deploymentName, config.Replicas)
	}

	// Create new deployment from template
	return m.createDeploymentFromTemplate(ctx, config)
}

func (m *Manager) updateDeploymentReplicas(ctx context.Context, deploymentName string, replicas int32) error {
	deployment, err := m.kubeClient.AppsV1().Deployments(m.namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get deployment %s: %w", deploymentName, err)
	}

	if *deployment.Spec.Replicas != replicas {
		deployment.Spec.Replicas = &replicas
		_, err = m.kubeClient.AppsV1().Deployments(m.namespace).Update(ctx, deployment, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update deployment %s replicas: %w", deploymentName, err)
		}
		klog.Infof("Updated deployment %s replicas to %d", deploymentName, replicas)
	}

	return nil
}

func (m *Manager) createDeploymentFromTemplate(ctx context.Context, config *DeploymentConfig) error {
	// Read template file
	templateContent, err := os.ReadFile(m.templatePath)
	if err != nil {
		return fmt.Errorf("failed to read template file: %w", err)
	}

	// Replace placeholders
	content := strings.ReplaceAll(string(templateContent), "{{ORG_NAME}}", config.OrgName)

	// Base64 encode the token
	tokenB64 := base64.StdEncoding.EncodeToString([]byte(config.GitHubToken))
	content = strings.ReplaceAll(content, "{{ACCESS_TOKEN_B64}}", tokenB64)

	// Parse YAML documents
	docs := strings.Split(content, "---")

	for _, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		// Parse the document to determine its type
		var obj map[string]interface{}
		if err := yaml.Unmarshal([]byte(doc), &obj); err != nil {
			return fmt.Errorf("failed to parse YAML document: %w", err)
		}

		kind, ok := obj["kind"].(string)
		if !ok {
			continue
		}

		switch kind {
		case "Deployment":
			var deployment appsv1.Deployment
			if err := yaml.Unmarshal([]byte(doc), &deployment); err != nil {
				return fmt.Errorf("failed to unmarshal deployment: %w", err)
			}

			// Set the desired replica count
			deployment.Spec.Replicas = &config.Replicas

			// Create deployment
			_, err = m.kubeClient.AppsV1().Deployments(m.namespace).Create(ctx, &deployment, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create deployment: %w", err)
			}
			klog.Infof("Created deployment %s for organization %s", deployment.Name, config.OrgName)

		case "Secret":
			var secret corev1.Secret
			if err := yaml.Unmarshal([]byte(doc), &secret); err != nil {
				return fmt.Errorf("failed to unmarshal secret: %w", err)
			}

			// Create secret
			_, err = m.kubeClient.CoreV1().Secrets(m.namespace).Create(ctx, &secret, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create secret: %w", err)
			}
			klog.Infof("Created secret %s for organization %s", secret.Name, config.OrgName)
		}
	}

	return nil
}

func (m *Manager) DeleteDeployment(ctx context.Context, orgName string) error {
	deploymentName := fmt.Sprintf("github-runner-%s", orgName)
	secretName := fmt.Sprintf("github-runner-token-%s", orgName)

	// Delete deployment
	err := m.kubeClient.AppsV1().Deployments(m.namespace).Delete(ctx, deploymentName, metav1.DeleteOptions{})
	if err != nil {
		klog.Errorf("Failed to delete deployment %s: %v", deploymentName, err)
	}

	// Delete secret
	err = m.kubeClient.CoreV1().Secrets(m.namespace).Delete(ctx, secretName, metav1.DeleteOptions{})
	if err != nil {
		klog.Errorf("Failed to delete secret %s: %v", secretName, err)
	}

	klog.Infof("Deleted deployment and secret for organization %s", orgName)
	return nil
}

func (m *Manager) GetDeploymentPath() string {
	dir := filepath.Dir(m.templatePath)
	return filepath.Join(dir, "..")
}
