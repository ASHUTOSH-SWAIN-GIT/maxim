package db

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"github.com/lib/pq"
)

func ConnectAndVerify(dbType, user, password, host, port, dbname string) (*sql.DB, error) {
	var dsn string
	driverName := dbType

	if dbType == "psql" {
		driverName = "postgres"
		dsn = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, dbname)
	} else if dbType == "mysql" {
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", user, password, host, port, dbname)
	} else {
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

func CreateDBAndUser(adminDB *sql.DB, dbName, newUser, newPassword string) error {
	_, err := adminDB.Exec(fmt.Sprintf("CREATE DATABASE %s", pq.QuoteIdentifier(dbName)))

	if err != nil {
		return fmt.Errorf("could not create database:%w", err)
	}

	_, err = adminDB.Exec(fmt.Sprintf("CREATE USER %s WITH PASSWORD '%s'", pq.QuoteIdentifier(newUser), newPassword))
	if err != nil {
		return fmt.Errorf("could not create user: %w", err)
	}

	_, err = adminDB.Exec(fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE %s TO %s", pq.QuoteIdentifier(dbName), pq.QuoteIdentifier(newUser)))
	if err != nil {
		return fmt.Errorf("could not grant privileges: %w", err)
	}

	return nil
}
