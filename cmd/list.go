package cmd

import (
    "fmt"
    "os"

    "github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/db"
    "github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/docker"
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

        // Also list Docker containerized PostgreSQL databases
        containers, derr := docker.GetAllContainerDatabases()
        if derr != nil {
            // Non-fatal: just skip docker list on error
            containers = nil
        }

        if len(dbNames) == 0 && len(containers) == 0 {
            fmt.Println("No databases found.")
            return
        }

        fmt.Println("Databases:")
        fmt.Println()
        for _, dbName := range dbNames {
            fmt.Printf("  %s\n", dbName)
        }
        for _, c := range containers {
            label := c.DatabaseName
            if c.Port != "" {
                label = fmt.Sprintf("%s [Docker] (container: %s, port: %s)", c.DatabaseName, c.ContainerName, c.Port)
            } else {
                label = fmt.Sprintf("%s [Docker] (container: %s)", c.DatabaseName, c.ContainerName)
            }
            fmt.Printf("  %s\n", label)
        }
	},
}

func init() {
	// Command registration is handled in root.go
}
