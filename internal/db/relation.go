package db

import (
	"fmt"
	"strings"

	"github.com/lib/pq"
)

// Relation is the complete identity of a PostgreSQL table. Schema and table
// remain separate until each is quoted, so dots and mixed case are unambiguous.
type Relation struct {
	Schema string
	Name   string
}

func (relation Relation) Validate() error {
	if strings.TrimSpace(relation.Schema) == "" {
		return fmt.Errorf("schema name is required")
	}
	if strings.TrimSpace(relation.Name) == "" {
		return fmt.Errorf("table name is required")
	}
	return nil
}

func (relation Relation) DisplayName() string {
	return relation.Schema + "." + relation.Name
}

func (relation Relation) QualifiedName() string {
	return pq.QuoteIdentifier(relation.Schema) + "." + pq.QuoteIdentifier(relation.Name)
}
