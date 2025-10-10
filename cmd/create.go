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
		fmt.Println("Connecting using Postgres superuser...")

		adminDB, err := db.ConnectAndVerify("psql", "postgres", "your_postgres_password", "localhost", "5432", "postgres")
		if err != nil {
			fmt.Printf("Could not connect with admin credentials: %v\n", err)
			os.Exit(1)
		}
		defer adminDB.Close()

		// Connected as superuser

		formData, err := tui.RunCreateForm()
		if err != nil {
			fmt.Printf("Error running form: %v\n", err)
			os.Exit(1)
		}

		if formData.Quitting {
			fmt.Println("Database creation cancelled.")
			os.Exit(0)
		}

		dbName := formData.Inputs[0].Value()
		newUser := formData.Inputs[1].Value()
		newPassword := formData.Inputs[2].Value()

		err = db.CreateDBAndUser(adminDB, dbName, newUser, newPassword)
		if err != nil {
			fmt.Printf("\n Failed to create database: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\n Successfully created database '%s' and user '%s'!\n", dbName, newUser)
	},
}
