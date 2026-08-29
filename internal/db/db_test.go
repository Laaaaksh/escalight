package db

import (
	"io/fs"
	"testing"
)

func TestOpenAppliesMigrations(t *testing.T) {
	conn, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	tables := []string{"users", "incidents", "escalation_policies", "escalation_steps", "schedules", "services"}
	for _, table := range tables {
		var name string
		err := conn.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name)
		if err != nil {
			t.Errorf("table %q missing after migrate: %v", table, err)
		}
	}
}

func TestOpenIsIdempotent(t *testing.T) {
	conn, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	// Re-running the migration step (simulating a restart against the same DB)
	// must not error or double-apply.
	if err := migrate(conn); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}
	var count int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if count != len(entries) {
		t.Errorf("expected %d migrations recorded (one per file), got %d", len(entries), count)
	}
}
