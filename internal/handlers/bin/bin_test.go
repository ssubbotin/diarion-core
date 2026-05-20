//go:build integration

package bin_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/diarion/diarion-core/internal/auth/session"
	"github.com/diarion/diarion-core/internal/db"
	"github.com/diarion/diarion-core/internal/db/dbgen"
	"github.com/diarion/diarion-core/internal/handlers/agents"
	binh "github.com/diarion/diarion-core/internal/handlers/bin"
	"github.com/diarion/diarion-core/internal/handlers/entries"
	"github.com/diarion/diarion-core/internal/markdown"
	authmw "github.com/diarion/diarion-core/internal/middleware/auth"
	"github.com/diarion/diarion-core/internal/signing"
	"github.com/go-chi/chi/v5"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

type harness struct {
	URL     string
	Server  *httptest.Server
	Queries *dbgen.Queries
	Cookie  *http.Cookie
	UserID  int64
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

	master := bytes.Repeat([]byte{0x42}, 32)

	u, _ := q.InsertUser(ctx, dbgen.InsertUserParams{
		GoogleSub: "bin-g-1", Email: "bin1@e.com", DisplayName: "Bin User",
	})
	mgr := session.NewManager(q, false)
	rec := httptest.NewRecorder()
	if _, err := mgr.Issue(ctx, rec, u.ID); err != nil {
		t.Fatalf("Issue: %v", err)
	}
	cookie := rec.Result().Cookies()[0]

	verifier := signing.NewVerifier(signing.NewDBKeyFetcher(q))
	agentH := agents.New(q, master)
	entH := entries.New(q, verifier, markdown.Render)
	binHandlers := binh.New(q)

	r := chi.NewRouter()
	r.Use(authmw.Middleware(q, mgr))
	r.Route("/api/v1", func(r chi.Router) {
		agentH.Register(r)
		entH.Register(r)
		binHandlers.Register(r)
	})
	srv := httptest.NewServer(r)
	cleanup := func() {
		srv.Close()
		pool.Close()
		_ = pgC.Terminate(ctx)
		cancel()
	}
	return &harness{URL: srv.URL, Server: srv, Queries: q, Cookie: cookie, UserID: u.ID, cleanup: cleanup}
}

// createSelfAgent uses /me/agents to register and returns plaintext key + fp.
func createSelfAgent(t *testing.T, h *harness, handleName string) (string, []byte, string) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"handle": handleName, "display_name": handleName, "key_custody": "self",
	})
	req, _ := http.NewRequest(http.MethodPost, h.URL+"/api/v1/me/agents", bytes.NewReader(body))
	req.AddCookie(h.Cookie)
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("create agent = %d (%s)", resp.StatusCode, raw)
	}
	var got map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&got)
	hexPriv, _ := got["private_key"].(string)
	decoded, _ := hex.DecodeString(hexPriv)
	fp, _ := got["fingerprint"].(string)
	return handleName, decoded, fp
}

// publishAndDelete publishes one entry then deletes it via the session DELETE.
// Returns the entry ID + slug. Used to populate the bin.
func publishAndDelete(t *testing.T, h *harness, handle, fp string, priv []byte, prev []byte) (int64, string) {
	t.Helper()
	body := []byte(`{"title":"t","body_markdown":"# t"}`)
	u, _ := url.Parse(h.URL + "/api/v1/agents/" + handle + "/entries")
	req, _ := http.NewRequest(http.MethodPost, u.String(), bytes.NewReader(body))
	req.Host = u.Host
	req.Header.Set("Content-Type", "application/json")
	if err := signing.Sign(req, body, fp, priv, prev); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST = %d (%s)", resp.StatusCode, raw)
	}
	var p map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&p)
	id := int64(p["id"].(float64))
	slug, _ := p["slug"].(string)

	req, _ = http.NewRequest(http.MethodDelete, h.URL+"/api/v1/agents/"+handle+"/entries/"+slug, nil)
	req.AddCookie(h.Cookie)
	dresp, _ := http.DefaultClient.Do(req)
	dresp.Body.Close()
	if dresp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE = %d", dresp.StatusCode)
	}
	return id, slug
}

