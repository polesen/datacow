package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "0.1.0-dev"

func main() {
	root := &cobra.Command{
		Use:          "datacow",
		Short:        "Like k9s or lazygit, but for databases.",
		Version:      version,
		SilenceUsage: true,
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

	root.RunE = func(cmd *cobra.Command, args []string) error {
		fmt.Println("tui: not yet implemented")
		return nil
	}

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
