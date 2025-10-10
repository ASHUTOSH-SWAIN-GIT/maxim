package cmd

import (
	"fmt"
	"os"

	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/db"
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
			// Connect flow - use same credential management as other operations
			adminDB, err := getAdminConnection()
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			defer adminDB.Close()

			fmt.Printf("Success: connected to Postgres superuser.\n")
		case 1:
			// Create flow
			adminDB, err := getAdminConnection()
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			defer adminDB.Close()

			formData, err := tui.RunCreateForm()
			if err != nil {
				fmt.Printf("Error: could not open create form: %v\n", err)
				os.Exit(1)
			}
			if formData.Quitting {
				fmt.Println("Cancelled: database creation aborted by user.")
				return
			}
			dbName := formData.Inputs[0].Value()
			newUser := formData.Inputs[1].Value()
			newPassword := formData.Inputs[2].Value()
			if err := db.CreateDBAndUser(adminDB, "psql", dbName, newUser, newPassword); err != nil {
				fmt.Printf("Error: failed to create database/user: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Success: created database '%s' and user '%s'.\n", dbName, newUser)
		case 2:
			// List databases flow
			adminDB, err := getAdminConnection()
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			defer adminDB.Close()

			dbNames, err := db.ListDatabases(adminDB)
			if err != nil {
				fmt.Printf("Could not fetch database list: %v\n", err)
				os.Exit(1)
			}

			if err := tui.RunDBList(dbNames); err != nil {
				fmt.Printf("Error displaying database list: %v\n", err)
				os.Exit(1)
			}
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
	dbCmd.AddCommand(listCmd)
}
