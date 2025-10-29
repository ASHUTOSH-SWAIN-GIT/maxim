package tui

import (
	"sort"
	"strings"
)

// QueryCache stores and manages SQL query commands for autocomplete
type QueryCache struct {
	commands map[string]int // command -> frequency
	keywords []string       // sorted list of common SQL keywords
	columns  []string       // cached table columns
	tables   []string       // cached table names
}

// NewQueryCache creates a new query cache with common SQL keywords
func NewQueryCache() *QueryCache {
	cache := &QueryCache{
		commands: make(map[string]int),
		keywords: []string{
			"SELECT", "FROM", "WHERE", "INSERT", "INTO", "VALUES", "UPDATE", "SET",
			"DELETE", "CREATE", "TABLE", "DATABASE", "ALTER", "DROP", "INDEX",
			"JOIN", "INNER", "LEFT", "RIGHT", "OUTER", "ON", "AS", "GROUP", "BY",
			"ORDER", "HAVING", "LIMIT", "OFFSET", "DISTINCT", "COUNT", "SUM",
			"AVG", "MIN", "MAX", "AND", "OR", "NOT", "IN", "BETWEEN", "LIKE",
			"IS", "NULL", "TRUE", "FALSE", "ASC", "DESC", "UNION", "ALL",
			"EXISTS", "CASE", "WHEN", "THEN", "ELSE", "END", "IF", "EXISTS",
			"PRIMARY", "KEY", "NOT", "NULL", "VARCHAR", "INT", "INTEGER", "TEXT",
			"CHAR", "DECIMAL", "FLOAT", "DOUBLE", "BOOLEAN", "DATE", "TIME",
			"TIMESTAMP", "DATETIME", "BLOB", "CLOB", "AUTO_INCREMENT", "UNIQUE",
			"FOREIGN", "REFERENCES", "CONSTRAINT", "CHECK", "DEFAULT", "COMMENT", "SERIAL",
		},
	}

	// Sort keywords for efficient searching
	sort.Strings(cache.keywords)
	return cache
}

// AddCommand adds a command to the cache or increments its frequency
func (qc *QueryCache) AddCommand(command string) {
	// Extract individual keywords from the command
	words := strings.Fields(strings.ToUpper(command))
	for _, word := range words {
		// Only add if it's a meaningful SQL keyword (not too short)
		if len(word) > 1 && qc.isSQLKeyword(word) {
			qc.commands[word]++
		}
	}
}

// isSQLKeyword checks if a word is a known SQL keyword
func (qc *QueryCache) isSQLKeyword(word string) bool {
	for _, keyword := range qc.keywords {
		if keyword == word {
			return true
		}
	}
	return false
}

// GetMostUsedCommands returns the most frequently used commands
func (qc *QueryCache) GetMostUsedCommands(limit int) []string {
	type commandFreq struct {
		command string
		freq    int
	}

	var commands []commandFreq
	for cmd, freq := range qc.commands {
		commands = append(commands, commandFreq{cmd, freq})
	}

	// Sort by frequency (descending)
	sort.Slice(commands, func(i, j int) bool {
		return commands[i].freq > commands[j].freq
	})

	var result []string
	for i, cmd := range commands {
		if i >= limit {
			break
		}
		result = append(result, cmd.command)
	}

	return result
}

// CacheColumns caches table columns for autocomplete
func (qc *QueryCache) CacheColumns(columns []string) {
	qc.columns = make([]string, len(columns))
	copy(qc.columns, columns)
	sort.Strings(qc.columns)
}

// CacheTables caches table names for autocomplete
func (qc *QueryCache) CacheTables(tables []string) {
	qc.tables = make([]string, len(tables))
	copy(qc.tables, tables)
	sort.Strings(qc.tables)
}

// GetSuggestions returns suggestions based on the current input
func (qc *QueryCache) GetSuggestions(input string) []string {
	if input == "" {
		return []string{}
	}

	input = strings.ToUpper(strings.TrimSpace(input))
	var suggestions []string

	// First, check for exact keyword matches
	for _, keyword := range qc.keywords {
		if strings.HasPrefix(keyword, input) {
			suggestions = append(suggestions, keyword)
		}
	}

	// Then, check cached columns
	for _, column := range qc.columns {
		if strings.HasPrefix(strings.ToUpper(column), input) {
			suggestions = append(suggestions, column)
		}
	}

	// Then, check cached tables
	for _, table := range qc.tables {
		if strings.HasPrefix(strings.ToUpper(table), input) {
			suggestions = append(suggestions, table)
		}
	}

	// Then, check cached commands (most frequent first)
	type commandFreq struct {
		command string
		freq    int
	}

	var cachedCommands []commandFreq
	for cmd, freq := range qc.commands {
		if strings.HasPrefix(cmd, input) {
			cachedCommands = append(cachedCommands, commandFreq{cmd, freq})
		}
	}

	// Sort by frequency (descending)
	sort.Slice(cachedCommands, func(i, j int) bool {
		return cachedCommands[i].freq > cachedCommands[j].freq
	})

	// Add cached commands to suggestions
	for _, cmd := range cachedCommands {
		suggestions = append(suggestions, cmd.command)
	}

	// Remove duplicates and limit results
	seen := make(map[string]bool)
	var uniqueSuggestions []string
	for _, suggestion := range suggestions {
		if !seen[suggestion] {
			seen[suggestion] = true
			uniqueSuggestions = append(uniqueSuggestions, suggestion)
			if len(uniqueSuggestions) >= 10 { // Limit to 10 suggestions
				break
			}
		}
	}

	return uniqueSuggestions
}
