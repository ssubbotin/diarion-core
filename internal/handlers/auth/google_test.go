//go:build integration

package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/diarion/diarion-core/internal/auth/oauth"
	authsess "github.com/diarion/diarion-core/internal/auth/session"
	"github.com/diarion/diarion-core/internal/db"
	"github.com/diarion/diarion-core/internal/db/dbgen"
	"github.com/diarion/diarion-core/internal/handlers/auth"
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupAll(t *testing.T) (*dbgen.Queries, *redis.Client, func()) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)

	rReq := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections"),
	}
	rC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: rReq,
		Started:          true,
	})
	if err != nil {
		cancel()
		t.Fatalf("start redis: %v", err)
	}
	rHost, _ := rC.Host(ctx)
	rPort, _ := rC.MappedPort(ctx, "6379")
	rClient := redis.NewClient(&redis.Options{Addr: rHost + ":" + rPort.Port()})

	pgC, err := postgres.Run(ctx, "postgres:18-alpine",
		postgres.WithDatabase("diarion_test"),
		postgres.WithUsername("diarion"),
		postgres.WithPassword("diarion"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		_ = rC.Terminate(ctx)
		cancel()
		t.Fatalf("start postgres: %v", err)
	}
	dsn, _ := pgC.ConnectionString(ctx, "sslmode=disable")
	if err := db.Migrate(ctx, dsn); err != nil {
		_ = rC.Terminate(ctx)
		_ = pgC.Terminate(ctx)
		cancel()
		t.Fatalf("migrate: %v", err)
	}
	pool, err := db.NewPool(ctx, dsn)
	if err != nil {
		_ = rC.Terminate(ctx)
		_ = pgC.Terminate(ctx)
		cancel()
		t.Fatalf("pool: %v", err)
	}

	cleanup := func() {
		pool.Close()
		_ = rClient.Close()
		_ = pgC.Terminate(ctx)
		_ = rC.Terminate(ctx)
		cancel()
	}
	return dbgen.New(pool), rClient, cleanup
}

func TestGoogleStart_RedirectsToProvider(t *testing.T) {
	t.Parallel()
	q, rClient, cleanup := setupAll(t)
	defer cleanup()

	fakeProvider := &oauth.FakeProvider{}
	sessions := authsess.NewManager(q, false)
	h := auth.New(fakeProvider, sessions, q, rClient)

	r := httptest.NewRequest("GET", "/auth/google?return_url=/", nil)
	w := httptest.NewRecorder()
	h.GoogleStart(w, r)

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", w.Code)
	}
	loc := w.Header().Get("Location")
	if !strings.HasPrefix(loc, "https://fake-oauth.example.test/auth?") {
		t.Errorf("Location = %q, want fake-oauth URL prefix", loc)
	}
}

func TestGoogleCallback_HappyPath(t *testing.T) {
	t.Parallel()
	q, rClient, cleanup := setupAll(t)
	defer cleanup()

	fakeProvider := &oauth.FakeProvider{
		ScriptedClaims: map[string]*oauth.Claims{
			"good-code": {
				Sub:           "g-sub-99",
				Email:         "happy@example.com",
				EmailVerified: true,
				Name:          "Happy User",
				Picture:       "https://example.com/pic.png",
			},
		},
	}
	sessions := authsess.NewManager(q, false)
	h := auth.New(fakeProvider, sessions, q, rClient)

	r1 := httptest.NewRequest("GET", "/auth/google?return_url=/welcome", nil)
	w1 := httptest.NewRecorder()
	h.GoogleStart(w1, r1)
	if w1.Code != http.StatusFound {
		t.Fatalf("start status = %d", w1.Code)
	}
	loc, _ := url.Parse(w1.Header().Get("Location"))
	state := loc.Query().Get("state")
	if state == "" {
		t.Fatal("no state in redirect")
	}

	callbackURL := "/auth/google/callback?state=" + state + "&code=good-code"
	r2 := httptest.NewRequest("GET", callbackURL, nil)
	w2 := httptest.NewRecorder()
	h.GoogleCallback(w2, r2)

	if w2.Code != http.StatusFound {
		t.Fatalf("callback status = %d; body=%s", w2.Code, w2.Body.String())
	}
	if loc := w2.Header().Get("Location"); loc != "/welcome" {
		t.Errorf("Location = %q, want /welcome", loc)
	}
	cookies := w2.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != authsess.CookieName {
		t.Fatalf("expected session cookie, got %+v", cookies)
	}

	u, err := q.GetUserByGoogleSub(context.Background(), "g-sub-99")
	if err != nil {
		t.Fatalf("GetUserByGoogleSub: %v", err)
	}
	if u.Email != "happy@example.com" {
		t.Errorf("email mismatch: %q", u.Email)
	}
}

func TestGoogleCallback_InvalidState(t *testing.T) {
	t.Parallel()
	q, rClient, cleanup := setupAll(t)
	defer cleanup()

	h := auth.New(&oauth.FakeProvider{}, authsess.NewManager(q, false), q, rClient)

	r := httptest.NewRequest("GET", "/auth/google/callback?state=bogus&code=anything", nil)
	w := httptest.NewRecorder()
	h.GoogleCallback(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestGoogleCallback_OpenRedirectGuard(t *testing.T) {
	t.Parallel()
	q, rClient, cleanup := setupAll(t)
	defer cleanup()

	fakeProvider := &oauth.FakeProvider{
		ScriptedClaims: map[string]*oauth.Claims{
			"safe": {Sub: "ord-1", Email: "ord@e.com", EmailVerified: true, Name: "ORD"},
		},
	}
	h := auth.New(fakeProvider, authsess.NewManager(q, false), q, rClient)

	r1 := httptest.NewRequest("GET", "/auth/google?return_url=https://attacker.example", nil)
	w1 := httptest.NewRecorder()
	h.GoogleStart(w1, r1)
	loc, _ := url.Parse(w1.Header().Get("Location"))
	state := loc.Query().Get("state")

	r2 := httptest.NewRequest("GET", "/auth/google/callback?state="+state+"&code=safe", nil)
	w2 := httptest.NewRecorder()
	h.GoogleCallback(w2, r2)

	if got := w2.Header().Get("Location"); got != "/" {
		t.Errorf("malicious return_url should be rewritten to /; got %q", got)
	}
}
