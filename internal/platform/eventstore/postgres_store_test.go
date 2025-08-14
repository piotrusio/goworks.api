package eventstore

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/salesworks/s-works/api/internal/platform/database"
	"github.com/salesworks/s-works/api/internal/platform/messaging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type postgresTestFixture struct {
	db    *sql.DB
	store *PostgresStore
	t     *testing.T
}

func setup(t *testing.T) *postgresTestFixture {
	t.Helper()

	uri := os.Getenv("POSTGRES_URI")
	if uri == "" {
		t.Skip("Skipping integratino test: POSTGRES_URI env variable is not set")
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dbConn, err := database.NewPostgresDB(ctx, uri, 5, 5, 5*time.Minute, logger)
	require.NoError(t, err, "Failed to connect to postgres for test")

	store := NewPostgresStore(dbConn.Pool)

	t.Cleanup(func() {
		_, err := dbConn.Pool.Exec("DELETE FROM events")
		if err != nil {
			t.Fatalf("Failed to clean up test data: %v", err)
		}
	})

	return &postgresTestFixture{
		db:    dbConn.Pool,
		store: store,
		t:     t,
	}
}

func TestPostgresStore_Save(t *testing.T) {
	// --- Arrange ---
	fixture := setup(t)
	ctx := context.Background()

	envelope := messaging.NewEventEnvelope(
		"fabric.created",
		"FABRIC001",
		"Fabric",
		1,
		map[string]interface{}{"name": "Test Fabric"},
	)

	// --- Act ---
	err := fixture.store.Save(ctx, envelope)
	require.NoError(t, err, "Save should not return an error")

	// --- Assert ---
	var eventType string
	dbErr := fixture.db.QueryRowContext(
		ctx, "SELECT event_type FROM events WHERE event_id = $1", envelope.EventID,
	).Scan(&eventType)
	require.NoError(t, dbErr, "Event should be found in the database")
	assert.Equal(t, "fabric.created", eventType)
}
