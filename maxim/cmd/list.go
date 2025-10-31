package cmd

import (
	"fmt"
	"os"

	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/db"
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

		if len(dbNames) == 0 {
			fmt.Println("No databases found on this server.")
		} else {
			fmt.Println("Databases on Server:")
			fmt.Println()
			for _, dbName := range dbNames {
				fmt.Printf("  %s\n", dbName)
			}
		}
	},
}

func init() {
	// Command registration is handled in root.go
}
