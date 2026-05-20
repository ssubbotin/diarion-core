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
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

type harness struct {
	URL     string
	Server  *httptest.Server
	Queries *dbgen.Queries
	Pool    *pgxpool.Pool
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
	return &harness{URL: srv.URL, Server: srv, Queries: q, Pool: pool, cleanup: cleanup}
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

func TestPublicGetAgent_SuspendedIs404(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()
	a := seedAgent(t, h, "suspended", "Owner", true)
	// No sqlc query for suspension exists yet; UPDATE directly via the pool.
	_, err := h.Pool.Exec(context.Background(),
		`UPDATE agents SET suspended_at = NOW() WHERE id = $1`, a.ID)
	if err != nil {
		t.Fatalf("UPDATE suspended_at: %v", err)
	}
	resp, _ := http.Get(h.URL + "/api/v1/agents/suspended")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404 (suspended)", resp.StatusCode)
	}
}

func seedEntry(t *testing.T, h *harness, agentID, keyID int64, slug, title string) {
	t.Helper()
	sig := make([]byte, 64)
	hash := make([]byte, 32)
	tags := []string{}
	_, err := h.Queries.InsertEntry(context.Background(), dbgen.InsertEntryParams{
		AgentID:      agentID,
		SigningKeyID: keyID,
		Slug:         slug,
		Title:        title,
		BodyMarkdown: "# " + title,
		BodyHTML:     "<h1>" + title + "</h1>",
		Tags:         tags,
		Frontmatter:  []byte("{}"),
		Signature:    sig,
		ContentHash:  hash,
	})
	if err != nil {
		t.Fatalf("InsertEntry: %v", err)
	}
}

func keyIDForAgent(t *testing.T, h *harness, agentID int64) int64 {
	t.Helper()
	k, err := h.Queries.GetActiveKeyForAgent(context.Background(), agentID)
	if err != nil {
		t.Fatalf("GetActiveKeyForAgent: %v", err)
	}
	return k.ID
}

func TestPublicListEntries_Paginated(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()
	a := seedAgent(t, h, "carol", "Owner", false)
	kid := keyIDForAgent(t, h, a.ID)
	for i := 0; i < 5; i++ {
		seedEntry(t, h, a.ID, kid, "entry-"+string(rune('a'+i)), "Title "+string(rune('A'+i)))
	}

	resp, _ := http.Get(h.URL + "/api/v1/agents/carol/entries?limit=2")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d (%s)", resp.StatusCode, raw)
	}
	var got map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&got)
	items, _ := got["entries"].([]any)
	if len(items) != 2 {
		t.Errorf("len(entries) = %d, want 2", len(items))
	}
	if got["total"].(float64) != 5 {
		t.Errorf("total = %v, want 5", got["total"])
	}
}

func TestPublicListEntries_OmitsBinned(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()
	a := seedAgent(t, h, "dora", "Owner", false)
	kid := keyIDForAgent(t, h, a.ID)
	seedEntry(t, h, a.ID, kid, "live", "Live")
	seedEntry(t, h, a.ID, kid, "dead", "Dead")
	row, _ := h.Queries.GetEntryByAgentAndSlug(context.Background(), dbgen.GetEntryByAgentAndSlugParams{AgentID: a.ID, Slug: "dead"})
	reason := "t"
	_ = h.Queries.SoftDeleteEntry(context.Background(), dbgen.SoftDeleteEntryParams{ID: row.ID, RemovedReason: &reason})

	resp, _ := http.Get(h.URL + "/api/v1/agents/dora/entries")
	defer resp.Body.Close()
	var got map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&got)
	items, _ := got["entries"].([]any)
	if len(items) != 1 {
		t.Errorf("expected 1 entry (live only); got %d", len(items))
	}
	if total, _ := got["total"].(float64); total != 1 {
		t.Errorf("total = %v, want 1 (binned excluded from count)", total)
	}
	first, _ := items[0].(map[string]any)
	if first["slug"] != "live" {
		t.Errorf("slug = %v, want live", first["slug"])
	}
}

func TestPublicListEntries_UnknownAgent_404(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()
	resp, _ := http.Get(h.URL + "/api/v1/agents/missing/entries")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestPublicGetEntry_Found(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()
	a := seedAgent(t, h, "eve", "Owner", false)
	kid := keyIDForAgent(t, h, a.ID)
	seedEntry(t, h, a.ID, kid, "first-post", "First Post")

	resp, _ := http.Get(h.URL + "/api/v1/agents/eve/entries/first-post")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d (%s)", resp.StatusCode, raw)
	}
	var got map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&got)
	if got["slug"] != "first-post" {
		t.Errorf("slug = %v", got["slug"])
	}
	if got["title"] != "First Post" {
		t.Errorf("title = %v", got["title"])
	}
	if got["body_html"] == nil || got["body_html"] == "" {
		t.Errorf("body_html missing/empty")
	}
	if got["content_hash"] == "" {
		t.Errorf("content_hash empty")
	}
}

func TestPublicGetEntry_BinnedIs404(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()
	a := seedAgent(t, h, "fae", "Owner", false)
	kid := keyIDForAgent(t, h, a.ID)
	seedEntry(t, h, a.ID, kid, "doomed", "Doomed")
	row, _ := h.Queries.GetEntryByAgentAndSlug(context.Background(), dbgen.GetEntryByAgentAndSlugParams{AgentID: a.ID, Slug: "doomed"})
	reason := "t"
	_ = h.Queries.SoftDeleteEntry(context.Background(), dbgen.SoftDeleteEntryParams{ID: row.ID, RemovedReason: &reason})

	resp, _ := http.Get(h.URL + "/api/v1/agents/fae/entries/doomed")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestPublicGetEntry_UnknownSlug_404(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()
	seedAgent(t, h, "grace", "Owner", false)

	resp, _ := http.Get(h.URL + "/api/v1/agents/grace/entries/nope")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}
