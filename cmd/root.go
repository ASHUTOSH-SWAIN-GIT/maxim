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
		// Show beautiful ASCII art and description
		tui.ShowASCIIArt()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "An error occurred: %s\n", err)
		os.Exit(1)
	}
}

// startCmd represents the start command for TUI interface
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the interactive TUI interface",
	Long: `Start the interactive terminal user interface for database operations.
This will launch the main menu where you can connect to databases,
create new databases, and perform various database operations.`,
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

				case 2: // Editor
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

			selectedDB, err := tui.RunDBList(dbNames)
			if err != nil {
				fmt.Printf("Error selecting database: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Selected database: %s\n", selectedDB)
		case 3:
			// Delete database flow
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

			selectedDB, err := tui.RunDBList(dbNames)
			if err != nil {
				fmt.Printf("Error selecting database: %v\n", err)
				os.Exit(1)
			}

			// Confirm deletion
			fmt.Printf("Are you sure you want to delete database '%s'? This action cannot be undone! (y/N): ", selectedDB)
			var confirm string
			fmt.Scanln(&confirm)
			if confirm != "y" && confirm != "Y" {
				fmt.Println("Database deletion cancelled.")
				return
			}

			if err := db.DeleteDatabase(adminInfo.DB, "psql", selectedDB); err != nil {
				fmt.Printf("Error deleting database: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Success: database '%s' has been deleted.\n", selectedDB)
		}
	},
}

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a database",
	Long: `Delete a PostgreSQL database.
This will permanently remove the selected database and all its data.`,
	Run: func(cmd *cobra.Command, args []string) {
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

		selectedDB, err := tui.RunDBList(dbNames)
		if err != nil {
			fmt.Printf("Error selecting database: %v\n", err)
			os.Exit(1)
		}

		// Confirm deletion
		fmt.Printf("Are you sure you want to delete database '%s'? This action cannot be undone! (y/N): ", selectedDB)
		var confirm string
		fmt.Scanln(&confirm)
		if confirm != "y" && confirm != "Y" {
			fmt.Println("Database deletion cancelled.")
			return
		}

		if err := db.DeleteDatabase(adminInfo.DB, "psql", selectedDB); err != nil {
			fmt.Printf("Error deleting database: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Success: database '%s' has been deleted.\n", selectedDB)
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(connectCmd)
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(dbCmd)
}
