package cmd

import (
	"fmt"
	"os"

	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/db"
	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/tui"
	"github.com/spf13/cobra"
)

var connectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect to a database and save credentials",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		conn, result, err := openManagedConnection()
		if err != nil {
			fmt.Printf("Connection failed: %v\n", err)
			os.Exit(1)
		}
		if result.Quitting {
			fmt.Println("Connection cancelled.")
			return
		}
		defer conn.Close()

		fmt.Println("\n Connected successfully!")

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
			default:
				return
			}
		}
	},
}

func saveConnectionProfile(result tui.ConnectResult) error {
	return saveConnectionProfileAs("", result)
}
