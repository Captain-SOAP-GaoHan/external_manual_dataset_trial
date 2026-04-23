package dbmate_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/amacneil/dbmate/v2/pkg/dbmate"
	"github.com/amacneil/dbmate/v2/pkg/dbtest"
	_ "github.com/amacneil/dbmate/v2/pkg/driver/sqlite"

	"github.com/stretchr/testify/require"
)

// newStatusTestDB creates a test DB with SQLite and captures output in a buffer
func newStatusTestDB(t *testing.T) (*dbmate.DB, *bytes.Buffer) {
	t.Helper()

	var buf bytes.Buffer
	u := dbtest.MustParseURL(t, "sqlite:status_test.sqlite3")

	var err error
	rootDir := ""
	if rootDir == "" {
		rootDir, err = filepath.Abs("../..")
		require.NoError(t, err)
	}
	t.Chdir(rootDir + "/testdata")

	db := dbmate.New(u)
	db.Log = &buf
	db.AutoDumpSchema = false

	return db, &buf
}

// TestStatusAppliedPendingCount tests feature 1: total applied/pending migration counts
func TestStatusAppliedPendingCount(t *testing.T) {
	db, buf := newStatusTestDB(t)

	// clean up
	err := db.Drop()
	require.NoError(t, err)
	err = db.Create()
	require.NoError(t, err)

	t.Run("all pending", func(t *testing.T) {
		pending, err := db.Status(false)
		require.NoError(t, err)
		require.Equal(t, 2, pending, "should have 2 pending migrations")

		output := buf.String()
		require.Contains(t, output, "Applied:          0")
		require.Contains(t, output, "Pending:          2")
		buf.Reset()
	})

	t.Run("one applied one pending", func(t *testing.T) {
		// Apply first migration only using a fake FS with only the first migration
		// We just run Migrate to apply all, then Rollback one
		err := db.Migrate()
		require.NoError(t, err)

		err = db.Rollback()
		require.NoError(t, err)

		pending, err := db.Status(false)
		require.NoError(t, err)
		require.Equal(t, 1, pending, "should have 1 pending migration")

		output := buf.String()
		require.Contains(t, output, "Applied:          1")
		require.Contains(t, output, "Pending:          1")
		buf.Reset()
	})

	t.Run("all applied", func(t *testing.T) {
		err := db.Migrate()
		require.NoError(t, err)

		pending, err := db.Status(false)
		require.NoError(t, err)
		require.Equal(t, 0, pending, "should have 0 pending migrations")

		output := buf.String()
		require.Contains(t, output, "Applied:          2")
		require.Contains(t, output, "Pending:          0")
		buf.Reset()
	})

	// cleanup
	_ = db.Drop()
}

// TestStatusMigrationList tests feature 2: list all migrations with their status
func TestStatusMigrationList(t *testing.T) {
	db, buf := newStatusTestDB(t)

	err := db.Drop()
	require.NoError(t, err)
	err = db.Create()
	require.NoError(t, err)

	t.Run("pending migrations shown", func(t *testing.T) {
		pending, err := db.Status(false)
		require.NoError(t, err)
		require.Equal(t, 2, pending)

		output := buf.String()
		// Both migrations should be shown with [ ] (pending) status
		require.Contains(t, output, "[ ]")
		require.Contains(t, output, "20151129054053")
		require.Contains(t, output, "20200227231541")
		buf.Reset()
	})

	t.Run("applied migrations shown", func(t *testing.T) {
		err := db.Migrate()
		require.NoError(t, err)

		pending, err := db.Status(false)
		require.NoError(t, err)
		require.Equal(t, 0, pending)

		output := buf.String()
		// Both migrations should be shown with [X] (applied) status
		require.Contains(t, output, "[X]")
		require.Contains(t, output, "20151129054053")
		require.Contains(t, output, "20200227231541")
		buf.Reset()
	})

	// cleanup
	_ = db.Drop()
}

