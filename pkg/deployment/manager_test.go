package deployment

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func TestNewManager(t *testing.T) {
	client := fake.NewSimpleClientset()
	manager := NewManager(client, "test-namespace", "/path/to/template")

	if manager.kubeClient == nil {
		t.Error("Expected kubeClient to be set")
	}
	if manager.namespace != "test-namespace" {
		t.Errorf("Expected namespace 'test-namespace', got '%s'", manager.namespace)
	}
	if manager.templatePath != "/path/to/template" {
		t.Errorf("Expected template path '/path/to/template', got '%s'", manager.templatePath)
	}
}

func TestCreateDeploymentFromTemplate(t *testing.T) {
	// Create a test template
	template := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: github-runner-{{ORG_NAME}}
  namespace: test-namespace
  labels:
    app: github-runner
    org: {{ORG_NAME}}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: github-runner
      org: {{ORG_NAME}}
  template:
    metadata:
      labels:
        app: github-runner
        org: {{ORG_NAME}}
    spec:
      containers:
        - name: github-runner
          image: myoung34/github-runner:latest
          env:
            - name: ORG_NAME
              value: "{{ORG_NAME}}"
            - name: ACCESS_TOKEN
              valueFrom:
                secretKeyRef:
                  name: github-runner-token-{{ORG_NAME}}
                  key: access-token
---
apiVersion: v1
kind: Secret
metadata:
  name: github-runner-token-{{ORG_NAME}}
  namespace: test-namespace
  labels:
    app: github-runner
    org: {{ORG_NAME}}
type: Opaque
data:
  access-token: {{ACCESS_TOKEN_B64}}`

	tmpDir, err := os.MkdirTemp("", "deployment-test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Logf("Failed to remove temp dir: %v", err)
		}
	}()

	templatePath := filepath.Join(tmpDir, "template.yaml")
	if err := os.WriteFile(templatePath, []byte(template), 0644); err != nil {
		t.Fatal(err)
	}

	client := fake.NewSimpleClientset()
	manager := NewManager(client, "test-namespace", templatePath)

	config := &DeploymentConfig{
		OrgName:     "testorg",
		GitHubToken: "test-token",
		Replicas:    2,
	}

	ctx := context.Background()
	err = manager.createDeploymentFromTemplate(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create deployment from template: %v", err)
	}

	// Verify deployment was created
	deployments, err := client.AppsV1().Deployments("test-namespace").List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if len(deployments.Items) != 1 {
		t.Fatalf("Expected 1 deployment, got %d", len(deployments.Items))
	}

	deployment := deployments.Items[0]
	if deployment.Name != "github-runner-testorg" {
		t.Errorf("Expected deployment name 'github-runner-testorg', got '%s'", deployment.Name)
	}

	if *deployment.Spec.Replicas != 2 {
		t.Errorf("Expected 2 replicas, got %d", *deployment.Spec.Replicas)
	}

	if deployment.Labels["org"] != "testorg" {
		t.Errorf("Expected org label 'testorg', got '%s'", deployment.Labels["org"])
	}

	// Verify secret was created
	secrets, err := client.CoreV1().Secrets("test-namespace").List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if len(secrets.Items) != 1 {
		t.Fatalf("Expected 1 secret, got %d", len(secrets.Items))
	}

	secret := secrets.Items[0]
	if secret.Name != "github-runner-token-testorg" {
		t.Errorf("Expected secret name 'github-runner-token-testorg', got '%s'", secret.Name)
	}

	// Verify token is stored correctly (Kubernetes secrets automatically handle base64)
	actualToken := string(secret.Data["access-token"])
	if actualToken != "test-token" {
		t.Errorf("Token not stored correctly. Expected: test-token, Got: %s", actualToken)
	}
}

func TestUpdateDeploymentReplicas(t *testing.T) {
	// Create existing deployment
	existingDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "test-namespace",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(3),
		},
	}

	client := fake.NewSimpleClientset(existingDeployment)
	manager := NewManager(client, "test-namespace", "/path/to/template")

	ctx := context.Background()
	err := manager.updateDeploymentReplicas(ctx, "test-deployment", 5)
	if err != nil {
		t.Fatalf("Failed to update deployment replicas: %v", err)
	}

	// Verify replicas were updated
	deployment, err := client.AppsV1().Deployments("test-namespace").Get(ctx, "test-deployment", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if *deployment.Spec.Replicas != 5 {
		t.Errorf("Expected 5 replicas, got %d", *deployment.Spec.Replicas)
	}
}

func TestUpdateDeploymentReplicasNoChange(t *testing.T) {
	// Create existing deployment with target replicas
	existingDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "test-namespace",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(3),
		},
	}

	client := fake.NewSimpleClientset(existingDeployment)
	var updateCalled bool
	client.PrependReactor("update", "deployments", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		updateCalled = true
		return false, nil, nil
	})

	manager := NewManager(client, "test-namespace", "/path/to/template")

	ctx := context.Background()
	err := manager.updateDeploymentReplicas(ctx, "test-deployment", 3)
	if err != nil {
		t.Fatalf("Failed to update deployment replicas: %v", err)
	}

	// Verify no update was called since replicas are the same
	if updateCalled {
		t.Error("Expected no update call when replicas are unchanged")
	}
}

func TestCreateOrUpdateDeployment(t *testing.T) {
	tests := []struct {
		name               string
		existingDeployment *appsv1.Deployment
		config             *DeploymentConfig
		expectCreate       bool
		expectUpdate       bool
	}{
		{
			name:         "create new deployment",
			config:       &DeploymentConfig{OrgName: "neworg", GitHubToken: "token", Replicas: 1},
			expectCreate: true,
		},
		{
			name: "update existing deployment",
			existingDeployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "github-runner-existingorg",
					Namespace: "test-namespace",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: int32Ptr(1),
				},
			},
			config:       &DeploymentConfig{OrgName: "existingorg", GitHubToken: "token", Replicas: 3},
			expectUpdate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objects []runtime.Object
			if tt.existingDeployment != nil {
				objects = append(objects, tt.existingDeployment)
			}

			client := fake.NewSimpleClientset(objects...)

			// Create a simple template for create tests
			if tt.expectCreate {
				tmpDir, err := os.MkdirTemp("", "deployment-test")
				if err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := os.RemoveAll(tmpDir); err != nil {
						t.Logf("Failed to remove temp dir: %v", err)
					}
				}()

				template := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: github-runner-{{ORG_NAME}}
  namespace: test-namespace
spec:
  replicas: 1
  selector:
    matchLabels:
      app: github-runner
  template:
    metadata:
      labels:
        app: github-runner
    spec:
      containers:
        - name: github-runner
          image: test:latest`

				templatePath := filepath.Join(tmpDir, "template.yaml")
				if err := os.WriteFile(templatePath, []byte(template), 0644); err != nil {
					t.Fatal(err)
				}

				manager := NewManager(client, "test-namespace", templatePath)

				ctx := context.Background()
				err = manager.CreateOrUpdateDeployment(ctx, tt.config)
				if err != nil {
					t.Fatalf("Failed to create/update deployment: %v", err)
				}
			} else {
				manager := NewManager(client, "test-namespace", "/fake/template")

				ctx := context.Background()
				err := manager.CreateOrUpdateDeployment(ctx, tt.config)
				if err != nil {
					t.Fatalf("Failed to create/update deployment: %v", err)
				}
			}

			// Verify expected outcome
			if tt.expectCreate || tt.expectUpdate {
				deploymentName := "github-runner-" + tt.config.OrgName
				deployment, err := client.AppsV1().Deployments("test-namespace").Get(context.Background(), deploymentName, metav1.GetOptions{})
				if err != nil {
					t.Fatalf("Deployment not found: %v", err)
				}

				if *deployment.Spec.Replicas != tt.config.Replicas {
					t.Errorf("Expected %d replicas, got %d", tt.config.Replicas, *deployment.Spec.Replicas)
				}
			}
		})
	}
}

