package cmd

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/config"
	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/db"
	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/tui"
	"golang.org/x/term"
)

// AdminConnectionInfo holds the database connection and admin credentials
type AdminConnectionInfo struct {
	DB       *sql.DB
	User     string
	Password string
	Host     string
	Port     string
}

// getAdminConnection loads saved admin credentials or prompts the user to enter them.
// It returns a connected database handle or exits on error.
func getAdminConnection() (*sql.DB, error) {
	info, err := getAdminConnectionInfo()
	if err != nil {
		return nil, err
	}
	return info.DB, nil
}

// getAdminConnectionInfo loads saved admin credentials or prompts the user to enter them.
// It returns both the connected database handle and admin credentials.
func getAdminConnectionInfo() (*AdminConnectionInfo, error) {
	details, err := config.LoadAdminConnection()
	if err != nil {
		// No saved credentials, prompt user for all details
		fmt.Println("No saved superuser credentials found.")
		fmt.Println("Please enter Postgres superuser credentials:")

		result, err := tui.RunAdminForm()
		if err != nil {
			return nil, fmt.Errorf("could not open credentials form: %w", err)
		}

		if result.Quitting {
			fmt.Println("Cancelled: operation aborted by user.")
			os.Exit(0)
		}

		// Try to connect with provided credentials (always use postgres database for superuser)
		adminDB, err := db.ConnectAndVerify("psql", result.User, result.Password, "localhost", result.Port, "postgres")
		if err != nil {
			return nil, fmt.Errorf("connection failed: %w", err)
		}

		// Save credentials (except password) for future use
		detailsToSave := config.ConnectionDetails{
			Host:   "localhost",
			Port:   result.Port,
			User:   result.User,
			DBName: "postgres",
		}

		if err := config.SaveAdminConnection(detailsToSave, result.Password); err != nil {
			fmt.Printf("Warning: could not save credentials: %v\n", err)
		} else {
			fmt.Println("Superuser credentials saved successfully.")
		}

		return &AdminConnectionInfo{
			DB:       adminDB,
			User:     result.User,
			Password: result.Password,
			Host:     "localhost",
			Port:     result.Port,
		}, nil
	}

	// Credentials found, prompt for password only
	fmt.Printf("Enter the password of your superuser: %s\n", details.User)
	fmt.Print("Password: ")

	// Read password securely (hidden input)
	passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return nil, fmt.Errorf("could not read password: %w", err)
	}
	fmt.Println() // New line after hidden input

	password := strings.TrimSpace(string(passwordBytes))
	if password == "" {
		fmt.Println("Cancelled: no password entered.")
		os.Exit(0)
	}

	// Connect with saved credentials + entered password
	adminDB, err := db.ConnectAndVerify("psql", details.User, password, details.Host, details.Port, details.DBName)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}

	return &AdminConnectionInfo{
		DB:       adminDB,
		User:     details.User,
		Password: password,
		Host:     details.Host,
		Port:     details.Port,
	}, nil
}
