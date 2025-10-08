package cmd

import (
	"fmt"
	"os"

	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/db"
	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/tui"
	"github.com/spf13/cobra"
)

var connectCmd = &cobra.Command{
	Use:   "connect [psql|mysql]",
	Short: "Connect to a database and save credentials",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		dbType := args[0]
		if dbType != "psql" && dbType != "mysql" {
			fmt.Println("Error: db type must be 'psql' or 'mysql'")
			os.Exit(1)
		}

		finalModel, err := tui.RunConnectForm()
		if err != nil {
			fmt.Printf("Error running form: %v\n", err)
			os.Exit(1)
		}

		if finalModel.Quitting {
			fmt.Println("Connection cancelled.")
			os.Exit(0)
		}

		username := finalModel.Inputs[0].Value()
		password := finalModel.Inputs[1].Value()
		port := finalModel.Inputs[2].Value()
		dbname := finalModel.Inputs[3].Value()
		host := "localhost"

		conn, err := db.ConnectAndVerify(dbType, username, password, host, port, dbname)
		if err != nil {
			fmt.Printf("\n Connection failed: %v\n", err)
			os.Exit(1)
		}
		defer conn.Close()

		fmt.Println("\n Connected successfully!")
	},
}
