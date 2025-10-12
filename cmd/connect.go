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

		fmt.Printf("Database connection '%s' saved successfully.\n", connectionName)
	},
}
