//go:build integration

package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/diarion/diarion-core/internal/auth/pat"
	"github.com/diarion/diarion-core/internal/auth/session"
	"github.com/diarion/diarion-core/internal/db"
	"github.com/diarion/diarion-core/internal/db/dbgen"
	authmw "github.com/diarion/diarion-core/internal/middleware/auth"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func setupQueries(t *testing.T) (*dbgen.Queries, func()) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	pgC, err := postgres.Run(ctx, "postgres:18-alpine",
		postgres.WithDatabase("diarion_test"),
		postgres.WithUsername("diarion"),
		postgres.WithPassword("diarion"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		cancel()
		t.Fatalf("start postgres: %v", err)
	}
	dsn, _ := pgC.ConnectionString(ctx, "sslmode=disable")
	if err := db.Migrate(ctx, dsn); err != nil {
		_ = pgC.Terminate(ctx)
		cancel()
		t.Fatalf("migrate: %v", err)
	}
	pool, err := db.NewPool(ctx, dsn)
	if err != nil {
		_ = pgC.Terminate(ctx)
		cancel()
		t.Fatalf("pool: %v", err)
	}
	cleanup := func() {
		pool.Close()
		_ = pgC.Terminate(ctx)
		cancel()
	}
	return dbgen.New(pool), cleanup
}

func echoHandler(w http.ResponseWriter, r *http.Request) {
	u := authmw.FromContext(r.Context())
	if u == nil {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("anonymous"))
		return
	}
	if u.ViaPAT {
		_, _ = w.Write([]byte("pat:" + u.Email))
		return
	}
	_, _ = w.Write([]byte("session:" + u.Email))
}

func TestMiddleware_Anonymous(t *testing.T) {
	t.Parallel()
	q, cleanup := setupQueries(t)
	defer cleanup()

	mgr := session.NewManager(q, false)
	h := authmw.Middleware(q, mgr)(http.HandlerFunc(echoHandler))

	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if body := w.Body.String(); body != "anonymous" {
		t.Errorf("body = %q, want %q", body, "anonymous")
	}
}

func TestMiddleware_Session(t *testing.T) {
	t.Parallel()
	q, cleanup := setupQueries(t)
	defer cleanup()
	ctx := context.Background()

	u, _ := q.InsertUser(ctx, dbgen.InsertUserParams{
		GoogleSub: "g-mw-1", Email: "mw1@e.com", DisplayName: "MW1",
	})

	mgr := session.NewManager(q, false)
	w := httptest.NewRecorder()
	if _, err := mgr.Issue(ctx, w, u.ID); err != nil {
		t.Fatalf("Issue: %v", err)
	}
	cookies := w.Result().Cookies()

	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(cookies[0])
	w2 := httptest.NewRecorder()

	h := authmw.Middleware(q, mgr)(http.HandlerFunc(echoHandler))
	h.ServeHTTP(w2, r)
	if body := w2.Body.String(); body != "session:mw1@e.com" {
		t.Errorf("body = %q, want session:mw1@e.com", body)
	}
}

func TestMiddleware_PAT(t *testing.T) {
	t.Parallel()
	q, cleanup := setupQueries(t)
	defer cleanup()
	ctx := context.Background()

	u, _ := q.InsertUser(ctx, dbgen.InsertUserParams{
		GoogleSub: "g-mw-2", Email: "mw2@e.com", DisplayName: "MW2",
	})

	plain, hash, _ := pat.Generate()
	if _, err := q.InsertPAT(ctx, dbgen.InsertPATParams{
		UserID:    u.ID,
		TokenHash: hash,
		Name:      "test-pat",
		ExpiresAt: pgtype.Timestamptz{Valid: false},
	}); err != nil {
		t.Fatalf("InsertPAT: %v", err)
	}

	mgr := session.NewManager(q, false)
	h := authmw.Middleware(q, mgr)(http.HandlerFunc(echoHandler))

	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer "+plain)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if body := w.Body.String(); body != "pat:mw2@e.com" {
		t.Errorf("body = %q, want pat:mw2@e.com", body)
	}
}

func TestMiddleware_RevokedPATIsAnonymous(t *testing.T) {
	t.Parallel()
	q, cleanup := setupQueries(t)
	defer cleanup()
	ctx := context.Background()

	u, _ := q.InsertUser(ctx, dbgen.InsertUserParams{
		GoogleSub: "g-mw-3", Email: "mw3@e.com", DisplayName: "MW3",
	})
	plain, hash, _ := pat.Generate()
	row, _ := q.InsertPAT(ctx, dbgen.InsertPATParams{
		UserID:    u.ID,
		TokenHash: hash,
		Name:      "revoke-test",
		ExpiresAt: pgtype.Timestamptz{Valid: false},
	})
	if err := q.RevokePAT(ctx, dbgen.RevokePATParams{ID: row.ID, UserID: u.ID}); err != nil {
		t.Fatalf("RevokePAT: %v", err)
	}

	mgr := session.NewManager(q, false)
	h := authmw.Middleware(q, mgr)(http.HandlerFunc(echoHandler))

	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer "+plain)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if body := w.Body.String(); body != "anonymous" {
		t.Errorf("revoked PAT should resolve to anonymous, got %q", body)
	}
}
