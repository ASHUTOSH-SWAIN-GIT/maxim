package db

import (
	"context"
	"net/url"
	"strings"
	"testing"
)

func TestPostgresDSNEscapesCredentialsAndEnablesSSL(t *testing.T) {
	dsn, err := postgresDSN(ConnectionOptions{
		Host: "db.example.com", Port: "5432", User: "user@example.com",
		Password: "quote' slash/ question?", Database: "app db", SSLMode: "verify-full",
	})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	password, _ := parsed.User.Password()
	if parsed.User.Username() != "user@example.com" || password != "quote' slash/ question?" {
		t.Fatalf("credentials were not preserved in DSN: %s", dsn)
	}
	if parsed.Query().Get("sslmode") != "verify-full" {
		t.Fatalf("SSL mode missing from DSN: %s", dsn)
	}
	if parsed.Query().Get("connect_timeout") != "10" {
		t.Fatalf("connection timeout missing from DSN: %s", dsn)
	}
}

func TestQueryContextErrorsHaveClearMessages(t *testing.T) {
	if got := formatQueryError(context.Canceled); got != "Query cancelled." {
		t.Fatalf("cancel message = %q", got)
	}
	if got := formatQueryError(context.DeadlineExceeded); !strings.Contains(got, QueryTimeout.String()) {
		t.Fatalf("timeout message does not name deadline: %q", got)
	}
}

func TestPostgresDSNRejectsInvalidOptions(t *testing.T) {
	for _, options := range []ConnectionOptions{
		{Port: "5432"},
		{Host: "localhost"},
		{Host: "localhost", Port: "5432", SSLMode: "unsafe"},
	} {
		if _, err := postgresDSN(options); err == nil {
			t.Fatalf("expected options to be rejected: %#v", options)
		}
	}
}