func TestDeleteDeployment(t *testing.T) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "github-runner-testorg",
			Namespace: "test-namespace",
		},
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "github-runner-token-testorg",
			Namespace: "test-namespace",
		},
	}

	client := fake.NewSimpleClientset(deployment, secret)
	manager := NewManager(client, "test-namespace", "/path/to/template")

	ctx := context.Background()
	err := manager.DeleteDeployment(ctx, "testorg")
	if err != nil {
		t.Fatalf("Failed to delete deployment: %v", err)
	}

	// Verify deployment was deleted
	_, err = client.AppsV1().Deployments("test-namespace").Get(ctx, "github-runner-testorg", metav1.GetOptions{})
	if err == nil {
		t.Error("Expected deployment to be deleted")
	}

	// Verify secret was deleted
	_, err = client.CoreV1().Secrets("test-namespace").Get(ctx, "github-runner-token-testorg", metav1.GetOptions{})
	if err == nil {
		t.Error("Expected secret to be deleted")
	}
}

func TestTemplateSubstitution(t *testing.T) {
	template := "org: {{ORG_NAME}}, token: {{ACCESS_TOKEN_B64}}"

	content := strings.ReplaceAll(template, "{{ORG_NAME}}", "testorg")
	tokenB64 := base64.StdEncoding.EncodeToString([]byte("test-token"))
	content = strings.ReplaceAll(content, "{{ACCESS_TOKEN_B64}}", tokenB64)

	expected := "org: testorg, token: " + tokenB64
	if content != expected {
		t.Errorf("Expected '%s', got '%s'", expected, content)
	}
}

// Helper function
func int32Ptr(i int32) *int32 {
	return &i
}
