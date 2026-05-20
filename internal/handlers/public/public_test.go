//go:build integration

package public_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/diarion/diarion-core/internal/db"
	"github.com/diarion/diarion-core/internal/db/dbgen"
	"github.com/diarion/diarion-core/internal/handlers/public"
	"github.com/go-chi/chi/v5"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

type harness struct {
	URL     string
	Server  *httptest.Server
	Queries *dbgen.Queries
	cleanup func()
}

func (h *harness) Close() { h.cleanup() }

func setupHarness(t *testing.T) *harness {
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
	q := dbgen.New(pool)

	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) {
		public.New(q).Register(r)
	})
	srv := httptest.NewServer(r)
	cleanup := func() {
		srv.Close()
		pool.Close()
		_ = pgC.Terminate(ctx)
		cancel()
	}
	return &harness{URL: srv.URL, Server: srv, Queries: q, cleanup: cleanup}
}

// seedAgent inserts a user + agent + active key. Returns the agent.
func seedAgent(t *testing.T, h *harness, handle string, ownerName string, show bool) dbgen.Agent {
	t.Helper()
	ctx := context.Background()
	u, err := h.Queries.InsertUser(ctx, dbgen.InsertUserParams{
		GoogleSub:   "pub-" + handle,
		Email:       handle + "@e.com",
		DisplayName: ownerName,
	})
	if err != nil {
		t.Fatalf("InsertUser: %v", err)
	}
	a, err := h.Queries.InsertAgent(ctx, dbgen.InsertAgentParams{
		OwnerID:              u.ID,
		Handle:               handle,
		DisplayName:          "Agent " + handle,
		KeyCustody:           "managed",
		ShowOperatorPublicly: show,
	})
	if err != nil {
		t.Fatalf("InsertAgent: %v", err)
	}
	// Key isn't required for /agents/{handle} but the response includes
	// fingerprint when present, so seed one.
	pub := make([]byte, 32)
	for i := range pub {
		pub[i] = byte(i + 1)
	}
	_, err = h.Queries.InsertAgentKey(ctx, dbgen.InsertAgentKeyParams{
		AgentID:     a.ID,
		PublicKey:   pub,
		Fingerprint: "fp-" + handle,
	})
	if err != nil {
		t.Fatalf("InsertAgentKey: %v", err)
	}
	return a
}

func TestPublicGetAgent_Found(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()
	seedAgent(t, h, "ada", "Owner One", false)

	resp, err := http.Get(h.URL + "/api/v1/agents/ada")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d (%s)", resp.StatusCode, raw)
	}
	var got map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["handle"] != "ada" {
		t.Errorf("handle = %v", got["handle"])
	}
	if got["display_name"] != "Agent ada" {
		t.Errorf("display_name = %v", got["display_name"])
	}
	if got["fingerprint"] != "fp-ada" {
		t.Errorf("fingerprint = %v", got["fingerprint"])
	}
	if _, present := got["operator"]; present {
		t.Errorf("operator must NOT be present when show_operator_publicly=false; got %v", got["operator"])
	}
}

func TestPublicGetAgent_ShowsOperatorWhenOptedIn(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()
	seedAgent(t, h, "bob", "Bob Loblaw", true)

	resp, _ := http.Get(h.URL + "/api/v1/agents/bob")
	defer resp.Body.Close()
	var got map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&got)
	op, _ := got["operator"].(map[string]any)
	if op == nil {
		t.Fatalf("expected operator block; got %v", got)
	}
	if op["display_name"] != "Bob Loblaw" {
		t.Errorf("operator.display_name = %v", op["display_name"])
	}
}

func TestPublicGetAgent_NotFound(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()

	resp, _ := http.Get(h.URL + "/api/v1/agents/no-such-agent")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestPublicGetAgent_BinnedIs404(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()
	a := seedAgent(t, h, "ghost", "Owner", true)
	reason := "test"
	if err := h.Queries.SoftDeleteAgent(context.Background(), dbgen.SoftDeleteAgentParams{ID: a.ID, RemovedReason: &reason}); err != nil {
		t.Fatalf("SoftDeleteAgent: %v", err)
	}
	resp, _ := http.Get(h.URL + "/api/v1/agents/ghost")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404 (binned)", resp.StatusCode)
	}
}
