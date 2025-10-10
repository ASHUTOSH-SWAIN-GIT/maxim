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
			// Connect flow (same as cmd/connect.go)
			result, err := tui.RunConnectForm()
			if err != nil {
				fmt.Printf("Error: could not open connect form: %v\n", err)
				os.Exit(1)
			}
			if result.Quitting {
				fmt.Println("Cancelled: connection aborted by user.")
				return
			}
			conn, err := db.ConnectAndVerify("psql", result.User, result.Password, "localhost", result.Port, result.DBName)
			if err != nil {
				fmt.Printf("Error: connection failed: %v\n", err)
				os.Exit(1)
			}
			defer conn.Close()
			fmt.Printf("Success: connected to Postgres at localhost:%s as %s (db %s).\n", result.Port, result.User, result.DBName)
		case 1:
			// Create flow (same as cmd/create.go)
			fmt.Println("Connecting using Postgres superuser...")
			adminDB, err := db.ConnectAndVerify("psql", "postgres", "your_postgres_password", "localhost", "5432", "postgres")
			if err != nil {
				fmt.Printf("Error: could not connect as superuser: %v\n", err)
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
	dbCmd.AddCommand(listCmd)
}
