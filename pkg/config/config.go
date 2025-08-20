package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	GitHub     GitHubConfig     `yaml:"github"`
	Kubernetes KubernetesConfig `yaml:"kubernetes"`
	Controller ControllerConfig `yaml:"controller"`
	Deployment DeploymentConfig `yaml:"deployment"`
}

type GitHubConfig struct {
	Token         string   `yaml:"token"`
	Organizations []string `yaml:"organizations"`
	PersonalRepos []string `yaml:"personalRepos"`
}

type KubernetesConfig struct {
	KubeConfig       string `yaml:"kubeconfig"`
	Namespace        string `yaml:"namespace"`
	DeploymentPrefix string `yaml:"deploymentPrefix"`
}

type ControllerConfig struct {
	PollInterval      string  `yaml:"pollInterval"`
	MinRunners        int32   `yaml:"minRunners"`
	MaxRunners        int32   `yaml:"maxRunners"`
	ScaleDownDelay    string  `yaml:"scaleDownDelay"`
	ScaleDownInterval string  `yaml:"scaleDownInterval"`
	RunnersPerJob     float64 `yaml:"runnersPerJob"`
}

type DeploymentConfig struct {
	TemplatePath string `yaml:"templatePath"`
}

func LoadConfig(configPath string) (*Config, error) {
	// If no config path provided, look for default locations
	if configPath == "" {
		// Try current directory first
		if _, err := os.Stat("config.yaml"); err == nil {
			configPath = "config.yaml"
		} else if _, err := os.Stat("config/config.yaml"); err == nil {
			configPath = "config/config.yaml"
		} else {
			return nil, fmt.Errorf("no config file found, specify --config flag")
		}
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
	}

	// Validate and set defaults
	if err := config.setDefaults(); err != nil {
		return nil, fmt.Errorf("failed to set config defaults: %w", err)
	}

	// Override with environment variables
	config.overrideWithEnv()

	return &config, nil
}

func (c *Config) setDefaults() error {
	// Set defaults for missing values
	if c.Kubernetes.Namespace == "" {
		c.Kubernetes.Namespace = "github-runners"
	}
	if c.Kubernetes.DeploymentPrefix == "" {
		c.Kubernetes.DeploymentPrefix = "github-runner-"
	}
	if c.Controller.PollInterval == "" {
		c.Controller.PollInterval = "30s"
	}
	if c.Controller.MaxRunners == 0 {
		c.Controller.MaxRunners = 10
	}
	if c.Controller.ScaleDownDelay == "" {
		c.Controller.ScaleDownDelay = "5m"
	}
	if c.Controller.ScaleDownInterval == "" {
		c.Controller.ScaleDownInterval = "1m"
	}
	if c.Controller.RunnersPerJob == 0 {
		c.Controller.RunnersPerJob = 1.2
	}
	if c.Deployment.TemplatePath == "" {
		c.Deployment.TemplatePath = "deploy/runner-deployment-template.yaml"
	}

	return nil
}

func (c *Config) overrideWithEnv() {
	// Override GitHub token from environment if not set in config
	if c.GitHub.Token == "" {
		if token := os.Getenv("GITHUB_TOKEN"); token != "" {
			c.GitHub.Token = token
		}
	}

	// Override kubeconfig from environment if not set
	if c.Kubernetes.KubeConfig == "" {
		if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
			c.Kubernetes.KubeConfig = kubeconfig
		}
	}
}

func (c *Config) GetPollInterval() (time.Duration, error) {
	return time.ParseDuration(c.Controller.PollInterval)
}

func (c *Config) GetScaleDownDelay() (time.Duration, error) {
	return time.ParseDuration(c.Controller.ScaleDownDelay)
}

func (c *Config) GetScaleDownInterval() (time.Duration, error) {
	return time.ParseDuration(c.Controller.ScaleDownInterval)
}

func (c *Config) GetAbsoluteTemplatePath() (string, error) {
	if filepath.IsAbs(c.Deployment.TemplatePath) {
		return c.Deployment.TemplatePath, nil
	}

	// Make relative to current working directory
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	return filepath.Join(wd, c.Deployment.TemplatePath), nil
}

func (c *Config) Validate() error {
	if c.GitHub.Token == "" {
		return fmt.Errorf("GitHub token is required (set GITHUB_TOKEN environment variable or specify in config)")
	}

	if len(c.GitHub.Organizations) == 0 && len(c.GitHub.PersonalRepos) == 0 {
		return fmt.Errorf("at least one organization or personal repository must be specified")
	}

	// Check template file exists
	templatePath, err := c.GetAbsoluteTemplatePath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		return fmt.Errorf("deployment template file not found: %s", templatePath)
	}

	return nil
}
