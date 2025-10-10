package cmd

import (
	"fmt"
	"os"

	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/db"
	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/tui"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all databases on the connected server",
	Run: func(cmd *cobra.Command, args []string) {
		adminDB, err := getAdminConnection()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		defer adminDB.Close()

		dbNames, err := db.ListDatabases(adminDB)
		if err != nil {
			fmt.Printf("Could not fetch database list: %v\n", err)
			os.Exit(1)
		}

		if err := tui.RunDBList(dbNames); err != nil {
			fmt.Printf("Error displaying database list: %v\n", err)
			os.Exit(1)
		}
	},
}
