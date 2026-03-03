package config

import (
	"os"
	"fmt"
)

type Config struct {
	Kubeconfig string `yaml:"kubeconfig"`
	Namespace  string `yaml:"namespace"`
	Severity   string `yaml:"severity"`
	Output    string `yaml:"output"`
}

func Default() *Config {
	return &Config{
		Kubeconfig: os.Getenv("KUBECONFIG"),
		Namespace: "all",
		Severity:  "all",
		Output:    "text",
	}
}

func Load(path string) (*Config, error) {
	if path == "" {
		return Default(), nil
	}
	// In full implementation, would parse YAML file
	return Default(), nil
}

func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("config is nil")
	}
	return nil
}
