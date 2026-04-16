package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/beetio/datacow/internal/core/db"
	"github.com/beetio/datacow/internal/tui"
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
	connStr, _ := cmd.Flags().GetString("connection-string")

	cfg := tui.Config{
		ConnectionString: connStr,
		Version:          version,
	}

	var client db.Client
	var connErr error

	if connStr == "" {
		connErr = fmt.Errorf("no --connection-string provided")
	} else {
		client, connErr = db.Connect(connStr)
		if client != nil {
			defer func() { _ = client.Close() }()
		}
	}

	app := tui.New(cfg, client, connErr)

	p := tea.NewProgram(app, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
