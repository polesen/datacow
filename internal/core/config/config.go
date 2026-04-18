package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Datasources []DatasourceConfig `yaml:"datasources"`
	Datasets    []DatasetConfig    `yaml:"datasets"`
}

type DatasourceConfig struct {
	Name             string `yaml:"name"`
	ConnectionString string `yaml:"connection_string"`
}

type DatasetConfig struct {
	Name       string `yaml:"name"`
	Datasource string `yaml:"datasource"`
	Table      string `yaml:"table"`
	SQL        string `yaml:"sql"`
}

// Load reads and parses a config file. Returns empty Config if file does not exist.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Config{}, nil
		}
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	for _, ds := range cfg.Datasets {
		if ds.Name == "" {
			return nil, fmt.Errorf("dataset missing required field: name")
		}
		if ds.Table == "" && ds.SQL == "" {
			return nil, fmt.Errorf("dataset %q: must specify either table or sql", ds.Name)
		}
		if ds.Table != "" && ds.SQL != "" {
			return nil, fmt.Errorf("dataset %q: table and sql are mutually exclusive", ds.Name)
		}
	}

	return &cfg, nil
}

// DefaultPaths returns the search order for config files.
func DefaultPaths() []string {
	home, _ := os.UserHomeDir()
	return []string{
		"./datacow.yaml",
		filepath.Join(home, ".datacow", "config.yaml"),
	}
}
