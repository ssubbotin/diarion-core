//go:build integration

package agents_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/diarion/diarion-core/internal/auth/session"
	"github.com/diarion/diarion-core/internal/db"
	"github.com/diarion/diarion-core/internal/db/dbgen"
	"github.com/diarion/diarion-core/internal/handlers/agents"
	authmw "github.com/diarion/diarion-core/internal/middleware/auth"
	"github.com/go-chi/chi/v5"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

type harness struct {
	URL     string
	Queries *dbgen.Queries
	Cookie  *http.Cookie
	UserID  int64
	Master  []byte
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
	if _, err := rand.Read(master); err != nil {
		t.Fatalf("rand: %v", err)
	}

	u, _ := q.InsertUser(ctx, dbgen.InsertUserParams{
		GoogleSub: "agents-g-1", Email: "agents1@e.com", DisplayName: "Owner1",
	})
	mgr := session.NewManager(q, false)
	rec := httptest.NewRecorder()
	if _, err := mgr.Issue(ctx, rec, u.ID); err != nil {
		t.Fatalf("Issue: %v", err)
	}
	cookie := rec.Result().Cookies()[0]

	h := agents.New(q, master)
	r := chi.NewRouter()
	r.Use(authmw.Middleware(q, mgr))
	r.Route("/api/v1", func(r chi.Router) {
		h.Register(r)
	})

	srv := httptest.NewServer(r)
	cleanup := func() {
		srv.Close()
		pool.Close()
		_ = pgC.Terminate(ctx)
		cancel()
	}
	return &harness{URL: srv.URL, Queries: q, Cookie: cookie, UserID: u.ID, Master: master, cleanup: cleanup}
}

func doJSON(t *testing.T, method, url string, cookie *http.Cookie, body any) *http.Response {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, rdr)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	return resp
}

func TestCreate_Managed(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()

	body := map[string]any{
		"handle":       "ada-bot",
		"display_name": "Ada Bot",
		"bio":          "A diarist of Babbage machines.",
		"key_custody":  "managed",
	}
	resp := doJSON(t, http.MethodPost, h.URL+"/api/v1/me/agents", h.Cookie, body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d (%s)", resp.StatusCode, string(raw))
	}
	var got map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["handle"] != "ada-bot" {
		t.Errorf("handle = %v", got["handle"])
	}
	if got["key_custody"] != "managed" {
		t.Errorf("key_custody = %v, want managed", got["key_custody"])
	}
	if _, has := got["private_key"]; has {
		t.Errorf("managed-mode response must NOT include private_key")
	}
	if fp, _ := got["fingerprint"].(string); len(fp) != 64 {
		t.Errorf("fingerprint = %q, want 64 hex chars", fp)
	}
}

func TestCreate_Self_ReturnsPlaintextOnce(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()

	body := map[string]any{
		"handle":       "selfie-bot",
		"display_name": "Selfie Bot",
		"key_custody":  "self",
	}
	resp := doJSON(t, http.MethodPost, h.URL+"/api/v1/me/agents", h.Cookie, body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d (%s)", resp.StatusCode, string(raw))
	}
	var got map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	priv, _ := got["private_key"].(string)
	if priv == "" {
		t.Fatalf("self-mode create must return private_key plaintext")
	}
	decoded, err := hex.DecodeString(priv)
	if err != nil {
		t.Fatalf("hex.DecodeString: %v", err)
	}
	if len(decoded) != ed25519.PrivateKeySize {
		t.Errorf("private_key len = %d, want %d", len(decoded), ed25519.PrivateKeySize)
	}

	// Verify the returned privkey signs against the stored pubkey.
	ctx := context.Background()
	agent, _ := h.Queries.GetAgentByHandle(ctx, "selfie-bot")
	storedKey, _ := h.Queries.GetActiveKeyForAgent(ctx, agent.ID)
	if storedKey.EncryptedPrivateKey != nil {
		t.Errorf("self-mode agent must not have encrypted_private_key stored; got %d bytes", len(storedKey.EncryptedPrivateKey))
	}
	msg := []byte("hello")
	sig := ed25519.Sign(ed25519.PrivateKey(decoded), msg)
	if !ed25519.Verify(ed25519.PublicKey(storedKey.PublicKey), msg, sig) {
		t.Errorf("returned privkey does not match stored pubkey")
	}
}

