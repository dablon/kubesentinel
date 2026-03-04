package config

import (
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Kubeconfig string `yaml:"kubeconfig"`
	Namespace  string `yaml:"namespace"`
	Severity   string `yaml:"severity"`
	Output     string `yaml:"output"`
}

func Default() *Config {
	return &Config{
		Kubeconfig: resolveKubeconfig(""),
		Namespace:  "all",
		Severity:   "all",
		Output:     "text",
	}
}

func Load(path string) (*Config, error) {
	cfg := Default()
	if path != "" {
		cfg.Kubeconfig = resolveKubeconfig(path)
	}
	return cfg, nil
}

// resolveKubeconfig determines the kubeconfig path.
// Priority: explicit path > KUBECONFIG env > ~/.kube/config
func resolveKubeconfig(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if env := os.Getenv("KUBECONFIG"); env != "" {
		return env
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	defaultPath := filepath.Join(home, ".kube", "config")
	if _, err := os.Stat(defaultPath); err == nil {
		return defaultPath
	}
	return ""
}

func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("config is nil")
	}
	return nil
}
