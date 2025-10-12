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

		// Handle quitting
		if choice == -1 {
			fmt.Println("Goodbye!")
			return
		}

		switch choice {
		case 0:
			// Connect flow - connect to a specific database
			result, err := tui.RunConnectForm()
			if err != nil {
				fmt.Printf("Error running form: %v\n", err)
				os.Exit(1)
			}
			if result.Quitting {
				fmt.Println("Connection cancelled.")
				os.Exit(0)
			}

			conn, err := db.ConnectAndVerify("psql", result.User, result.Password, "localhost", result.Port, result.DBName)
			if err != nil {
				fmt.Printf(" Connection failed: %v\n", err)
				os.Exit(1)
			}
			defer conn.Close()

			// Show database operations menu
			for {
				choice, err := tui.RunDBOperationsMenu(result.DBName)
				if err != nil {
					fmt.Printf("Error running operations menu: %v\n", err)
					break
				}

				// Check if user pressed 'q' to quit
				if choice == -1 {
					break
				}

				switch choice {
				case 0: // List all tables
					tables, err := db.GetTables(conn)
					if err != nil {
						fmt.Printf("Error fetching tables: %v\n", err)
						continue
					}
					selectedTable, err := tui.RunTableList(tables)
					if err != nil {
						continue
					}
					fmt.Printf("Selected table: %s\n", selectedTable)

				case 1: // Show table data
					tables, err := db.GetTables(conn)
					if err != nil {
						fmt.Printf("Error fetching tables: %v\n", err)
						continue
					}
					selectedTable, err := tui.RunTableList(tables)
					if err != nil {
						continue
					}

					columns, rows, err := db.GetTableData(conn, selectedTable)
					if err != nil {
						fmt.Printf("Error fetching table data: %v\n", err)
						continue
					}

					if err := tui.RunDataViewer(selectedTable, columns, rows); err != nil {
						fmt.Printf("Error displaying data: %v\n", err)
					}

				case 2: // Smart Editor
					if err := tui.RunSQLEditor(conn, result.DBName); err != nil {
						fmt.Printf("Error running SQL editor: %v\n", err)
					}

				case 3: // Back to main menu
					return
				}
			}
		case 1:
			// Create flow
			adminInfo, err := getAdminConnectionInfo()
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			defer adminInfo.DB.Close()

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
			if err := db.CreateDBAndUser(adminInfo.DB, "psql", dbName, newUser, newPassword, adminInfo.User, adminInfo.Password, adminInfo.Host, adminInfo.Port); err != nil {
				fmt.Printf("Error: failed to create database/user: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Success: created database '%s' and user '%s'.\n", dbName, newUser)
		case 2:
			// List databases flow
			adminInfo, err := getAdminConnectionInfo()
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			defer adminInfo.DB.Close()

			dbNames, err := db.ListDatabases(adminInfo.DB)
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
