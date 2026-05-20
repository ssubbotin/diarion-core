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
	// fingerprint when present, so seed one. Derive bytes from handle so
	// multiple agents in the same harness don't collide on the UNIQUE
	// public_key constraint.
	pub := make([]byte, 32)
	for i := range pub {
		pub[i] = byte(i + 1)
	}
	for i := 0; i < len(handle) && i < 32; i++ {
		pub[i] ^= handle[i]
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

func TestPublicListGlobalEntries_Basic(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()
	a := seedAgent(t, h, "globe1", "Owner", false)
	kid := keyIDForAgent(t, h, a.ID)
	for i := 0; i < 3; i++ {
		seedEntry(t, h, a.ID, kid, "g-"+string(rune('a'+i)), "G "+string(rune('A'+i)))
	}

	resp, err := http.Get(h.URL + "/api/v1/entries?limit=10")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d (%s)", resp.StatusCode, raw)
	}
	var got map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&got)
	items, _ := got["entries"].([]any)
	if len(items) != 3 {
		t.Errorf("len(entries) = %d, want 3", len(items))
	}
	first, _ := items[0].(map[string]any)
	if first["agent_handle"] != "globe1" {
		t.Errorf("agent_handle = %v", first["agent_handle"])
	}
}

func TestPublicListGlobalEntries_CursorPagination(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()
	a := seedAgent(t, h, "globe2", "Owner", false)
	kid := keyIDForAgent(t, h, a.ID)
	for i := 0; i < 5; i++ {
		seedEntry(t, h, a.ID, kid, "p-"+string(rune('a'+i)), "P "+string(rune('A'+i)))
	}

	// Page 1: limit 2.
	resp, _ := http.Get(h.URL + "/api/v1/entries?limit=2")
	defer resp.Body.Close()
	var page1 map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&page1)
	items1, _ := page1["entries"].([]any)
	if len(items1) != 2 {
		t.Fatalf("page1 len = %d", len(items1))
	}
	cursor1, _ := page1["next_cursor"].(string)
	if cursor1 == "" {
		t.Fatalf("page1 missing next_cursor")
	}

	// Page 2 via cursor.
	resp2, _ := http.Get(h.URL + "/api/v1/entries?limit=2&after=" + cursor1)
	defer resp2.Body.Close()
	var page2 map[string]any
	_ = json.NewDecoder(resp2.Body).Decode(&page2)
	items2, _ := page2["entries"].([]any)
	if len(items2) != 2 {
		t.Errorf("page2 len = %d", len(items2))
	}
	// Distinct items
	first1 := items1[0].(map[string]any)
	first2 := items2[0].(map[string]any)
	if first1["id"] == first2["id"] {
		t.Errorf("cursor failed to advance: same id on page1 and page2")
	}
}

func TestPublicListGlobalEntries_TagFilter(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()
	a := seedAgent(t, h, "globe3", "Owner", false)
	kid := keyIDForAgent(t, h, a.ID)

	// Two entries: one tagged "alpha", one untagged.
	sig := make([]byte, 64)
	hash := make([]byte, 32)
	_, _ = h.Queries.InsertEntry(context.Background(), dbgen.InsertEntryParams{
		AgentID: a.ID, SigningKeyID: kid, Slug: "tagged", Title: "Tagged",
		BodyMarkdown: "x", BodyHTML: "<p>x</p>",
		Tags: []string{"alpha", "beta"}, Frontmatter: []byte("{}"),
		Signature: sig, ContentHash: hash,
	})
	seedEntry(t, h, a.ID, kid, "plain", "Plain")

	resp, _ := http.Get(h.URL + "/api/v1/entries?tag=alpha")
	defer resp.Body.Close()
	var got map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&got)
	items, _ := got["entries"].([]any)
	if len(items) != 1 {
		t.Fatalf("len(entries) = %d, want 1 (only tagged)", len(items))
	}
	first := items[0].(map[string]any)
	if first["slug"] != "tagged" {
		t.Errorf("slug = %v, want tagged", first["slug"])
	}
}

func TestPublicListGlobalEntries_InvalidCursor_400(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()
	resp, _ := http.Get(h.URL + "/api/v1/entries?after=not-a-real-cursor")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestPublicListGlobalEntries_OmitsBinnedAgent(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()
	a := seedAgent(t, h, "ghost-feed", "Owner", false)
	kid := keyIDForAgent(t, h, a.ID)
	seedEntry(t, h, a.ID, kid, "doomed", "Doomed")
	reason := "t"
	_ = h.Queries.SoftDeleteAgent(context.Background(), dbgen.SoftDeleteAgentParams{ID: a.ID, RemovedReason: &reason})

	resp, _ := http.Get(h.URL + "/api/v1/entries?limit=50")
	defer resp.Body.Close()
	var got map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&got)
	items, _ := got["entries"].([]any)
	for _, it := range items {
		m := it.(map[string]any)
		if m["agent_handle"] == "ghost-feed" {
			t.Errorf("global feed leaked binned agent's entry")
		}
	}
}

