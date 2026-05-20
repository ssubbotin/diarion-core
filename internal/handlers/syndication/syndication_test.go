//go:build integration

package syndication_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/diarion/diarion-core/internal/db"
	"github.com/diarion/diarion-core/internal/db/dbgen"
	"github.com/diarion/diarion-core/internal/handlers/syndication"
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
	syndication.New(q, "https://diarion.test").Register(r)
	srv := httptest.NewServer(r)
	cleanup := func() {
		srv.Close()
		pool.Close()
		_ = pgC.Terminate(ctx)
		cancel()
	}
	return &harness{URL: srv.URL, Server: srv, Queries: q, cleanup: cleanup}
}

func seedAgentWithEntry(t *testing.T, h *harness, handle, slug, title string) {
	t.Helper()
	ctx := context.Background()
	u, _ := h.Queries.InsertUser(ctx, dbgen.InsertUserParams{
		GoogleSub: "synd-" + handle, Email: handle + "@e.com", DisplayName: "Owner " + handle,
	})
	a, _ := h.Queries.InsertAgent(ctx, dbgen.InsertAgentParams{
		OwnerID: u.ID, Handle: handle, DisplayName: "Agent " + handle, KeyCustody: "managed",
	})
	pub := make([]byte, 32)
	for i := range pub {
		pub[i] = byte(i + 1)
	}
	// XOR per-handle bytes so the UNIQUE agent_keys.public_key constraint
	// doesn't collide across seeded agents in the same harness.
	for i := 0; i < len(handle) && i < len(pub); i++ {
		pub[i] ^= handle[i]
	}
	k, _ := h.Queries.InsertAgentKey(ctx, dbgen.InsertAgentKeyParams{
		AgentID: a.ID, PublicKey: pub, Fingerprint: "fp-" + handle,
	})
	sig := make([]byte, 64)
	hash := make([]byte, 32)
	_, _ = h.Queries.InsertEntry(ctx, dbgen.InsertEntryParams{
		AgentID: a.ID, SigningKeyID: k.ID, Slug: slug, Title: title,
		BodyMarkdown: "x", BodyHTML: "<p>x</p>",
		Tags: []string{"tag-a"}, Frontmatter: []byte("{}"),
		Signature: sig, ContentHash: hash,
	})
}

func TestAgentFeed_RSS(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()
	seedAgentWithEntry(t, h, "rss-agent", "post-1", "Post One")

	resp, _ := http.Get(h.URL + "/rss-agent/feed.xml")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d (%s)", resp.StatusCode, raw)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/rss+xml") {
		t.Errorf("Content-Type = %q", ct)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "public, max-age=300" {
		t.Errorf("Cache-Control = %q", cc)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "<rss") {
		t.Errorf("body lacks <rss>: %s", body)
	}
	if !strings.Contains(string(body), "Post One") {
		t.Errorf("body lacks entry title")
	}
}

func TestAgentFeed_Atom(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()
	seedAgentWithEntry(t, h, "atom-agent", "post-1", "Atom Post")

	resp, _ := http.Get(h.URL + "/atom-agent/feed.atom")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/atom+xml") {
		t.Errorf("Content-Type = %q", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "<feed xmlns=") {
		t.Errorf("body lacks atom feed root")
	}
}

func TestAgentFeed_JSON(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()
	seedAgentWithEntry(t, h, "json-agent", "post-1", "JSON Post")

	resp, _ := http.Get(h.URL + "/json-agent/feed.json")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/feed+json") {
		t.Errorf("Content-Type = %q", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("invalid JSON Feed: %v", err)
	}
	if got["version"] == nil {
		t.Errorf("missing JSON Feed version")
	}
}

func TestAgentFeed_UnknownAgent_404(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()

	resp, _ := http.Get(h.URL + "/missing-agent/feed.xml")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestAgentFeed_BinnedAgent_404(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()
	seedAgentWithEntry(t, h, "binned-feed", "post-1", "Bin Post")
	ctx := context.Background()
	a, _ := h.Queries.GetAgentByHandle(ctx, "binned-feed")
	reason := "test"
	_ = h.Queries.SoftDeleteAgent(ctx, dbgen.SoftDeleteAgentParams{ID: a.ID, RemovedReason: &reason})

	resp, _ := http.Get(h.URL + "/binned-feed/feed.xml")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}
