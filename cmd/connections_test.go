package cmd

import (
	"errors"
	"os"
	"testing"

	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/config"
	"github.com/ASHUTOSH-SWAIN-GIT/maxim/internal/tui"
)

func TestValidateConnectionResult(t *testing.T) {
	valid := tui.ConnectResult{
		Host: "localhost", Port: "5432", User: "maxim",
		DBName: "app", SSLMode: "disable",
	}
	if err := validateConnectionResult(valid); err != nil {
		t.Fatalf("valid connection rejected: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*tui.ConnectResult)
	}{
		{"host", func(result *tui.ConnectResult) { result.Host = "" }},
		{"port", func(result *tui.ConnectResult) { result.Port = "" }},
		{"username", func(result *tui.ConnectResult) { result.User = "" }},
		{"database", func(result *tui.ConnectResult) { result.DBName = "" }},
		{"SSL mode", func(result *tui.ConnectResult) { result.SSLMode = "" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := valid
			test.mutate(&result)
			if err := validateConnectionResult(result); err == nil {
				t.Fatal("invalid connection was accepted")
			}
		})
	}
}

func TestSaveConnectionProfileAs(t *testing.T) {
	t.Setenv("MAXIM_CONFIG_HOME", t.TempDir())
	result := tui.ConnectResult{
		Host: "db.example.com", Port: "5432", User: "maxim",
		Password: "never-store-this", DBName: "app", SSLMode: "require",
	}
	if err := saveConnectionProfileAs("production", result); err != nil {
		t.Fatal(err)
	}
	details, err := config.LoadDatabaseConnection("production")
	if err != nil {
		t.Fatal(err)
	}
	if details.Host != result.Host || details.User != result.User || details.SSLMode != result.SSLMode {
		t.Fatalf("saved details changed: %#v", details)
	}
	if _, err := config.LoadDatabaseConnection("maxim@db.example.com:5432/app"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("explicit profile name was not preserved")
	}
}
