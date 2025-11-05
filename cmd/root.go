package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/db"
	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/docker"
	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/tui"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "maxim",
	Short: "Maxim is a terminal-based client for PostgreSQL and MySQL.",
	Long: `A fast and modern TUI for interacting with your databases
directly from the terminal.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Show beautiful ASCII art and description
		tui.ShowASCIIArt()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "An error occurred: %s\n", err)
		os.Exit(1)
	}
}

// startCmd represents the start command for TUI interface
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the interactive TUI interface",
	Long: `Start the interactive terminal user interface for database operations.
This will launch the main menu where you can connect to databases,
create new databases, and perform various database operations.`,
	Run: func(cmd *cobra.Command, args []string) {
		choice, err := tui.RunMainMenu()
		if err != nil {
			fmt.Printf("Error running main menu: %v\n", err)
			os.Exit(1)
		}

		// Handle quitting
		if choice == -1 {
			fmt.Println("Goodbye!")
			return
		}

		switch choice {
		case 0:
			// Connect to local DB - show connect form directly
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
				fmt.Printf(" Connection failed: %v\n", err)
				os.Exit(1)
			}
			defer conn.Close()

			// Show database operations menu
			for {
				opsChoice, err := tui.RunDBOperationsMenu(result.DBName)
				if err != nil {
					fmt.Printf("Error running operations menu: %v\n", err)
					break
				}

				// Check if user pressed 'q' to quit
				if opsChoice == -1 {
					break
				}

				switch opsChoice {
				case 0: // Show table data
					tables, err := db.GetTables(conn)
					if err != nil {
						fmt.Printf("Error fetching tables: %v\n", err)
						continue
					}
					selectedTable, err := tui.RunTableList(tables)
					if err != nil {
						continue
					}

					if err := tui.RunPagedDataViewer(conn, selectedTable); err != nil {
						fmt.Printf("Error displaying data: %v\n", err)
					}

				case 1: // Editor
					if err := tui.RunSQLEditor(conn, result.DBName); err != nil {
						fmt.Printf("Error running SQL editor: %v\n", err)
					}

				default:
					return
				}
			}
		case 1:
			// Create flow - show submenu for Local vs Docker
			createType, err := tui.RunCreateTypeMenu()
			if err != nil {
				fmt.Printf("Error running create type menu: %v\n", err)
				os.Exit(1)
			}

			if createType == -1 {
				fmt.Println("Database creation cancelled.")
				return
			}

			if createType == 0 {
				// Local database creation
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
					return
				}
				dbName := formData.Inputs[0].Value()
				newUser := formData.Inputs[1].Value()
				newPassword := formData.Inputs[2].Value()
				if err := db.CreateDBAndUser(adminInfo.DB, "psql", dbName, newUser, newPassword, adminInfo.User, adminInfo.Password, adminInfo.Host, adminInfo.Port); err != nil {
					fmt.Printf("Error: failed to create database/user: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("Success: created database '%s' and user '%s'.\n", dbName, newUser)
			} else if createType == 1 {
				// Docker container creation
				handleContainerSpinUp()
			}
		case 2:
			// List databases flow
			adminInfo, err := getAdminConnectionInfo()
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			defer adminInfo.DB.Close()

			dbNames, err := db.ListDatabases(adminInfo.DB)
			if err != nil {
				fmt.Printf("Could not fetch database list: %v\n", err)
				os.Exit(1)
			}

			if err := tui.RunDBListDisplay(dbNames); err != nil {
				fmt.Printf("Error displaying databases: %v\n", err)
				os.Exit(1)
			}
		case 3:
			// Delete database flow
			handleDeleteDatabase()
		}
	},
}

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a database",
	Long: `Delete a PostgreSQL database.