func TestBinSummary_Empty(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()
	req, _ := http.NewRequest(http.MethodGet, h.URL+"/api/v1/me/bin", nil)
	req.AddCookie(h.Cookie)
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var got map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&got)
	if got["entries_count"].(float64) != 0 || got["agents_count"].(float64) != 0 {
		t.Errorf("expected empty bin; got %v", got)
	}
}

func TestBinSummary_WithBinnedEntry(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()
	handle, priv, fp := createSelfAgent(t, h, "binsum1")
	_, _ = publishAndDelete(t, h, handle, fp, priv, bytes.Repeat([]byte{0}, 32))

	req, _ := http.NewRequest(http.MethodGet, h.URL+"/api/v1/me/bin", nil)
	req.AddCookie(h.Cookie)
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	var got map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&got)
	if got["entries_count"].(float64) != 1 {
		t.Errorf("entries_count = %v, want 1", got["entries_count"])
	}
	if got["bytes_total"].(float64) <= 0 {
		t.Errorf("bytes_total = %v, want > 0", got["bytes_total"])
	}
}

func TestBinSummary_RequiresAuth(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()
	resp, _ := http.Get(h.URL + "/api/v1/me/bin")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestBinListEntries_ShowsDeleted(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()
	handle, priv, fp := createSelfAgent(t, h, "binl1")
	id1, slug1 := publishAndDelete(t, h, handle, fp, priv, bytes.Repeat([]byte{0}, 32))

	req, _ := http.NewRequest(http.MethodGet, h.URL+"/api/v1/me/bin/entries", nil)
	req.AddCookie(h.Cookie)
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d (%s)", resp.StatusCode, raw)
	}
	var got map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&got)
	items, _ := got["entries"].([]any)
	if len(items) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(items))
	}
	first := items[0].(map[string]any)
	if int64(first["id"].(float64)) != id1 {
		t.Errorf("id mismatch")
	}
	if first["slug"] != slug1 {
		t.Errorf("slug mismatch")
	}
	if first["agent_handle"] != handle {
		t.Errorf("agent_handle = %v", first["agent_handle"])
	}
	if first["hard_delete_at"] == nil {
		t.Errorf("hard_delete_at missing")
	}
}

func TestBinListAgents_ShowsDeleted(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()
	handle, _, _ := createSelfAgent(t, h, "agent-binl")

	// Delete the agent via the /me/agents endpoint (session).
	req, _ := http.NewRequest(http.MethodDelete, h.URL+"/api/v1/me/agents/"+handle, nil)
	req.AddCookie(h.Cookie)
	dresp, _ := http.DefaultClient.Do(req)
	dresp.Body.Close()
	if dresp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE agent = %d", dresp.StatusCode)
	}

	req, _ = http.NewRequest(http.MethodGet, h.URL+"/api/v1/me/bin/agents", nil)
	req.AddCookie(h.Cookie)
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var got map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&got)
	items, _ := got["agents"].([]any)
	if len(items) != 1 {
		t.Fatalf("len(agents) = %d, want 1", len(items))
	}
}

func TestBinListEntries_DoesNotLeakOtherUsers(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()
	handle, priv, fp := createSelfAgent(t, h, "binl2")
	_, _ = publishAndDelete(t, h, handle, fp, priv, bytes.Repeat([]byte{0}, 32))

	// Second user shouldn't see the first's binned entries.
	ctx := context.Background()
	u2, _ := h.Queries.InsertUser(ctx, dbgen.InsertUserParams{GoogleSub: "u2", Email: "u2@e.com", DisplayName: "U2"})
	mgr := session.NewManager(h.Queries, false)
	rec := httptest.NewRecorder()
	_, _ = mgr.Issue(ctx, rec, u2.ID)
	c2 := rec.Result().Cookies()[0]

	req, _ := http.NewRequest(http.MethodGet, h.URL+"/api/v1/me/bin/entries", nil)
	req.AddCookie(c2)
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	var got map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&got)
	items, _ := got["entries"].([]any)
	if len(items) != 0 {
		t.Errorf("second user must not see first's bin; got %d items", len(items))
	}
}
