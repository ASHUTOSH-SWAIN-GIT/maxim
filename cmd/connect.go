package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var connectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect to a database and save credentials",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runManagedWorkspace(); err != nil {
			fmt.Printf("Workspace error: %v\n", err)
			os.Exit(1)
		}
	},
}
