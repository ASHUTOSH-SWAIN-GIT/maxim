package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

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
				case 0: // Show table data
					tables, err := db.GetTables(conn)
					if err != nil {
						fmt.Printf("Error fetching tables: %v\n", err)
						continue
					}
					selectedTable, err := tui.RunTableList(tables)
					if err != nil {
						continue
					}

					if err := tui.RunPagedDataViewer(conn, selectedTable); err != nil {
						fmt.Printf("Error displaying data: %v\n", err)
					}

				case 1: // Editor
					if err := tui.RunSQLEditor(conn, result.DBName); err != nil {
						fmt.Printf("Error running SQL editor: %v\n", err)
					}
				case 2: // NL2SQL
					form, err := tui.RunNL2SQLForm()
					if err != nil || form.Quitting || form.NL == "" {
						continue
					}
					dbURI := fmt.Sprintf("postgresql+psycopg2://%s:%s@localhost:%s/%s", result.User, result.Password, result.Port, result.DBName)
					payload := map[string]string{
						"nl_query": form.NL,
						"db_uri":   dbURI,
					}
					body, _ := json.Marshal(payload)
					resp, err := http.Post("http://127.0.0.1:5000/generate-sql", "application/json", bytes.NewReader(body))
					if err != nil {
						fmt.Printf("Error calling NL2SQL API: %v\n", err)
						continue
					}
					respBody, _ := io.ReadAll(resp.Body)
					resp.Body.Close()
					if resp.StatusCode != 200 {
						fmt.Printf("NL2SQL API error: %s\n", string(respBody))
						continue
					}
					sql := strings.TrimSpace(string(respBody))
					res := db.ExecuteQuery(conn, sql)
					if res.Success {
						out := res.Data
						out = strings.Replace(out, "Query executed successfully!\n\n", "", 1)
						fmt.Println(out)
					} else {
						fmt.Println(res.Error)
					}
				default:
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
	// rootCmd.AddCommand(dbCmd)
}
