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
		// Show connect form directly for local database
		result, err := tui.RunConnectForm()
		if err != nil {
			fmt.Printf("Error running form: %v\n", err)
			os.Exit(1)
		}

		if result.Quitting {
			fmt.Println("Connection cancelled.")
			os.Exit(0)
		}

		conn, err := db.ConnectPostgres(db.ConnectionOptions{
			User: result.User, Password: result.Password, Host: result.Host,
			Port: result.Port, Database: result.DBName, SSLMode: result.SSLMode,
		})
		if err != nil {
			fmt.Printf("\n Connection failed: %v\n", err)
			os.Exit(1)
		}
		defer conn.Close()

		fmt.Println("\n Connected successfully!")

		if err := saveConnectionProfile(result); err != nil {
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
	details := config.ConnectionDetails{
		Host: result.Host, Port: result.Port, User: result.User,
		DBName: result.DBName, SSLMode: result.SSLMode,
	}
	name := fmt.Sprintf("%s@%s:%s/%s", result.User, result.Host, result.Port, result.DBName)
	return config.SaveDatabaseConnection(name, details, result.Password)
}