func TestPublicSearch_BasicMatch(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()
	a := seedAgent(t, h, "search1", "Owner", false)
	kid := keyIDForAgent(t, h, a.ID)
	sig := make([]byte, 64)
	hash := make([]byte, 32)
	_, _ = h.Queries.InsertEntry(context.Background(), dbgen.InsertEntryParams{
		AgentID: a.ID, SigningKeyID: kid, Slug: "rust", Title: "Building with Rust",
		BodyMarkdown: "Rust ownership rules are subtle but powerful.",
		BodyHTML:     "<p>Rust ownership rules are subtle but powerful.</p>",
		Tags:         []string{"rust"}, Frontmatter: []byte("{}"),
		Signature: sig, ContentHash: hash,
	})
	_, _ = h.Queries.InsertEntry(context.Background(), dbgen.InsertEntryParams{
		AgentID: a.ID, SigningKeyID: kid, Slug: "go", Title: "Building with Go",
		BodyMarkdown: "Go interfaces are structural.",
		BodyHTML:     "<p>Go interfaces are structural.</p>",
		Tags:         []string{"go"}, Frontmatter: []byte("{}"),
		Signature: sig, ContentHash: hash,
	})

	resp, _ := http.Get(h.URL + "/api/v1/search?q=ownership")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d (%s)", resp.StatusCode, raw)
	}
	var got map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&got)
	items, _ := got["results"].([]any)
	if len(items) != 1 {
		t.Fatalf("len(results) = %d, want 1 (only Rust matches)", len(items))
	}
	first := items[0].(map[string]any)
	if first["slug"] != "rust" {
		t.Errorf("slug = %v, want rust", first["slug"])
	}
	if first["headline"] == nil || first["headline"] == "" {
		t.Errorf("headline missing")
	}
}

func TestPublicSearch_RequiresQuery(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()
	resp, _ := http.Get(h.URL + "/api/v1/search")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (missing q)", resp.StatusCode)
	}
}

func TestPublicSearch_AgentFilter(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()
	a1 := seedAgent(t, h, "search-a", "Owner1", false)
	a2 := seedAgent(t, h, "search-b", "Owner2", false)
	k1 := keyIDForAgent(t, h, a1.ID)
	k2 := keyIDForAgent(t, h, a2.ID)
	sig := make([]byte, 64)
	hash := make([]byte, 32)
	_, _ = h.Queries.InsertEntry(context.Background(), dbgen.InsertEntryParams{
		AgentID: a1.ID, SigningKeyID: k1, Slug: "x", Title: "shared topic",
		BodyMarkdown: "shared", BodyHTML: "<p>shared</p>", Tags: []string{},
		Frontmatter: []byte("{}"), Signature: sig, ContentHash: hash,
	})
	_, _ = h.Queries.InsertEntry(context.Background(), dbgen.InsertEntryParams{
		AgentID: a2.ID, SigningKeyID: k2, Slug: "y", Title: "shared again",
		BodyMarkdown: "shared", BodyHTML: "<p>shared</p>", Tags: []string{},
		Frontmatter: []byte("{}"), Signature: sig, ContentHash: hash,
	})

	resp, _ := http.Get(h.URL + "/api/v1/search?q=shared&agent=search-a")
	defer resp.Body.Close()
	var got map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&got)
	items, _ := got["results"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected 1 result (filtered); got %d", len(items))
	}
	first := items[0].(map[string]any)
	if first["agent_handle"] != "search-a" {
		t.Errorf("agent_handle = %v", first["agent_handle"])
	}
}

func TestPublicSearch_CursorPagination(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()
	a := seedAgent(t, h, "search-cur", "Owner", false)
	kid := keyIDForAgent(t, h, a.ID)
	sig := make([]byte, 64)
	hash := make([]byte, 32)
	for i := 0; i < 4; i++ {
		_, _ = h.Queries.InsertEntry(context.Background(), dbgen.InsertEntryParams{
			AgentID: a.ID, SigningKeyID: kid,
			Slug:         "doc-" + string(rune('a'+i)),
			Title:        "Document " + string(rune('A'+i)),
			BodyMarkdown: "common keyword in body " + string(rune('A'+i)),
			BodyHTML:     "<p>common keyword</p>",
			Tags:         []string{}, Frontmatter: []byte("{}"),
			Signature: sig, ContentHash: hash,
		})
	}

	resp, _ := http.Get(h.URL + "/api/v1/search?q=keyword&limit=2")
	defer resp.Body.Close()
	var p1 map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&p1)
	items1, _ := p1["results"].([]any)
	if len(items1) != 2 {
		t.Fatalf("page1 len = %d", len(items1))
	}
	cur, _ := p1["next_cursor"].(string)
	if cur == "" {
		t.Fatalf("missing next_cursor for paginated search")
	}

	resp2, _ := http.Get(h.URL + "/api/v1/search?q=keyword&limit=2&after=" + cur)
	defer resp2.Body.Close()
	var p2 map[string]any
	_ = json.NewDecoder(resp2.Body).Decode(&p2)
	items2, _ := p2["results"].([]any)
	if len(items2) == 0 {
		t.Errorf("page2 empty")
	}
	if items1[0].(map[string]any)["id"] == items2[0].(map[string]any)["id"] {
		t.Errorf("search cursor failed to advance")
	}
}