This will permanently remove the selected database and all its data.`,
	Run: func(cmd *cobra.Command, args []string) {
		handleDeleteDatabase()
	},
}

func handleDeleteDatabase() {
	adminInfo, err := getAdminConnectionInfo()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer adminInfo.DB.Close()

	// Get databases from PostgreSQL server
	dbNames, err := db.ListDatabases(adminInfo.DB)
	if err != nil {
		fmt.Printf("Could not fetch database list: %v\n", err)
		os.Exit(1)
	}

	// Get databases from Docker containers
	containerDBs, err := docker.GetAllContainerDatabases()
	if err == nil && len(containerDBs) > 0 {
		// Add Docker container databases to the list with a marker
		for _, containerDB := range containerDBs {
			// Check if already in list (avoid duplicates)
			found := false
			for _, dbName := range dbNames {
				if dbName == containerDB.DatabaseName {
					found = true
					break
				}
			}
			if !found {
				// Mark as Docker container
				dbNames = append(dbNames, fmt.Sprintf("%s [Docker]", containerDB.DatabaseName))
			}
		}
	}

	selectedDB, err := tui.RunDBList(dbNames)
	if err != nil {
		fmt.Printf("Error selecting database: %v\n", err)
		os.Exit(1)
	}

	// Check if it's a Docker container database (has [Docker] marker)
	if strings.HasSuffix(selectedDB, " [Docker]") {
		selectedDB = strings.TrimSuffix(selectedDB, " [Docker]")
		// Find the container
		containerInfo, err := docker.FindContainerByDatabaseName(selectedDB)
		if err == nil && containerInfo != nil {
			fmt.Printf("Database '%s' is running in Docker container '%s'.\n", selectedDB, containerInfo.ContainerName)
			fmt.Printf("Are you sure you want to delete this container? This action cannot be undone! (y/N): ")
			var confirm string
			fmt.Scanln(&confirm)
			if confirm != "y" && confirm != "Y" {
				fmt.Println("Container deletion cancelled.")
				return
			}

			// Stop and remove the container
			fmt.Printf("Stopping container '%s'...\n", containerInfo.ContainerName)
			if err := docker.StopContainer(containerInfo.ContainerName); err != nil {
				fmt.Printf("Warning: failed to stop container: %v\n", err)
			}

			fmt.Printf("Removing container '%s'...\n", containerInfo.ContainerName)
			if err := docker.RemoveContainer(containerInfo.ContainerName); err != nil {
				fmt.Printf("Error removing container: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Success: container '%s' (database '%s') has been deleted.\n", containerInfo.ContainerName, selectedDB)
			return
		}
	}

	// Regular database deletion (not a Docker container)
	// Confirm deletion
	fmt.Printf("Are you sure you want to delete database '%s'? This action cannot be undone! (y/N): ", selectedDB)
	var confirm string
	fmt.Scanln(&confirm)
	if confirm != "y" && confirm != "Y" {
		fmt.Println("Database deletion cancelled.")
		return
	}

	if err := db.DeleteDatabase(adminInfo.DB, "psql", selectedDB); err != nil {
		fmt.Printf("Error deleting database: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Success: database '%s' has been deleted.\n", selectedDB)
}

func handleDockerContainerConnect() (tui.ConnectResult, error) {
	// Check if Docker is available
	if err := docker.IsDockerAvailable(); err != nil {
		return tui.ConnectResult{}, fmt.Errorf("Docker is not available. Please install Docker and ensure it's running")
	}

	// Get all Docker container databases
	containerDBs, err := docker.GetAllContainerDatabases()
	if err != nil {
		return tui.ConnectResult{}, fmt.Errorf("failed to get Docker containers: %w", err)
	}

	if len(containerDBs) == 0 {
		return tui.ConnectResult{}, fmt.Errorf("no Docker containers with PostgreSQL databases found")
	}

	// Create a list of container names for selection
	containerNames := make([]string, len(containerDBs))
	for i, containerDB := range containerDBs {
		containerNames[i] = fmt.Sprintf("%s [%s] (Port: %s)", containerDB.ContainerName, containerDB.DatabaseName, containerDB.Port)
	}

	selectedContainer, err := tui.RunDBList(containerNames)
	if err != nil {
		return tui.ConnectResult{Quitting: true}, nil
	}

	// Extract container name from selection
	containerName := strings.Split(selectedContainer, " ")[0]

	// Find the container info
	var selectedContainerInfo *docker.ContainerDBInfo
	for _, containerDB := range containerDBs {
		if containerDB.ContainerName == containerName {
			selectedContainerInfo = &containerDB
			break
		}
	}

	if selectedContainerInfo == nil {
		return tui.ConnectResult{}, fmt.Errorf("container not found")
	}

	// Get password from user
	fmt.Println("\nEnter password for the container database:")
	passwordForm, err := tui.RunPasswordForm()
	if err != nil {
		return tui.ConnectResult{Quitting: true}, nil
	}
	if passwordForm.Quitting {
		return tui.ConnectResult{Quitting: true}, nil
	}

	return tui.ConnectResult{
		DBType:   "psql",
		Port:     selectedContainerInfo.Port,
		User:     "postgres",
		Password: passwordForm.Password,
		DBName:   selectedContainerInfo.DatabaseName,
		Quitting: false,
	}, nil
}

func maskPassword(password string) string {
	if len(password) == 0 {
		return ""
	}
	// Show first character and mask the rest
	if len(password) == 1 {
		return "*"
	}
	return string(password[0]) + strings.Repeat("*", len(password)-1)
}

func handleContainerSpinUp() {
	// Check if Docker is available
	fmt.Println("\nChecking Docker availability...")
	if err := docker.IsDockerAvailable(); err != nil {
		fmt.Printf("Error: %v\n", err)
		fmt.Println("\nPlease ensure Docker is installed and running:")
		fmt.Println("- Install Docker Desktop: https://www.docker.com/products/docker-desktop")
		fmt.Println("- Or install Docker Engine: https://docs.docker.com/engine/install/")
		os.Exit(1)
	}
	fmt.Println("✓ Docker is available")

	// Show form to collect container details
	formResult, err := tui.RunContainerForm()
	if err != nil {
		fmt.Printf("Error running form: %v\n", err)
		os.Exit(1)
	}

	if formResult.Quitting {
		fmt.Println("Container spin-up cancelled.")
		return
	}

	// Validate inputs
	if formResult.ContainerName == "" {
		fmt.Println("Error: Container name is required")
		os.Exit(1)
	}
	if formResult.DatabaseName == "" {
		fmt.Println("Error: Database name is required")
		os.Exit(1)
	}
	if formResult.Port == "" {
		fmt.Println("Error: Port is required")
		os.Exit(1)
	}
	if formResult.Password == "" {
		fmt.Println("Error: Password is required")
		os.Exit(1)
	}

	// Prepare container info
	containerInfo := docker.ContainerInfo{
		ContainerName: formResult.ContainerName,
		DatabaseName:  formResult.DatabaseName,
		Port:          formResult.Port,
		Password:      formResult.Password,
	}

	// Start container
	fmt.Printf("\nStarting PostgreSQL container '%s'...\n", containerInfo.ContainerName)
	if err := docker.StartPostgreSQLContainer(containerInfo); err != nil {
		fmt.Printf("Error starting container: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Container started successfully\n")

	// Wait for container to be ready
	fmt.Println("Waiting for PostgreSQL to be ready...")
	if err := docker.WaitForContainerReady(containerInfo.ContainerName, 60*time.Second); err != nil {
		fmt.Printf("Error: %v\n", err)
		fmt.Printf("Container logs:\n")
		logs, logErr := docker.GetContainerLogs(containerInfo.ContainerName)
		if logErr == nil {
			fmt.Println(logs)
		}
		os.Exit(1)
	}
	fmt.Println("✓ PostgreSQL is ready")

	fmt.Printf("\n✓ Container created and PostgreSQL is ready!\n")
	fmt.Printf("\nConnection details:\n")
	fmt.Printf("  Host: localhost\n")
	fmt.Printf("  Port: %s\n", containerInfo.Port)
	fmt.Printf("  Username: postgres\n")
	fmt.Printf("  Password: %s\n", maskPassword(containerInfo.Password))
	fmt.Printf("  Database: %s\n", containerInfo.DatabaseName)
	fmt.Printf("\nYou can now connect to this database using the 'Connect to a DB' option.\n")
}

func init() {
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(connectCmd)
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(deleteCmd)
	// rootCmd.AddCommand(dbCmd)
}
