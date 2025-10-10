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
	Short: "Connect to a Postgres database and save credentials",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		result, err := tui.RunConnectForm()
		if err != nil {
			fmt.Printf("Error: could not open connect form: %v\n", err)
			os.Exit(1)
		}

		if result.Quitting {
			fmt.Println("Cancelled: connection aborted by user.")
			os.Exit(0)
		}

		conn, err := db.ConnectAndVerify("psql", result.User, result.Password, "localhost", result.Port, result.DBName)
		if err != nil {
			fmt.Printf("Error: connection failed: %v\n", err)
			os.Exit(1)
		}
		defer conn.Close()

		fmt.Printf("Success: connected to Postgres at localhost:%s as %s (db %s).\n", result.Port, result.User, result.DBName)
	},
}
