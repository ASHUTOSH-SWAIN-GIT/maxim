//go:build integration

package cmd

import (
	"os"
	"testing"

	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/config"
	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/db"
	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/tui"
)

func TestIntegrationSavedProfileConnects(t *testing.T) {
	host := os.Getenv("MAXIM_TEST_DB_HOST")
	port := os.Getenv("MAXIM_TEST_DB_PORT")
	user := os.Getenv("MAXIM_TEST_DB_USER")
	password := os.Getenv("MAXIM_TEST_DB_PASSWORD")
	databaseName := os.Getenv("MAXIM_TEST_DB_NAME")
	if host == "" || port == "" || user == "" || databaseName == "" {
		t.Skip("integration database is not configured")
	}

	t.Setenv("MAXIM_CONFIG_HOME", t.TempDir())
	result := tui.ConnectResult{
		Host: host, Port: port, User: user, Password: password,
		DBName: databaseName, SSLMode: "disable",
	}
	if err := saveConnectionProfileAs("integration", result); err != nil {
		t.Fatalf("save connection profile: %v", err)
	}

	details, err := config.LoadDatabaseConnection("integration")
	if err != nil {
		t.Fatalf("load connection profile: %v", err)
	}
	connection, err := db.ConnectPostgres(db.ConnectionOptions{
		Host: details.Host, Port: details.Port, User: details.User,
		Password: password, Database: details.DBName, SSLMode: details.SSLMode,
	})
	if err != nil {
		t.Fatalf("connect using saved profile: %v", err)
	}
	defer connection.Close()

	var currentDatabase string
	if err := connection.QueryRow("SELECT current_database()").Scan(&currentDatabase); err != nil {
		t.Fatalf("query through saved profile: %v", err)
	}
	if currentDatabase != databaseName {
		t.Fatalf("connected to %q, want %q", currentDatabase, databaseName)
	}
}
