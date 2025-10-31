package db

import (
	"database/sql"
	"fmt"
	"strings"
)

// SchemaCache holds cached database schema information for autocomplete
type SchemaCache struct {
	Tables      []string
	Columns     map[string][]string // table -> columns
	Functions   []string
	Keywords    []string
	DataTypes   []string
	initialized bool
}

// NewSchemaCache creates a new schema cache and loads the database schema
func NewSchemaCache(db *sql.DB) (*SchemaCache, error) {
	cache := &SchemaCache{
		Tables:    []string{},
		Columns:   make(map[string][]string),
		Functions: []string{},
		Keywords:  []string{},
		DataTypes: []string{},
	}

	if err := cache.LoadSchema(db); err != nil {
		return nil, fmt.Errorf("failed to load schema: %w", err)
	}

	return cache, nil
}

// LoadSchema loads the database schema into the cache
func (sc *SchemaCache) LoadSchema(db *sql.DB) error {
	// Load tables
	if err := sc.loadTables(db); err != nil {
		return fmt.Errorf("failed to load tables: %w", err)
	}

	// Load columns for each table
	if err := sc.loadColumns(db); err != nil {
		return fmt.Errorf("failed to load columns: %w", err)
	}

	// Load functions
	if err := sc.loadFunctions(db); err != nil {
		return fmt.Errorf("failed to load functions: %w", err)
	}

	// Set predefined keywords and data types
	sc.setPredefinedData()

	sc.initialized = true
	return nil
}

// loadTables loads all table names from the database
func (sc *SchemaCache) loadTables(db *sql.DB) error {
	query := `
		SELECT table_name 
		FROM information_schema.tables 
		WHERE table_schema = 'public' 
		ORDER BY table_name
	`

	rows, err := db.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return err
		}
		sc.Tables = append(sc.Tables, tableName)
	}

	return rows.Err()
}

// loadColumns loads column names for each table
func (sc *SchemaCache) loadColumns(db *sql.DB) error {
	for _, table := range sc.Tables {
		query := `
			SELECT column_name 
			FROM information_schema.columns 
			WHERE table_schema = 'public' 
			AND table_name = $1 
			ORDER BY ordinal_position
		`

		rows, err := db.Query(query, table)
		if err != nil {
			continue // Skip tables that can't be queried
		}

		var columns []string
		for rows.Next() {
			var columnName string
			if err := rows.Scan(&columnName); err != nil {
				rows.Close()
				continue
			}
			columns = append(columns, columnName)
		}
		rows.Close()

		sc.Columns[table] = columns
	}

	return nil
}

// loadFunctions loads available database functions
func (sc *SchemaCache) loadFunctions(db *sql.DB) error {
	query := `
		SELECT routine_name 
		FROM information_schema.routines 
		WHERE routine_schema = 'public' 
		AND routine_type = 'FUNCTION'
		ORDER BY routine_name
	`

	rows, err := db.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var functionName string
		if err := rows.Scan(&functionName); err != nil {
			return err
		}
		sc.Functions = append(sc.Functions, functionName)
	}

	return rows.Err()
}

// setPredefinedData sets common SQL keywords and data types
func (sc *SchemaCache) setPredefinedData() {
	sc.Keywords = []string{
		"SELECT", "FROM", "WHERE", "INSERT", "INTO", "VALUES", "UPDATE", "SET",
		"DELETE", "CREATE", "TABLE", "ALTER", "DROP", "INDEX", "VIEW", "DATABASE",
		"GRANT", "REVOKE", "COMMIT", "ROLLBACK", "BEGIN", "END", "TRANSACTION",
		"ORDER", "BY", "GROUP", "HAVING", "JOIN", "INNER", "LEFT", "RIGHT", "OUTER",
		"ON", "AS", "DISTINCT", "COUNT", "SUM", "AVG", "MIN", "MAX", "LIMIT", "OFFSET",
		"AND", "OR", "NOT", "IN", "EXISTS", "BETWEEN", "LIKE", "IS", "NULL",
		"ASC", "DESC", "UNION", "ALL", "CASE", "WHEN", "THEN", "ELSE", "END",
	}

	sc.DataTypes = []string{
		"INTEGER", "BIGINT", "SMALLINT", "DECIMAL", "NUMERIC", "REAL", "DOUBLE",
		"PRECISION", "MONEY", "CHAR", "VARCHAR", "TEXT", "BYTEA", "TIMESTAMP",
		"DATE", "TIME", "INTERVAL", "BOOLEAN", "UUID", "JSON", "JSONB", "ARRAY",
	}
}

