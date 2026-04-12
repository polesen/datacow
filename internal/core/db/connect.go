package db

import (
	"fmt"
	"net/url"
	"strings"
)

// Connect auto-detects the driver from the DSN prefix and returns a Client.
// Supported prefixes: postgres://, postgresql://, mysql://
// Also accepts native MySQL DSNs (user:pass@tcp(host:port)/db).
func Connect(dsn string) (Client, error) {
	switch {
	case strings.HasPrefix(dsn, "postgres://"), strings.HasPrefix(dsn, "postgresql://"):
		return newPostgresClient(dsn)
	case strings.HasPrefix(dsn, "mysql://"):
		native, err := mysqlURLToNative(dsn)
		if err != nil {
			return nil, fmt.Errorf("parse mysql DSN: %w", err)
		}
		return newMySQLClient(native)
	case strings.Contains(dsn, "@tcp("):
		return newMySQLClient(dsn)
	default:
		return nil, fmt.Errorf("unsupported DSN %q: prefix must be postgres://, mysql://, or native mysql DSN", dsn)
	}
}

// mysqlURLToNative converts a mysql:// URL to the go-sql-driver/mysql native format.
func mysqlURLToNative(dsn string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", err
	}
	host := u.Hostname()
	port := u.Port()
	if port == "" {
		port = "3306"
	}
	user := u.User.Username()
	pass, _ := u.User.Password()
	db := strings.TrimPrefix(u.Path, "/")
	params := u.RawQuery
	native := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", user, pass, host, port, db)
	if params != "" {
		native += "?" + params
	}
	return native, nil
}