func TestCreate_RejectsBadHandle(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()

	resp := doJSON(t, http.MethodPost, h.URL+"/api/v1/me/agents", h.Cookie, map[string]any{
		"handle":       "API", // uppercase + reserved
		"display_name": "Bad",
		"key_custody":  "managed",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestCreate_RejectsDuplicateHandle(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()

	body := map[string]any{
		"handle":       "twin-bot",
		"display_name": "Twin",
		"key_custody":  "managed",
	}
	resp1 := doJSON(t, http.MethodPost, h.URL+"/api/v1/me/agents", h.Cookie, body)
	resp1.Body.Close()
	if resp1.StatusCode != http.StatusCreated {
		t.Fatalf("first create status = %d", resp1.StatusCode)
	}
	resp2 := doJSON(t, http.MethodPost, h.URL+"/api/v1/me/agents", h.Cookie, body)
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusConflict {
		t.Errorf("duplicate create status = %d, want 409", resp2.StatusCode)
	}
}

func TestCreate_RequiresAuth(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()
	resp := doJSON(t, http.MethodPost, h.URL+"/api/v1/me/agents", nil, map[string]any{
		"handle":       "anon-bot",
		"display_name": "Anon",
		"key_custody":  "managed",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestList_OnlyOwn(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()
	ctx := context.Background()

	// Create one for our user via the API.
	resp := doJSON(t, http.MethodPost, h.URL+"/api/v1/me/agents", h.Cookie,
		map[string]any{"handle": "mine-1", "display_name": "Mine 1", "key_custody": "managed"})
	resp.Body.Close()
	resp2 := doJSON(t, http.MethodPost, h.URL+"/api/v1/me/agents", h.Cookie,
		map[string]any{"handle": "mine-2", "display_name": "Mine 2", "key_custody": "managed"})
	resp2.Body.Close()

	// And one for a different user, directly via the DB.
	other, _ := h.Queries.InsertUser(ctx, dbgen.InsertUserParams{
		GoogleSub: "other-g", Email: "other@e.com", DisplayName: "Other",
	})
	_, _ = h.Queries.InsertAgent(ctx, dbgen.InsertAgentParams{
		OwnerID: other.ID, Handle: "not-mine", DisplayName: "Not Mine", KeyCustody: "managed",
	})

	listResp := doJSON(t, http.MethodGet, h.URL+"/api/v1/me/agents", h.Cookie, nil)
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", listResp.StatusCode)
	}
	var items []map[string]any
	if err := json.NewDecoder(listResp.Body).Decode(&items); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("len(items) = %d, want 2", len(items))
	}
	for _, it := range items {
		if it["handle"] == "not-mine" {
			t.Errorf("list leaked another user's agent")
		}
	}
}

func TestGet_Owned(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()
	resp := doJSON(t, http.MethodPost, h.URL+"/api/v1/me/agents", h.Cookie,
		map[string]any{"handle": "get-me", "display_name": "Get Me", "key_custody": "managed"})
	resp.Body.Close()

	getResp := doJSON(t, http.MethodGet, h.URL+"/api/v1/me/agents/get-me", h.Cookie, nil)
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", getResp.StatusCode)
	}
	var got map[string]any
	_ = json.NewDecoder(getResp.Body).Decode(&got)
	if got["handle"] != "get-me" {
		t.Errorf("handle = %v", got["handle"])
	}
	if fp, _ := got["fingerprint"].(string); len(fp) != 64 {
		t.Errorf("fingerprint = %q", fp)
	}
}

func TestGet_NotOwned_Returns404(t *testing.T) {
	t.Parallel()
	h := setupHarness(t)
	defer h.Close()
	ctx := context.Background()

	other, _ := h.Queries.InsertUser(ctx, dbgen.InsertUserParams{
		GoogleSub: "other-g-2", Email: "other2@e.com", DisplayName: "Other2",
	})
	_, _ = h.Queries.InsertAgent(ctx, dbgen.InsertAgentParams{
		OwnerID: other.ID, Handle: "not-mine-2", DisplayName: "Not Mine 2", KeyCustody: "managed",
	})

	resp := doJSON(t, http.MethodGet, h.URL+"/api/v1/me/agents/not-mine-2", h.Cookie, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}
