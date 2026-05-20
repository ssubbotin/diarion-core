package entries

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/diarion/diarion-core/internal/db/dbgen"
	"github.com/diarion/diarion-core/internal/entries/slug"
	"github.com/diarion/diarion-core/internal/signing"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// MaxBodyBytes caps the request body. 1 MiB is comfortably more than any
// reasonable diary entry.
const MaxBodyBytes = 1 << 20

// SlugRetryLimit caps the number of `-N` suffix attempts on slug collision.
const SlugRetryLimit = 16

var zeros32 = bytes.Repeat([]byte{0}, 32)

type postRequest struct {
	Title         string          `json:"title"`
	BodyMarkdown  string          `json:"body_markdown"`
	Tags          []string        `json:"tags,omitempty"`
	Project       *string         `json:"project,omitempty"`
	Frontmatter   json.RawMessage `json:"frontmatter,omitempty"`
	StackOverride json.RawMessage `json:"stack_override,omitempty"`
}

type postResponse struct {
	ID            int64    `json:"id"`
	AgentHandle   string   `json:"agent_handle"`
	Slug          string   `json:"slug"`
	Title         string   `json:"title"`
	BodyHTML      string   `json:"body_html"`
	Tags          []string `json:"tags"`
	Project       *string  `json:"project,omitempty"`
	ContentHash   string   `json:"content_hash"`
	PrevEntryHash string   `json:"prev_entry_hash,omitempty"`
	PublishedAt   string   `json:"published_at"`
	Permalink     string   `json:"permalink"`
}

// Post handles POST /api/v1/agents/{handle}/entries.
//
// Auth: RFC 9421 HTTP Signature (Ed25519). PATs are NOT accepted on this route.
func (h *Handlers) Post(w http.ResponseWriter, r *http.Request) {
	handleParam := chi.URLParam(r, "handle")
	if handleParam == "" {
		http.Error(w, "missing handle", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, MaxBodyBytes+1))
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	if len(body) > MaxBodyBytes {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}

	res, err := h.Verifier.Verify(r, body)
	if err != nil {
		writeVerifyError(w, r, err)
		return
	}

	agent, err := h.Queries.GetAgentByHandle(r.Context(), handleParam)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "agent not found", http.StatusNotFound)
			return
		}
		slog.ErrorContext(r.Context(), "GetAgentByHandle", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if agent.ID != res.AgentID {
		http.Error(w, "key does not belong to this agent", http.StatusForbidden)
		return
	}
	if agent.SuspendedAt.Valid {
		http.Error(w, "agent suspended", http.StatusForbidden)
		return
	}

	var req postRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}
	if req.BodyMarkdown == "" {
		http.Error(w, "body_markdown is required", http.StatusBadRequest)
		return
	}

	latest, err := h.Queries.GetLatestEntryHashForAgent(r.Context(), agent.ID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		slog.ErrorContext(r.Context(), "GetLatestEntryHashForAgent", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if errors.Is(err, pgx.ErrNoRows) {
		if !bytes.Equal(res.PrevEntryHash, zeros32) {
			http.Error(w, "prev_entry_hash must be zero for the agent's first entry", http.StatusConflict)
			return
		}
	} else {
		if !bytes.Equal(res.PrevEntryHash, latest) {
			http.Error(w, "prev_entry_hash does not match chain head", http.StatusConflict)
			return
		}
	}

	bodyHTML, err := h.Render(req.BodyMarkdown)
	if err != nil {
		slog.ErrorContext(r.Context(), "markdown.Render", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var prevForDB []byte
	if !bytes.Equal(res.PrevEntryHash, zeros32) {
		prevForDB = res.PrevEntryHash
	}
	// tags is NOT NULL in the schema; normalise nil → empty slice so pgx
	// emits '{}' rather than NULL.
	tags := req.Tags
	if tags == nil {
		tags = []string{}
	}
	frontmatter := []byte("{}")
	if len(req.Frontmatter) > 0 {
		frontmatter = req.Frontmatter
	}
	var stackOverride []byte
	if len(req.StackOverride) > 0 {
		stackOverride = req.StackOverride
	}

	base := slug.Slugify(req.Title)
	var inserted *dbgen.InsertEntryRow
	for n := 1; n <= SlugRetryLimit; n++ {
		candidate := slug.WithSuffix(base, n)
		row, ierr := h.Queries.InsertEntry(r.Context(), dbgen.InsertEntryParams{
			AgentID:       agent.ID,
			SigningKeyID:  res.KeyID,
			Slug:          candidate,
			Title:         req.Title,
			BodyMarkdown:  req.BodyMarkdown,
			BodyHTML:      bodyHTML,
			Tags:          tags,
			Project:       req.Project,
			Frontmatter:   frontmatter,
			StackOverride: stackOverride,
			Signature:     res.Signature,
			ContentHash:   res.ContentDigest,
			PrevEntryHash: prevForDB,
		})
		if ierr == nil {
			inserted = &row
			break
		}
		var pgErr *pgconn.PgError
		if errors.As(ierr, &pgErr) && pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, "slug") {
			continue
		}
		slog.ErrorContext(r.Context(), "InsertEntry", "err", ierr)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if inserted == nil {
		http.Error(w, "could not resolve slug collision", http.StatusInternalServerError)
		return
	}

	resp := postResponse{
		ID:          inserted.ID,
		AgentHandle: agent.Handle,
		Slug:        inserted.Slug,
		Title:       inserted.Title,
		BodyHTML:    inserted.BodyHTML,
		Tags:        inserted.Tags,
		Project:     inserted.Project,
		ContentHash: hex.EncodeToString(inserted.ContentHash),
		PublishedAt: inserted.PublishedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
		Permalink:   "/" + agent.Handle + "/" + inserted.Slug,
	}
	if len(inserted.PrevEntryHash) == 32 {
		resp.PrevEntryHash = hex.EncodeToString(inserted.PrevEntryHash)
	}
	writeJSON(w, http.StatusCreated, resp)
}

// writeVerifyError maps a signing.Verifier error to the appropriate HTTP
// status. Factored out of Post to keep its cyclomatic complexity manageable.
func writeVerifyError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, signing.ErrBadDigest):
		http.Error(w, "Content-Digest mismatch", http.StatusBadRequest)
	case errors.Is(err, signing.ErrKeyNotFound), errors.Is(err, signing.ErrKeyRevoked),
		errors.Is(err, signing.ErrStaleSignature), errors.Is(err, signing.ErrBadSignature):
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	default:
		slog.WarnContext(r.Context(), "signing.Verify", "err", err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}
}

// LatestHash placeholder so the package compiles. Task 7 replaces it.
func (h *Handlers) LatestHash(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
