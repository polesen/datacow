package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/polesen/datacow/internal/core/config"
	"github.com/polesen/datacow/internal/core/db"
	"github.com/polesen/datacow/internal/tui"
)

var version = "0.1.0-dev"

func main() {
	root := &cobra.Command{
		Use:          "datacow",
		Short:        "Like k9s or lazygit, but for databases.",
		Version:      version,
		SilenceUsage: true,
		RunE:         runTUI,
	}

	root.PersistentFlags().String("connection-string", "", "Database connection string (e.g. postgres://user:pass@host/db)")
	root.PersistentFlags().String("config", "", "Config file path (default: ./datacow.yaml or ~/.datacow/config.yaml)")

	serve := &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP API server and web app",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("serve: not yet implemented")
			return nil
		},
	}
	serve.Flags().Int("port", 8080, "Port to listen on")

	root.AddCommand(serve)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func runTUI(cmd *cobra.Command, args []string) error {
	connStr, _ := cmd.Flags().GetString("connection-string")   // error only if flag unregistered or wrong type, impossible here
	configPath, _ := cmd.Flags().GetString("config")           // same

	cfg, err := loadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	var (
		client           db.Client
		connErr          error
		activeDatasource string
	)

	switch {
	case connStr != "":
		// Explicit connection string — connect immediately, ignore config datasources.
		client, connErr = db.Connect(connStr)
		if client != nil {
			defer func() { _ = client.Close() }()
		}

	case len(cfg.Datasources) == 1:
		// Exactly one configured datasource — connect automatically.
		connStr = cfg.Datasources[0].ConnectionString
		activeDatasource = cfg.Datasources[0].Name
		client, connErr = db.Connect(connStr)
		if client != nil {
			defer func() { _ = client.Close() }()
		}

	case len(cfg.Datasources) == 0:
		connErr = fmt.Errorf("no --connection-string provided")

		// len > 1: multi-datasource mode — TUI handles lazy connection.
	}

	tuiCfg := tui.Config{
		ConnectionString: connStr,
		Version:          version,
		ConfigDatasets:   cfg.Datasets,
		ActiveDatasource: activeDatasource,
		Datasources:      cfg.Datasources,
	}

	app := tui.New(tuiCfg, client, connErr)
	defer app.Close()

	p := tea.NewProgram(app, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

// loadConfig loads config from an explicit path or searches the default locations.
func loadConfig(explicitPath string) (*config.Config, error) {
	if explicitPath != "" {
		return config.Load(explicitPath)
	}
	for _, p := range config.DefaultPaths() {
		cfg, err := config.Load(p)
		if err != nil {
			return nil, err
		}
		if len(cfg.Datasources) > 0 || len(cfg.Datasets) > 0 {
			return cfg, nil
		}
	}
	return &config.Config{}, nil
}
