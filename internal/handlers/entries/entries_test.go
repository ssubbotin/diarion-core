//go:build integration

package entries_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/diarion/diarion-core/internal/auth/session"
	"github.com/diarion/diarion-core/internal/db"
	"github.com/diarion/diarion-core/internal/db/dbgen"
	"github.com/diarion/diarion-core/internal/handlers/agents"
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

	master := make([]byte, 32)
	_, _ = rand.Read(master)

	u, _ := q.InsertUser(ctx, dbgen.InsertUserParams{
		GoogleSub: "entries-g-1", Email: "e1@e.com", DisplayName: "E1",
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

	r := chi.NewRouter()
	r.Use(authmw.Middleware(q, mgr))
	r.Route("/api/v1", func(r chi.Router) {
		agentH.Register(r)
		entH.Register(r)
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

// createSelfAgent uses /me/agents Create to register a self-custody agent and
// returns its handle + plaintext private key + fingerprint.
func createSelfAgent(t *testing.T, h *harness, handleName string) (handle string, priv ed25519.PrivateKey, fingerprint string) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"handle":       handleName,
		"display_name": handleName,
		"key_custody":  "self",
	})
	req, _ := http.NewRequest(http.MethodPost, h.URL+"/api/v1/me/agents", bytes.NewReader(body))
	req.AddCookie(h.Cookie)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("create agent status = %d (%s)", resp.StatusCode, string(raw))
	}
	var got map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&got)
	hexPriv, _ := got["private_key"].(string)
	decoded, _ := hex.DecodeString(hexPriv)
	fp, _ := got["fingerprint"].(string)
	return handleName, ed25519.PrivateKey(decoded), fp
}

func signedPOST(t *testing.T, h *harness, handleName, fp string, priv ed25519.PrivateKey, prev []byte, body []byte) *http.Response {
	t.Helper()
	u, _ := url.Parse(h.URL + "/api/v1/agents/" + handleName + "/entries")
	req, _ := http.NewRequest(http.MethodPost, u.String(), bytes.NewReader(body))
	req.Host = u.Host
	req.Header.Set("Content-Type", "application/json")
	if err := signing.Sign(req, body, fp, priv, prev); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	return resp
}

func TestPostEntry_FirstEntry(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()

	handle, priv, fp := createSelfAgent(t, h, "ada-1")

	body := []byte(`{"title":"Hello world","body_markdown":"# Hi\n\nA paragraph.","tags":["greeting"]}`)
	prev := bytes.Repeat([]byte{0}, 32)

	resp := signedPOST(t, h, handle, fp, priv, prev, body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d (%s)", resp.StatusCode, string(raw))
	}
	var got map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&got)
	if got["slug"] != "hello-world" {
		t.Errorf("slug = %v, want hello-world", got["slug"])
	}
	if !strings.Contains(got["body_html"].(string), "<h1") {
		t.Errorf("body_html missing <h1>: %v", got["body_html"])
	}
	ch, _ := got["content_hash"].(string)
	expectedHash := sha256.Sum256(body)
	if ch != hex.EncodeToString(expectedHash[:]) {
		t.Errorf("content_hash mismatch; got %s, want %s", ch, hex.EncodeToString(expectedHash[:]))
	}
	prevHash, _ := got["prev_entry_hash"].(string)
	if prevHash != "" {
		t.Errorf("prev_entry_hash should be empty for first entry; got %q", prevHash)
	}
}

func TestPostEntry_ChainSecondEntry(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()

	handle, priv, fp := createSelfAgent(t, h, "ada-2")

	body1 := []byte(`{"title":"First","body_markdown":"x"}`)
	prev := bytes.Repeat([]byte{0}, 32)
	r1 := signedPOST(t, h, handle, fp, priv, prev, body1)
	defer r1.Body.Close()
	if r1.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(r1.Body)
		t.Fatalf("first status = %d (%s)", r1.StatusCode, string(raw))
	}
	var got1 map[string]any
	_ = json.NewDecoder(r1.Body).Decode(&got1)
	firstHash, _ := hex.DecodeString(got1["content_hash"].(string))

	body2 := []byte(`{"title":"Second","body_markdown":"y"}`)
	r2 := signedPOST(t, h, handle, fp, priv, firstHash, body2)
	defer r2.Body.Close()
	if r2.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(r2.Body)
		t.Fatalf("second status = %d (%s)", r2.StatusCode, string(raw))
	}
	var got2 map[string]any
	_ = json.NewDecoder(r2.Body).Decode(&got2)
	if got2["prev_entry_hash"] != got1["content_hash"] {
		t.Errorf("chain broken; second.prev=%v want %v", got2["prev_entry_hash"], got1["content_hash"])
	}
}

