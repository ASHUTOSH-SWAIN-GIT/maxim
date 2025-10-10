package db

import (
	"database/sql"
	"fmt"

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

	if _, err := adminDB.Exec(fmt.Sprintf("CREATE DATABASE %s", pq.QuoteIdentifier(dbName))); err != nil {
		return fmt.Errorf("could not create database: %w", err)
	}
	if _, err := adminDB.Exec(fmt.Sprintf("CREATE USER %s WITH PASSWORD '%s'", pq.QuoteIdentifier(newUser), newPassword)); err != nil {
		return fmt.Errorf("could not create user: %w", err)
	}
	if _, err := adminDB.Exec(fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE %s TO %s", pq.QuoteIdentifier(dbName), pq.QuoteIdentifier(newUser))); err != nil {
		return fmt.Errorf("could not grant privileges: %w", err)
	}
	return nil
}
