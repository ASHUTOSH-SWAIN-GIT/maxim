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