// GetSuggestions returns autocomplete suggestions based on the current query and cursor position
func (sc *SchemaCache) GetSuggestions(query string, cursorPos int) []string {
	if !sc.initialized {
		return []string{}
	}

	var suggestions []string
	queryLower := strings.ToLower(query)

	// Get the current word being typed
	currentWord := sc.getCurrentWord(query, cursorPos)
	currentWordLower := strings.ToLower(currentWord)

	// If we're at the beginning or after common keywords, suggest tables
	if sc.shouldSuggestTables(queryLower, cursorPos) {
		for _, table := range sc.Tables {
			if strings.HasPrefix(strings.ToLower(table), currentWordLower) {
				suggestions = append(suggestions, table)
			}
		}
	}

	// If we're after FROM or JOIN, suggest tables
	if sc.isAfterFromOrJoin(queryLower, cursorPos) {
		for _, table := range sc.Tables {
			if strings.HasPrefix(strings.ToLower(table), currentWordLower) {
				suggestions = append(suggestions, table)
			}
		}
	}

	// If we're after a table name, suggest columns
	if tableName := sc.getTableNameBeforeCursor(query, cursorPos); tableName != "" {
		if columns, exists := sc.Columns[tableName]; exists {
			for _, column := range columns {
				if strings.HasPrefix(strings.ToLower(column), currentWordLower) {
					suggestions = append(suggestions, column)
				}
			}
		}
	}

	// Suggest SQL keywords
	for _, keyword := range sc.Keywords {
		if strings.HasPrefix(strings.ToLower(keyword), currentWordLower) {
			suggestions = append(suggestions, keyword)
		}
	}

	// Suggest functions
	for _, function := range sc.Functions {
		if strings.HasPrefix(strings.ToLower(function), currentWordLower) {
			suggestions = append(suggestions, function+"()")
		}
	}

	// Limit suggestions to prevent overwhelming the user
	if len(suggestions) > 20 {
		suggestions = suggestions[:20]
	}

	return suggestions
}

// Helper methods for suggestion logic

func (sc *SchemaCache) getCurrentWord(query string, cursorPos int) string {
	if cursorPos <= 0 || cursorPos > len(query) {
		return ""
	}

	// Find the start of the current word
	start := cursorPos - 1
	for start >= 0 && (isAlphanumeric(query[start]) || query[start] == '_') {
		start--
	}
	start++

	// Find the end of the current word
	end := cursorPos
	for end < len(query) && (isAlphanumeric(query[end]) || query[end] == '_') {
		end++
	}

	return query[start:end]
}

func (sc *SchemaCache) shouldSuggestTables(queryLower string, cursorPos int) bool {
	// Check if we're at the beginning of the query or after common keywords
	beforeCursor := queryLower[:cursorPos]

	// Remove the current word being typed
	words := strings.Fields(beforeCursor)
	if len(words) == 0 {
		return true
	}

	lastWord := words[len(words)-1]
	return lastWord == "from" || lastWord == "join" || lastWord == "update" || lastWord == "into"
}

func (sc *SchemaCache) isAfterFromOrJoin(queryLower string, cursorPos int) bool {
	beforeCursor := queryLower[:cursorPos]
	return strings.Contains(beforeCursor, " from ") || strings.Contains(beforeCursor, " join ")
}

func (sc *SchemaCache) getTableNameBeforeCursor(query string, cursorPos int) string {
	beforeCursor := query[:cursorPos]
	words := strings.Fields(beforeCursor)

	// Look for table name after FROM or JOIN
	for i, word := range words {
		if (strings.ToLower(word) == "from" || strings.ToLower(word) == "join") && i+1 < len(words) {
			// Check if the next word is a table name
			nextWord := words[i+1]
			for _, table := range sc.Tables {
				if strings.EqualFold(table, nextWord) {
					return table
				}
			}
		}
	}

	return ""
}

func isAlphanumeric(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}
