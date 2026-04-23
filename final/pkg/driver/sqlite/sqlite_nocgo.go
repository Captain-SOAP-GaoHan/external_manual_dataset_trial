//go:build !cgo

package sqlite

import _ "modernc.org/sqlite" // pure Go database/sql driver

// driverName is the database/sql driver name for the pure Go SQLite driver
const driverName = "sqlite"
