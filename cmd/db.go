package cmd

import (
	"github.com/spf13/cobra"
)

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Manage db connections",
}

func init() {
	// fmt.Println("DEBUG: db.go init() is running")
	rootCmd.AddCommand(dbCmd)
}
