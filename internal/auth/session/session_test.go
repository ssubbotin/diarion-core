//go:build integration

package session_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/diarion/diarion-core/internal/auth/session"
	"github.com/diarion/diarion-core/internal/db"
	"github.com/diarion/diarion-core/internal/db/dbgen"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func setupTestQueries(t *testing.T) (*dbgen.Queries, func()) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)

	pgC, err := postgres.Run(ctx,
		"postgres:18-alpine",
		postgres.WithDatabase("diarion_test"),
		postgres.WithUsername("diarion"),
		postgres.WithPassword("diarion"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		cancel()
		t.Fatalf("start postgres: %v", err)
	}
	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = pgC.Terminate(ctx)
		cancel()
		t.Fatalf("dsn: %v", err)
	}
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

func TestManager_IssueAndLookup(t *testing.T) {
	t.Parallel()
	q, cleanup := setupTestQueries(t)
	defer cleanup()
	ctx := context.Background()

	u, _ := q.InsertUser(ctx, dbgen.InsertUserParams{
		GoogleSub: "g-sess-1", Email: "sess1@e.com", DisplayName: "S1",
	})

	mgr := session.NewManager(q, false)

	w := httptest.NewRecorder()
	s, err := mgr.Issue(ctx, w, u.ID)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if s.UserID != u.ID {
		t.Errorf("session user mismatch")
	}

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	if cookies[0].Name != session.CookieName {
		t.Errorf("cookie name = %q, want %q", cookies[0].Name, session.CookieName)
	}
	if !cookies[0].HttpOnly {
		t.Errorf("cookie should be HttpOnly")
	}
	if cookies[0].SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie SameSite should be Lax")
	}

	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(cookies[0])

	got, err := mgr.Lookup(ctx, r)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if got.ID != s.ID {
		t.Errorf("Lookup returned wrong session id")
	}
}

func TestManager_Revoke(t *testing.T) {
	t.Parallel()
	q, cleanup := setupTestQueries(t)
	defer cleanup()
	ctx := context.Background()

	u, _ := q.InsertUser(ctx, dbgen.InsertUserParams{
		GoogleSub: "g-sess-2", Email: "sess2@e.com", DisplayName: "S2",
	})

	mgr := session.NewManager(q, false)

	w1 := httptest.NewRecorder()
	if _, err := mgr.Issue(ctx, w1, u.ID); err != nil {
		t.Fatalf("Issue: %v", err)
	}
	cookies := w1.Result().Cookies()

	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(cookies[0])

	w2 := httptest.NewRecorder()
	if err := mgr.Revoke(ctx, w2, r); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	if _, err := mgr.Lookup(ctx, r); err == nil {
		t.Errorf("expected ErrNoSession after Revoke")
	} else if !strings.Contains(err.Error(), "no session") {
		t.Errorf("unexpected error: %v", err)
	}

	revokedCookies := w2.Result().Cookies()
	if len(revokedCookies) == 0 || revokedCookies[0].MaxAge >= 0 {
		t.Errorf("expected cleared cookie with MaxAge<0; got %+v", revokedCookies)
	}
}

func TestManager_RevokeAllForUser(t *testing.T) {
	t.Parallel()
	q, cleanup := setupTestQueries(t)
	defer cleanup()
	ctx := context.Background()

	u, _ := q.InsertUser(ctx, dbgen.InsertUserParams{
		GoogleSub: "g-sess-3", Email: "sess3@e.com", DisplayName: "S3",
	})
	mgr := session.NewManager(q, false)

	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		if _, err := mgr.Issue(ctx, w, u.ID); err != nil {
			t.Fatalf("Issue %d: %v", i, err)
		}
	}

	w := httptest.NewRecorder()
	if err := mgr.RevokeAllForUser(ctx, w, u.ID); err != nil {
		t.Fatalf("RevokeAllForUser: %v", err)
	}
}
