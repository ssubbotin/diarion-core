//go:build integration

package me_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/diarion/diarion-core/internal/auth/session"
	"github.com/diarion/diarion-core/internal/db"
	"github.com/diarion/diarion-core/internal/db/dbgen"
	"github.com/diarion/diarion-core/internal/handlers/me"
	authmw "github.com/diarion/diarion-core/internal/middleware/auth"
	"github.com/go-chi/chi/v5"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func setupServer(t *testing.T) (string, *dbgen.Queries, func(), *http.Cookie, int64) {
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

	u, _ := q.InsertUser(ctx, dbgen.InsertUserParams{
		GoogleSub: "me-g-1", Email: "me1@e.com", DisplayName: "Me1",
	})

	mgr := session.NewManager(q, false)

	r := chi.NewRouter()
	r.Use(authmw.Middleware(q, mgr))
	h := me.New(q)
	r.Get("/api/v1/me", h.Get)
	r.Post("/api/v1/me/tokens", h.CreateToken)
	r.Get("/api/v1/me/tokens", h.ListTokens)
	r.Delete("/api/v1/me/tokens/{id}", h.RevokeToken)

	srv := httptest.NewServer(r)

	w := httptest.NewRecorder()
	if _, err := mgr.Issue(ctx, w, u.ID); err != nil {
		t.Fatalf("Issue: %v", err)
	}
	cookies := w.Result().Cookies()
	cookie := cookies[0]

	cleanup := func() {
		srv.Close()
		pool.Close()
		_ = pgC.Terminate(ctx)
		cancel()
	}
	return srv.URL, q, cleanup, cookie, u.ID
}

func doReq(t *testing.T, method, urlStr string, cookie *http.Cookie, body any) *http.Response {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, urlStr, rdr)
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

func TestGetMe_AuthRequired(t *testing.T) {
	t.Parallel()
	urlBase, _, cleanup, _, _ := setupServer(t)
	defer cleanup()

	resp := doReq(t, "GET", urlBase+"/api/v1/me", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestGetMe_Authenticated(t *testing.T) {
	t.Parallel()
	urlBase, _, cleanup, cookie, _ := setupServer(t)
	defer cleanup()

	resp := doReq(t, "GET", urlBase+"/api/v1/me", cookie, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["email"] != "me1@e.com" {
		t.Errorf("email = %v", got["email"])
	}
	if got["via_pat"] != false {
		t.Errorf("via_pat = %v, want false", got["via_pat"])
	}
}

func TestTokens_CreateListRevoke(t *testing.T) {
	t.Parallel()
	urlBase, _, cleanup, cookie, _ := setupServer(t)
	defer cleanup()

	createResp := doReq(t, "POST", urlBase+"/api/v1/me/tokens", cookie, map[string]string{"name": "claude-desktop"})
	defer createResp.Body.Close()
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d", createResp.StatusCode)
	}
	var created map[string]any
	_ = json.NewDecoder(createResp.Body).Decode(&created)
	plain, _ := created["token"].(string)
	if !strings.HasPrefix(plain, "diarion_pat_") {
		t.Errorf("token = %q, missing prefix", plain)
	}
	idF, _ := created["id"].(float64)
	id := int64(idF)

	listResp := doReq(t, "GET", urlBase+"/api/v1/me/tokens", cookie, nil)
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d", listResp.StatusCode)
	}
	var listed []map[string]any
	_ = json.NewDecoder(listResp.Body).Decode(&listed)
	if len(listed) != 1 || listed[0]["name"] != "claude-desktop" {
		t.Errorf("list mismatch: %+v", listed)
	}
	for _, item := range listed {
		if _, has := item["token"]; has {
			t.Errorf("plaintext token leaked in list response")
		}
	}

	revResp := doReq(t, "DELETE", urlBase+"/api/v1/me/tokens/"+strconv.FormatInt(id, 10), cookie, nil)
	defer revResp.Body.Close()
	if revResp.StatusCode != http.StatusNoContent {
		t.Errorf("revoke status = %d", revResp.StatusCode)
	}

	listResp2 := doReq(t, "GET", urlBase+"/api/v1/me/tokens", cookie, nil)
	defer listResp2.Body.Close()
	var listed2 []map[string]any
	_ = json.NewDecoder(listResp2.Body).Decode(&listed2)
	if len(listed2) != 0 {
		t.Errorf("after revoke, list should be empty; got %d", len(listed2))
	}
}

func TestCreateToken_PATCannotMintTokens(t *testing.T) {
	t.Parallel()
	urlBase, q, cleanup, cookie, userID := setupServer(t)
	defer cleanup()

	createResp := doReq(t, "POST", urlBase+"/api/v1/me/tokens", cookie, map[string]string{"name": "first-pat"})
	defer createResp.Body.Close()
	var created map[string]any
	_ = json.NewDecoder(createResp.Body).Decode(&created)
	plain := created["token"].(string)

	req, _ := http.NewRequest("POST", urlBase+"/api/v1/me/tokens",
		bytes.NewBufferString(`{"name":"forbidden"}`))
	req.Header.Set("Authorization", "Bearer "+plain)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("PAT-minting-PAT status = %d, want 403", resp.StatusCode)
	}

	tokens, _ := q.ListPATsForUser(context.Background(), userID)
	if len(tokens) != 1 {
		t.Errorf("expected 1 PAT after forbidden mint attempt, got %d", len(tokens))
	}
}