// TestStatusAppliedTimestamp tests feature 3: execution timestamps for applied migrations
func TestStatusAppliedTimestamp(t *testing.T) {
	db, buf := newStatusTestDB(t)

	err := db.Drop()
	require.NoError(t, err)
	err = db.Create()
	require.NoError(t, err)

	t.Run("pending migrations show dash for applied_at", func(t *testing.T) {
		_, err := db.Status(false)
		require.NoError(t, err)

		output := buf.String()
		// Pending migrations should show "-" for Applied At
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			if strings.Contains(line, "[ ]") {
				require.Contains(t, line, " -", "pending migration should show '-' for applied_at")
			}
		}
		buf.Reset()
	})

	t.Run("applied migrations show timestamp", func(t *testing.T) {
		err := db.Migrate()
		require.NoError(t, err)

		_, err = db.Status(false)
		require.NoError(t, err)

		output := buf.String()
		// Applied migrations should show a timestamp (matching YYYY-MM-DD HH:MM:SS format)
		lines := strings.Split(output, "\n")
		appliedCount := 0
		for _, line := range lines {
			if strings.Contains(line, "[X]") {
				appliedCount++
				// Should contain a timestamp-like pattern
				// Format: 2006-01-02 15:04:05
				found := false
				for i := 0; i < len(line)-19; i++ {
					if line[i] >= '2' && line[i] <= '2' && line[i+4] == '-' && line[i+7] == '-' && line[i+10] == ' ' && line[i+13] == ':' && line[i+16] == ':' {
						found = true
						break
					}
				}
				require.True(t, found, "applied migration should have a timestamp, line: %s", line)
			}
		}
		require.Equal(t, 2, appliedCount, "should find 2 applied migrations")
		buf.Reset()
	})

	// cleanup
	_ = db.Drop()
}

// TestStatusTableFormat tests feature 4: table format output with borders and alignment
func TestStatusTableFormat(t *testing.T) {
	db, buf := newStatusTestDB(t)
	db.StatusFormat = "table"

	err := db.Drop()
	require.NoError(t, err)
	err = db.Create()
	require.NoError(t, err)

	t.Run("table format with borders", func(t *testing.T) {
		_, err := db.Status(false)
		require.NoError(t, err)

		output := buf.String()
		// Should contain table borders
		require.Contains(t, output, "+----------+")
		require.Contains(t, output, "| Status   |")
		require.Contains(t, output, "| Version  |")
		require.Contains(t, output, "| Migration Name")
		require.Contains(t, output, "| Applied At")
		// Should contain summary line
		require.Contains(t, output, "Total: 2 | Applied: 0 | Pending: 2")
		// Pending status should be spelled out
		require.Contains(t, output, "| pending  |")
		buf.Reset()
	})

	t.Run("table format with applied migrations", func(t *testing.T) {
		err := db.Migrate()
		require.NoError(t, err)

		_, err = db.Status(false)
		require.NoError(t, err)

		output := buf.String()
		require.Contains(t, output, "| applied  |")
		require.Contains(t, output, "Total: 2 | Applied: 2 | Pending: 0")
		buf.Reset()
	})

	// cleanup
	_ = db.Drop()
}

// TestStatusJSONFormat tests feature 5: JSON format output for automation
func TestStatusJSONFormat(t *testing.T) {
	db, buf := newStatusTestDB(t)
	db.StatusFormat = "json"

	err := db.Drop()
	require.NoError(t, err)
	err = db.Create()
	require.NoError(t, err)

	t.Run("json format with pending migrations", func(t *testing.T) {
		_, err := db.Status(false)
		require.NoError(t, err)

		output := buf.String()

		// Parse JSON output
		var result dbmate.StatusJSONOutput
		err = json.Unmarshal([]byte(strings.TrimSpace(output)), &result)
		require.NoError(t, err, "output should be valid JSON: %s", output)

		require.Equal(t, 2, result.Total)
		require.Equal(t, 0, result.Applied)
		require.Equal(t, 2, result.Pending)
		require.Len(t, result.Migrations, 2)

		// Check migration fields
		for _, m := range result.Migrations {
			require.False(t, m.Applied)
			require.Nil(t, m.AppliedAt)
			require.NotEmpty(t, m.Version)
			require.NotEmpty(t, m.FileName)
		}
		buf.Reset()
	})

	t.Run("json format with applied migrations", func(t *testing.T) {
		err := db.Migrate()
		require.NoError(t, err)

		_, err = db.Status(false)
		require.NoError(t, err)

		output := buf.String()

		var result dbmate.StatusJSONOutput
		err = json.Unmarshal([]byte(strings.TrimSpace(output)), &result)
		require.NoError(t, err, "output should be valid JSON: %s", output)

		require.Equal(t, 2, result.Total)
		require.Equal(t, 2, result.Applied)
		require.Equal(t, 0, result.Pending)
		require.Len(t, result.Migrations, 2)

		// Check applied migrations have timestamps
		for _, m := range result.Migrations {
			require.True(t, m.Applied)
			require.NotNil(t, m.AppliedAt, "applied migration should have applied_at timestamp")
			// Verify it's a valid RFC3339 timestamp
			_, err := time.Parse(time.RFC3339, *m.AppliedAt)
			require.NoError(t, err, "applied_at should be RFC3339 format: %s", *m.AppliedAt)
		}
		buf.Reset()
	})

	t.Run("json format with mixed applied/pending", func(t *testing.T) {
		err := db.Rollback()
		require.NoError(t, err)

		_, err = db.Status(false)
		require.NoError(t, err)

		output := buf.String()

		var result dbmate.StatusJSONOutput
		err = json.Unmarshal([]byte(strings.TrimSpace(output)), &result)
		require.NoError(t, err)

		require.Equal(t, 2, result.Total)
		require.Equal(t, 1, result.Applied)
		require.Equal(t, 1, result.Pending)
		require.Len(t, result.Migrations, 2)

		// First migration should be applied, second pending
		require.True(t, result.Migrations[0].Applied)
		require.NotNil(t, result.Migrations[0].AppliedAt)
		require.False(t, result.Migrations[1].Applied)
		require.Nil(t, result.Migrations[1].AppliedAt)
		buf.Reset()
	})

	// cleanup
	_ = db.Drop()
}

