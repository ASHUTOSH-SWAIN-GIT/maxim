package cmd

import (
	"fmt"
	"os"

	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/tui"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "maxim",
	Short: "Maxim is a terminal-based client for PostgreSQL and MySQL.",
	Long: `A fast and modern TUI for interacting with your databases
directly from the terminal.`,
	Run: func(cmd *cobra.Command, args []string) {
		choice, err := tui.RunMainMenu()
		if err != nil {
			fmt.Printf("Error running main menu: %v\n", err)
			os.Exit(1)
		}

		switch choice {
		case 0:
			// Open the Connect form
			if model, err := tui.RunConnectForm(); err != nil {
				fmt.Printf("Error running connect form: %v\n", err)
			} else if model.Quitting {
				// user cancelled; do nothing
			}
		case 1:
			// Open the Create DB form
			if model, err := tui.RunCreateForm(); err != nil {
				fmt.Printf("Error running create form: %v\n", err)
			} else if model.Quitting {
				// user cancelled; do nothing
			}
		case 2:
			fmt.Println("TODO: Execute 'list all dbs' logic here.")
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "An error occurred: %s\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(dbCmd)
	dbCmd.AddCommand(connectCmd)
	dbCmd.AddCommand(createCmd)
}
