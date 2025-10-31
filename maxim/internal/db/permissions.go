package db

import (
	"database/sql"
	"fmt"

	"github.com/lib/pq"
)

// CreateDBAndUser creates a new database and user with full permissions
func CreateDBAndUser(adminDB *sql.DB, dbType, dbName, newUser, newPassword, adminUser, adminPassword, adminHost, adminPort string) error {
	if dbType != "psql" {
		return fmt.Errorf("unsupported database type: %s", dbType)
	}

	// Create the database
	if _, err := adminDB.Exec(fmt.Sprintf("CREATE DATABASE %s", pq.QuoteIdentifier(dbName))); err != nil {
		return fmt.Errorf("could not create database: %w", err)
	}

	// Create the user
	if _, err := adminDB.Exec(fmt.Sprintf("CREATE USER %s WITH PASSWORD '%s'", pq.QuoteIdentifier(newUser), newPassword)); err != nil {
		return fmt.Errorf("could not create user: %w", err)
	}

	// Make the new user the owner of the database
	if _, err := adminDB.Exec(fmt.Sprintf("ALTER DATABASE %s OWNER TO %s", pq.QuoteIdentifier(dbName), pq.QuoteIdentifier(newUser))); err != nil {
		return fmt.Errorf("could not set database owner: %w", err)
	}

	// Grant database-level privileges
	if _, err := adminDB.Exec(fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE %s TO %s", pq.QuoteIdentifier(dbName), pq.QuoteIdentifier(newUser))); err != nil {
		return fmt.Errorf("could not grant database privileges: %w", err)
	}

	// Connect to the new database to grant schema and table permissions
	// We'll use the admin connection to grant permissions
	adminDSN := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", adminHost, adminPort, adminUser, adminPassword, dbName)
	newDB, err := sql.Open("postgres", adminDSN)
	if err != nil {
		return fmt.Errorf("could not connect to new database: %w", err)
	}
	defer newDB.Close()

	// Grant schema-level privileges on public schema
	if _, err := newDB.Exec(fmt.Sprintf("GRANT ALL ON SCHEMA public TO %s", pq.QuoteIdentifier(newUser))); err != nil {
		return fmt.Errorf("could not grant schema privileges: %w", err)
	}

	// Grant privileges on all existing tables in public schema
	if _, err := newDB.Exec(fmt.Sprintf("GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO %s", pq.QuoteIdentifier(newUser))); err != nil {
		return fmt.Errorf("could not grant table privileges: %w", err)
	}

	// Grant privileges on all existing sequences in public schema
	if _, err := newDB.Exec(fmt.Sprintf("GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO %s", pq.QuoteIdentifier(newUser))); err != nil {
		return fmt.Errorf("could not grant sequence privileges: %w", err)
	}

	// Grant privileges on all existing functions in public schema
	if _, err := newDB.Exec(fmt.Sprintf("GRANT ALL PRIVILEGES ON ALL FUNCTIONS IN SCHEMA public TO %s", pq.QuoteIdentifier(newUser))); err != nil {
		return fmt.Errorf("could not grant function privileges: %w", err)
	}

	// Set default privileges for future objects created in public schema
	if _, err := newDB.Exec(fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO %s", pq.QuoteIdentifier(newUser))); err != nil {
		return fmt.Errorf("could not set default table privileges: %w", err)
	}

	if _, err := newDB.Exec(fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO %s", pq.QuoteIdentifier(newUser))); err != nil {
		return fmt.Errorf("could not set default sequence privileges: %w", err)
	}

	if _, err := newDB.Exec(fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON FUNCTIONS TO %s", pq.QuoteIdentifier(newUser))); err != nil {
		return fmt.Errorf("could not set default function privileges: %w", err)
	}

	return nil
}

// GrantPermissionsToUser grants all permissions on a database to an existing user
func GrantPermissionsToUser(adminDB *sql.DB, dbType, dbName, username, adminUser, adminPassword, adminHost, adminPort string) error {
	if dbType != "psql" {
		return fmt.Errorf("unsupported database type: %s", dbType)
	}

	// Grant database-level privileges
	if _, err := adminDB.Exec(fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE %s TO %s", pq.QuoteIdentifier(dbName), pq.QuoteIdentifier(username))); err != nil {
		return fmt.Errorf("could not grant database privileges: %w", err)
	}

	// Connect to the database to grant schema and table permissions
	// We'll use the admin connection to grant permissions
	adminDSN := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", adminHost, adminPort, adminUser, adminPassword, dbName)
	targetDB, err := sql.Open("postgres", adminDSN)
	if err != nil {
		return fmt.Errorf("could not connect to database: %w", err)
	}
	defer targetDB.Close()

	// Grant schema-level privileges on public schema
	if _, err := targetDB.Exec(fmt.Sprintf("GRANT ALL ON SCHEMA public TO %s", pq.QuoteIdentifier(username))); err != nil {
		return fmt.Errorf("could not grant schema privileges: %w", err)
	}

	// Grant privileges on all existing tables in public schema
	if _, err := targetDB.Exec(fmt.Sprintf("GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO %s", pq.QuoteIdentifier(username))); err != nil {
		return fmt.Errorf("could not grant table privileges: %w", err)
	}

	// Grant privileges on all existing sequences in public schema
	if _, err := targetDB.Exec(fmt.Sprintf("GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO %s", pq.QuoteIdentifier(username))); err != nil {
		return fmt.Errorf("could not grant sequence privileges: %w", err)
	}

	// Grant privileges on all existing functions in public schema
	if _, err := targetDB.Exec(fmt.Sprintf("GRANT ALL PRIVILEGES ON ALL FUNCTIONS IN SCHEMA public TO %s", pq.QuoteIdentifier(username))); err != nil {
		return fmt.Errorf("could not grant function privileges: %w", err)
	}

	// Set default privileges for future objects created in public schema
	if _, err := targetDB.Exec(fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO %s", pq.QuoteIdentifier(username))); err != nil {
		return fmt.Errorf("could not set default table privileges: %w", err)
	}

	if _, err := targetDB.Exec(fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO %s", pq.QuoteIdentifier(username))); err != nil {
		return fmt.Errorf("could not set default sequence privileges: %w", err)
	}

	if _, err := targetDB.Exec(fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON FUNCTIONS TO %s", pq.QuoteIdentifier(username))); err != nil {
		return fmt.Errorf("could not set default function privileges: %w", err)
	}

	return nil
}

// GrantTablePermissions grants permissions on specific tables to a user
func GrantTablePermissions(adminDB *sql.DB, dbType, dbName, username string, tableNames []string, adminUser, adminPassword, adminHost, adminPort string) error {
	if dbType != "psql" {
		return fmt.Errorf("unsupported database type: %s", dbType)
	}

	// Connect to the database
	adminDSN := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", adminHost, adminPort, adminUser, adminPassword, dbName)
	targetDB, err := sql.Open("postgres", adminDSN)
	if err != nil {
		return fmt.Errorf("could not connect to database: %w", err)
	}
	defer targetDB.Close()

	// Grant permissions on each specific table
	for _, tableName := range tableNames {
		if _, err := targetDB.Exec(fmt.Sprintf("GRANT ALL PRIVILEGES ON TABLE %s TO %s", pq.QuoteIdentifier(tableName), pq.QuoteIdentifier(username))); err != nil {
			return fmt.Errorf("could not grant privileges on table %s: %w", tableName, err)
		}
	}

	return nil
}
