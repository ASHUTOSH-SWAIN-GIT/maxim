package cmd

import (
	"fmt"
	"os"

	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/config"
	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/db"
	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/tui"
	"github.com/spf13/cobra"
)

var connectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect to a database and save credentials",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
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
			fmt.Printf("\n Connection failed: %v\n", err)
			os.Exit(1)
		}
		defer conn.Close()

		fmt.Println("\n Connected successfully!")

		detailsToSave := config.ConnectionDetails{
			Host:   "localhost",
			Port:   result.Port,
			User:   result.User,
			DBName: result.DBName,
		}

		// Create a connection name based on the database name
		connectionName := fmt.Sprintf("%s@%s:%s", result.User, "localhost", result.Port)

		if err := config.SaveDatabaseConnection(connectionName, detailsToSave, result.Password); err != nil {
			fmt.Printf("\n Failed to save credentials: %v\n", err)
			os.Exit(1)
		}

		// Show database operations menu (same as TUI version)
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
	},
}

func init() {
	// Command registration is handled in root.go
}
