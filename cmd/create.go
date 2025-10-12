package cmd

import (
	"fmt"
	"os"

	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/db"
	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/tui"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new database and a dedicated user",
	Run: func(cmd *cobra.Command, args []string) {
		adminInfo, err := getAdminConnectionInfo()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		defer adminInfo.DB.Close()

		formData, err := tui.RunCreateForm()
		if err != nil {
			fmt.Printf("Error: could not open create form: %v\n", err)
			os.Exit(1)
		}

		if formData.Quitting {
			fmt.Println("Cancelled: database creation aborted by user.")
			os.Exit(0)
		}

		dbName := formData.Inputs[0].Value()
		newUser := formData.Inputs[1].Value()
		newPassword := formData.Inputs[2].Value()

		err = db.CreateDBAndUser(adminInfo.DB, "psql", dbName, newUser, newPassword, adminInfo.User, adminInfo.Password, adminInfo.Host, adminInfo.Port)
		if err != nil {
			fmt.Printf("Error: failed to create database/user: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Success: created database '%s' and user '%s'.\n", dbName, newUser)
	},
}
