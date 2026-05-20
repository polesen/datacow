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
	Name         string              `yaml:"name"`
	Datasource   string              `yaml:"datasource,omitempty"`
	Table        string              `yaml:"table,omitempty"`
	SQL          string              `yaml:"sql,omitempty"`
	Perspectives []PerspectiveConfig `yaml:"perspectives,omitempty"`
}

type PerspectiveConfig struct {
	Name    string         `yaml:"name"`
	Columns []string       `yaml:"columns,omitempty"`
	Filters []FilterConfig `yaml:"filters,omitempty"`
	Sort    []SortConfig   `yaml:"sort,omitempty"`
}

type FilterConfig struct {
	Column   string `yaml:"column"`
	Operator string `yaml:"operator"`
	Value    any    `yaml:"value"`
}

type SortConfig struct {
	Column string `yaml:"column"`
	Desc   bool   `yaml:"desc,omitempty"`
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
		if ds.SQL != "" && len(ds.Perspectives) > 0 {
			return nil, fmt.Errorf("dataset %q: perspectives are only supported on table datasets", ds.Name)
		}
		for _, p := range ds.Perspectives {
			if p.Name == "" {
				return nil, fmt.Errorf("dataset %q: perspective missing required field: name", ds.Name)
			}
		}
	}

	return &cfg, nil
}

// Save writes cfg to path atomically (write to .tmp, then rename).
// Creates parent directories as needed.
func Save(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename config: %w", err)
	}
	return nil
}

// AppendPerspective upserts a perspective under the given datasource+table in the YAML file at path.
// If no matching dataset entry exists, a minimal one is created.
// If a perspective with the same name already exists, it is replaced.
func AppendPerspective(path, datasource, tableName string, p PerspectiveConfig) error {
	cfg, err := Load(path)
	if err != nil {
		return err
	}

	// Find the matching DatasetConfig entry.
	idx := -1
	for i, ds := range cfg.Datasets {
		if ds.Table != tableName {
			continue
		}
		// Blank datasource matches any; otherwise must match exactly.
		if ds.Datasource == "" || ds.Datasource == datasource {
			idx = i
			break
		}
	}

	if idx < 0 {
		// No matching entry — create a minimal one.
		cfg.Datasets = append(cfg.Datasets, DatasetConfig{
			Name:       tableName,
			Datasource: datasource,
			Table:      tableName,
		})
		idx = len(cfg.Datasets) - 1
	}

	// Upsert the perspective by name.
	replaced := false
	for i, existing := range cfg.Datasets[idx].Perspectives {
		if existing.Name == p.Name {
			cfg.Datasets[idx].Perspectives[i] = p
			replaced = true
			break
		}
	}
	if !replaced {
		cfg.Datasets[idx].Perspectives = append(cfg.Datasets[idx].Perspectives, p)
	}

	return Save(path, cfg)
}

// DefaultPaths returns the search order for config files.
func DefaultPaths() []string {
	home, _ := os.UserHomeDir()
	return []string{
		"./datacow.yaml",
		filepath.Join(home, ".datacow", "config.yaml"),
	}
}
