//go:build cgo

package sqlite

import _ "github.com/mattn/go-sqlite3" // CGo database/sql driver

// driverName is the database/sql driver name for the CGo SQLite driver
const driverName = "sqlite3"
