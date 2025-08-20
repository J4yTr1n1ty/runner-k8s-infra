package config

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name       string
		configYAML string
		envVars    map[string]string
		wantErr    bool
		validate   func(t *testing.T, cfg *Config)
	}{
		{
			name: "valid config with all fields",
			configYAML: `
github:
  token: "test-token"
  organizations: ["org1", "org2"]
  personalRepos: ["user/repo1"]
  
kubernetes:
  namespace: "test-namespace"
  deploymentPrefix: "test-runner-"
  
controller:
  pollInterval: "60s"
  minRunners: 1
  maxRunners: 5
  scaleDownDelay: "10m"
  scaleDownInterval: "2m"
  runnersPerJob: 1.5

deployment:
  templatePath: "test-template.yaml"
`,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.GitHub.Token != "test-token" {
					t.Errorf("Expected token 'test-token', got '%s'", cfg.GitHub.Token)
				}
				if len(cfg.GitHub.Organizations) != 2 {
					t.Errorf("Expected 2 organizations, got %d", len(cfg.GitHub.Organizations))
				}
				if cfg.Controller.MaxRunners != 5 {
					t.Errorf("Expected max runners 5, got %d", cfg.Controller.MaxRunners)
				}
			},
		},
		{
			name: "config with defaults",
			configYAML: `
github:
  organizations: ["org1"]
`,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.Kubernetes.Namespace != "github-runners" {
					t.Errorf("Expected default namespace 'github-runners', got '%s'", cfg.Kubernetes.Namespace)
				}
				if cfg.Controller.MaxRunners != 10 {
					t.Errorf("Expected default max runners 10, got %d", cfg.Controller.MaxRunners)
				}
				if cfg.Controller.RunnersPerJob != 1.2 {
					t.Errorf("Expected default runners per job 1.2, got %f", cfg.Controller.RunnersPerJob)
				}
			},
		},
		{
			name: "environment variable override",
			configYAML: `
github:
  organizations: ["org1"]
`,
			envVars: map[string]string{
				"GITHUB_TOKEN": "env-token",
				"KUBECONFIG":   "/path/to/kubeconfig",
			},
			validate: func(t *testing.T, cfg *Config) {
				if cfg.GitHub.Token != "env-token" {
					t.Errorf("Expected token from env 'env-token', got '%s'", cfg.GitHub.Token)
				}
				if cfg.Kubernetes.KubeConfig != "/path/to/kubeconfig" {
					t.Errorf("Expected kubeconfig from env, got '%s'", cfg.Kubernetes.KubeConfig)
				}
			},
		},
		{
			name:       "invalid yaml",
			configYAML: "invalid: yaml: content:",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary config file
			tmpDir, err := ioutil.TempDir("", "config-test")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(tmpDir)

			configPath := filepath.Join(tmpDir, "config.yaml")
			if err := ioutil.WriteFile(configPath, []byte(tt.configYAML), 0644); err != nil {
				t.Fatal(err)
			}

			// Set environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
				defer os.Unsetenv(key)
			}

			// Load config
			cfg, err := LoadConfig(configPath)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if tt.validate != nil {
				tt.validate(t, cfg)
			}
		})
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				GitHub: GitHubConfig{
					Token:         "valid-token",
					Organizations: []string{"org1"},
				},
				Deployment: DeploymentConfig{
					TemplatePath: "testdata/valid-template.yaml",
				},
			},
		},
		{
			name: "missing token",
			config: &Config{
				GitHub: GitHubConfig{
					Organizations: []string{"org1"},
				},
			},
			wantErr: true,
		},
		{
			name: "no organizations or repos",
			config: &Config{
				GitHub: GitHubConfig{
					Token: "valid-token",
				},
			},
			wantErr: true,
		},
		{
			name: "missing template file",
			config: &Config{
				GitHub: GitHubConfig{
					Token:         "valid-token",
					Organizations: []string{"org1"},
				},
				Deployment: DeploymentConfig{
					TemplatePath: "nonexistent.yaml",
				},
			},
			wantErr: true,
		},
	}

	// Create test template file
	tmpDir, err := ioutil.TempDir("", "config-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testDataDir := filepath.Join(tmpDir, "testdata")
	os.MkdirAll(testDataDir, 0755)
	templatePath := filepath.Join(testDataDir, "valid-template.yaml")
	ioutil.WriteFile(templatePath, []byte("test: template"), 0644)

	// Change to temp directory so relative paths work
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set defaults
			tt.config.setDefaults()

			err := tt.config.Validate()
			if tt.wantErr && err == nil {
				t.Error("Expected validation error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected validation error: %v", err)
			}
		})
	}
}

func TestConfigDurationParsing(t *testing.T) {
	cfg := &Config{
		Controller: ControllerConfig{
			PollInterval:      "30s",
			ScaleDownDelay:    "5m",
			ScaleDownInterval: "1m",
		},
	}

	pollInterval, err := cfg.GetPollInterval()
	if err != nil {
		t.Errorf("Failed to parse poll interval: %v", err)
	}
	if pollInterval != 30*time.Second {
		t.Errorf("Expected 30s, got %v", pollInterval)
	}

	scaleDownDelay, err := cfg.GetScaleDownDelay()
	if err != nil {
		t.Errorf("Failed to parse scale down delay: %v", err)
	}
	if scaleDownDelay != 5*time.Minute {
		t.Errorf("Expected 5m, got %v", scaleDownDelay)
	}

	scaleDownInterval, err := cfg.GetScaleDownInterval()
	if err != nil {
		t.Errorf("Failed to parse scale down interval: %v", err)
	}
	if scaleDownInterval != 1*time.Minute {
		t.Errorf("Expected 1m, got %v", scaleDownInterval)
	}
}

func TestAbsoluteTemplatePath(t *testing.T) {
	tests := []struct {
		name         string
		templatePath string
		expectAbs    bool
	}{
		{
			name:         "relative path",
			templatePath: "deploy/template.yaml",
			expectAbs:    false,
		},
		{
			name:         "absolute path",
			templatePath: "/absolute/path/template.yaml",
			expectAbs:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Deployment: DeploymentConfig{
					TemplatePath: tt.templatePath,
				},
			}

			absPath, err := cfg.GetAbsoluteTemplatePath()
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if tt.expectAbs {
				if absPath != tt.templatePath {
					t.Errorf("Expected absolute path unchanged, got %s", absPath)
				}
			} else {
				if !filepath.IsAbs(absPath) {
					t.Errorf("Expected absolute path, got %s", absPath)
				}
			}
		})
	}
}