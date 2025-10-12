package db

import (
	"database/sql"
	"fmt"

	"github.com/charmbracelet/bubbles/table"
	"github.com/lib/pq"
)

func ConnectAndVerify(dbType, user, password, host, port, dbname string) (*sql.DB, error) {
	var dsn string
	driverName := dbType

	switch dbType {
	case "psql":
		driverName = "postgres"
		dsn = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, dbname)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, err
	}

	if err = db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func CreateDBAndUser(adminDB *sql.DB, dbType, dbName, newUser, newPassword string) error {
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
	adminDSN := fmt.Sprintf("host=localhost port=5432 user=postgres password=postgres dbname=%s sslmode=disable", dbName)
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

func ListDatabases(db *sql.DB) ([]string, error) {
	query := "SELECT datname FROM pg_database WHERE datistemplate = false;"

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dbNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		dbNames = append(dbNames, name)
	}

	return dbNames, nil
}

func GetTables(db *sql.DB) ([]string, error) {
	query := "SELECT tablename FROM pg_catalog.pg_tables WHERE schemaname = 'public';"
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tableNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tableNames = append(tableNames, name)
	}
	return tableNames, nil
}

func GetTableData(db *sql.DB, tableName string) ([]table.Column, []table.Row, error) {
	rows, err := db.Query(fmt.Sprintf("SELECT * FROM %s LIMIT 100", pq.QuoteIdentifier(tableName)))
	if err != nil {
		return nil, nil, err
	}

	defer rows.Close()

	colNames, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}

	cols := make([]table.Column, len(colNames))
	for i, colName := range colNames {
		cols[i] = table.Column{Title: colName, Width: 15}
	}

	var tableRows []table.Row
	for rows.Next() {
		rowVals := make([]interface{}, len(colNames))
		scanArgs := make([]interface{}, len(colNames))

		for i := range rowVals {
			scanArgs[i] = &rowVals[i]
		}

		if err := rows.Scan(scanArgs...); err != nil {
			return nil, nil, err
		}

		row := make(table.Row, len(colNames))
		for i, val := range rowVals {
			if val == nil {
				row[i] = "NULL"
			} else {
				row[i] = fmt.Sprintf("%s", val)
			}
		}
		tableRows = append(tableRows, row)
	}
	return cols, tableRows, nil
}
