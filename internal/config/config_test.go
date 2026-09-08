package config

import (
	"errors"
	"os"
	"reflect"
	"testing"
)

func useTemporaryConfig(t *testing.T) {
	t.Helper()
	t.Setenv("MAXIM_CONFIG_HOME", t.TempDir())
}

func TestSaveAdminConnectionPreservesDatabaseConnections(t *testing.T) {
	useTemporaryConfig(t)
	database := ConnectionDetails{
		Host: "db.example.com", Port: "5432", User: "app",
		DBName: "orders", SSLMode: "verify-full",
	}
	if err := SaveDatabaseConnection("production", database, "ignored"); err != nil {
		t.Fatal(err)
	}
	if err := SaveAdminConnection(ConnectionDetails{
		Host: "localhost", Port: "5432", User: "postgres", DBName: "postgres",
	}, "ignored"); err != nil {
		t.Fatal(err)
	}

	got, err := LoadDatabaseConnection("production")
	if err != nil {
		t.Fatalf("saved database connection was lost: %v", err)
	}
	if !reflect.DeepEqual(*got, database) {
		t.Fatalf("connection changed: got %#v, want %#v", *got, database)
	}
}

func TestRenameDeleteAndSortedConnectionList(t *testing.T) {
	useTemporaryConfig(t)
	for _, name := range []string{"zeta", "alpha"} {
		if err := SaveDatabaseConnection(name, ConnectionDetails{Host: "localhost"}, ""); err != nil {
			t.Fatal(err)
		}
	}

	names, err := ListDatabaseConnections()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(names, []string{"alpha", "zeta"}) {
		t.Fatalf("names are not sorted: %v", names)
	}
	if err := RenameDatabaseConnection("zeta", "beta"); err != nil {
		t.Fatal(err)
	}
	if err := DeleteDatabaseConnection("beta"); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadDatabaseConnection("beta"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("deleted connection still exists: %v", err)
	}
}

func TestConfigurationNeverStoresPassword(t *testing.T) {
	useTemporaryConfig(t)
	if err := SaveDatabaseConnection("local", ConnectionDetails{Host: "localhost"}, "secret"); err != nil {
		t.Fatal(err)
	}
	configPath, err := getConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) == "" || contains(string(contents), "secret") {
		t.Fatalf("password was written to configuration: %s", contents)
	}
}

func contains(value, substring string) bool {
	for i := 0; i+len(substring) <= len(value); i++ {
		if value[i:i+len(substring)] == substring {
			return true
		}
	}
	return false
}