func TestPostEntry_ChainMismatch_Returns409(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()

	handle, priv, fp := createSelfAgent(t, h, "ada-3")

	body1 := []byte(`{"title":"First","body_markdown":"x"}`)
	zeros := bytes.Repeat([]byte{0}, 32)
	r1 := signedPOST(t, h, handle, fp, priv, zeros, body1)
	r1.Body.Close()

	bogus := bytes.Repeat([]byte{0xFF}, 32)
	r2 := signedPOST(t, h, handle, fp, priv, bogus, []byte(`{"title":"Bad","body_markdown":"y"}`))
	defer r2.Body.Close()
	if r2.StatusCode != http.StatusConflict {
		t.Errorf("status = %d, want 409", r2.StatusCode)
	}
}

func TestPostEntry_SlugCollisionGetsSuffix(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()

	handle, priv, fp := createSelfAgent(t, h, "ada-4")

	body1 := []byte(`{"title":"Hello","body_markdown":"x"}`)
	prev := bytes.Repeat([]byte{0}, 32)
	r1 := signedPOST(t, h, handle, fp, priv, prev, body1)
	var got1 map[string]any
	_ = json.NewDecoder(r1.Body).Decode(&got1)
	r1.Body.Close()
	if got1["slug"] != "hello" {
		t.Fatalf("first slug = %v", got1["slug"])
	}
	prev1, _ := hex.DecodeString(got1["content_hash"].(string))

	body2 := []byte(`{"title":"Hello","body_markdown":"y"}`)
	r2 := signedPOST(t, h, handle, fp, priv, prev1, body2)
	defer r2.Body.Close()
	if r2.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(r2.Body)
		t.Fatalf("second status = %d (%s)", r2.StatusCode, string(raw))
	}
	var got2 map[string]any
	_ = json.NewDecoder(r2.Body).Decode(&got2)
	if got2["slug"] != "hello-2" {
		t.Errorf("collision suffix failed; slug = %v", got2["slug"])
	}
}

func TestPostEntry_HandleMismatchInURL_Returns404(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()

	_, priv, fp := createSelfAgent(t, h, "ada-5")

	body := []byte(`{"title":"x","body_markdown":"x"}`)
	prev := bytes.Repeat([]byte{0}, 32)
	resp := signedPOST(t, h, "nonexistent-handle", fp, priv, prev, body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestPostEntry_KeyBelongsToDifferentAgent_Returns403(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()

	_, priv, fp := createSelfAgent(t, h, "ada-6")
	otherHandle, _, _ := createSelfAgent(t, h, "ada-7")

	body := []byte(`{"title":"x","body_markdown":"x"}`)
	prev := bytes.Repeat([]byte{0}, 32)
	resp := signedPOST(t, h, otherHandle, fp, priv, prev, body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403", resp.StatusCode)
	}
}

func TestPostEntry_StaleSignature_Returns401(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()

	handle, priv, fp := createSelfAgent(t, h, "ada-8")
	body := []byte(`{"title":"x","body_markdown":"x"}`)
	prev := bytes.Repeat([]byte{0}, 32)

	u, _ := url.Parse(h.URL + "/api/v1/agents/" + handle + "/entries")
	req, _ := http.NewRequest(http.MethodPost, u.String(), bytes.NewReader(body))
	req.Host = u.Host
	req.Header.Set("Content-Type", "application/json")
	if err := signing.SignAt(req, body, fp, priv, prev, time.Now().Add(-10*time.Minute)); err != nil {
		t.Fatalf("SignAt: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestPostEntry_TamperedBody_Returns400(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()

	handle, priv, fp := createSelfAgent(t, h, "ada-9")
	body := []byte(`{"title":"x","body_markdown":"x"}`)
	prev := bytes.Repeat([]byte{0}, 32)

	u, _ := url.Parse(h.URL + "/api/v1/agents/" + handle + "/entries")
	req, _ := http.NewRequest(http.MethodPost, u.String(), bytes.NewReader([]byte(`{"title":"DIFFERENT","body_markdown":"x"}`)))
	req.Host = u.Host
	req.Header.Set("Content-Type", "application/json")
	if err := signing.Sign(req, body, fp, priv, prev); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}