// TestStatusQuietMode tests that quiet mode returns pending count without output
func TestStatusQuietMode(t *testing.T) {
	db, buf := newStatusTestDB(t)

	err := db.Drop()
	require.NoError(t, err)
	err = db.Create()
	require.NoError(t, err)

	pending, err := db.Status(true)
	require.NoError(t, err)
	require.Equal(t, 2, pending)

	output := buf.String()
	require.Empty(t, output, "quiet mode should produce no output")

	// cleanup
	_ = db.Drop()
}

// TestStatusEmptyMigrations tests status when no migration files exist
func TestStatusEmptyMigrations(t *testing.T) {
	dir := t.TempDir()

	u := dbtest.MustParseURL(t, "sqlite:status_empty_test.sqlite3")
	db := dbmate.New(u)
	db.AutoDumpSchema = false
	db.MigrationsDir = []string{dir}

	var buf bytes.Buffer
	db.Log = &buf

	err := db.Drop()
	require.NoError(t, err)
	err = db.Create()
	require.NoError(t, err)

	pending, err := db.Status(false)
	require.NoError(t, err)
	require.Equal(t, 0, pending)

	output := buf.String()
	require.Contains(t, output, "Applied:          0")
	require.Contains(t, output, "Pending:          0")

	// cleanup
	_ = db.Drop()
}

// TestStatusWithCustomFS tests status using an in-memory filesystem
func TestStatusWithCustomFS(t *testing.T) {
	mapFS := fstest.MapFS{
		"db/migrations/20260101120000_create_users.sql": {
			Data: []byte("-- migrate:up\nCREATE TABLE users (id INTEGER);\n-- migrate:down\nDROP TABLE users;\n"),
		},
		"db/migrations/20260101120001_create_posts.sql": {
			Data: []byte("-- migrate:up\nCREATE TABLE posts (id INTEGER);\n-- migrate:down\nDROP TABLE posts;\n"),
		},
	}

	u := dbtest.MustParseURL(t, "sqlite:status_fs_test.sqlite3")
	db := dbmate.New(u)
	db.AutoDumpSchema = false
	db.FS = mapFS

	var buf bytes.Buffer
	db.Log = &buf

	err := db.Drop()
	require.NoError(t, err)
	err = db.Create()
	require.NoError(t, err)

	t.Run("dashboard format with custom FS", func(t *testing.T) {
		pending, err := db.Status(false)
		require.NoError(t, err)
		require.Equal(t, 2, pending)

		output := buf.String()
		require.Contains(t, output, "Pending:          2")
		require.Contains(t, output, "20260101120000")
		require.Contains(t, output, "20260101120001")
		buf.Reset()
	})

	t.Run("table format with custom FS", func(t *testing.T) {
		db.StatusFormat = "table"
		pending, err := db.Status(false)
		require.NoError(t, err)
		require.Equal(t, 2, pending)

		output := buf.String()
		require.Contains(t, output, "| pending  |")
		require.Contains(t, output, "Total: 2 | Applied: 0 | Pending: 2")
		buf.Reset()
	})

	t.Run("json format with custom FS", func(t *testing.T) {
		db.StatusFormat = "json"
		pending, err := db.Status(false)
		require.NoError(t, err)
		require.Equal(t, 2, pending)

		output := buf.String()
		var result dbmate.StatusJSONOutput
		err = json.Unmarshal([]byte(strings.TrimSpace(output)), &result)
		require.NoError(t, err)
		require.Equal(t, 2, result.Total)
		require.Equal(t, 0, result.Applied)
		require.Equal(t, 2, result.Pending)
		buf.Reset()
	})

	// cleanup
	_ = db.Drop()
}
