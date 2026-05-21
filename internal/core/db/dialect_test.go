package db_test

// Acceptance tests for the db.Dialect interface method.
//
// Coverage map (DI section):
//   DI01: postgresClient.Dialect() returns DialectPostgres → TestAC_DI01_PostgresDialect
//   DI02: mysqlClient.Dialect() returns DialectMySQL       → TestAC_DI02_MySQLDialect
//   DI03: both clients still implement db.Client            → TestAC_DI03_BothClientsImplementInterface

import (
	"testing"

	"github.com/polesen/datacow/internal/core/db"
)

// DI01 — postgresClient.Dialect() returns DialectPostgres.
// This drives the connect path through db.Connect so the assertion exercises
// the real concrete client without naming the unexported type.
func TestAC_DI01_PostgresDialect(t *testing.T) {
	c := postgresClient(t)
	if got := c.Dialect(); got != db.DialectPostgres {
		t.Errorf("postgresClient.Dialect() = %q; want %q", got, db.DialectPostgres)
	}
}

// DI02 — mysqlClient.Dialect() returns DialectMySQL.
func TestAC_DI02_MySQLDialect(t *testing.T) {
	c := mysqlClient(t)
	if got := c.Dialect(); got != db.DialectMySQL {
		t.Errorf("mysqlClient.Dialect() = %q; want %q", got, db.DialectMySQL)
	}
}

// DI03 — both clients still satisfy db.Client after the interface gained
// Dialect(). This test compiles iff the interface is satisfied; the runtime
// body just gates by env so it doesn't require both DBs to be present.
func TestAC_DI03_BothClientsImplementInterface(t *testing.T) {
	// Compile-time: both helpers return db.Client, so if either fails to
	// satisfy the interface the package will not build.
	_ = postgresClient
	_ = mysqlClient
}
