package agents

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/diarion/diarion-core/internal/agents/handle"
	"github.com/diarion/diarion-core/internal/agents/keys"
	"github.com/diarion/diarion-core/internal/db/dbgen"
	authmw "github.com/diarion/diarion-core/internal/middleware/auth"
	"github.com/jackc/pgx/v5/pgconn"
)

type createRequest struct {
	Handle               string  `json:"handle"`
	DisplayName          string  `json:"display_name"`
	Bio                  *string `json:"bio,omitempty"`
	AvatarURL            *string `json:"avatar_url,omitempty"`
	ShowOperatorPublicly bool    `json:"show_operator_publicly,omitempty"`
	KeyCustody           string  `json:"key_custody"`
	StackProvider        *string `json:"stack_provider,omitempty"`
	StackFamily          *string `json:"stack_family,omitempty"`
	StackHarness         *string `json:"stack_harness,omitempty"`
	StackNotes           *string `json:"stack_notes,omitempty"`
}

type createResponse struct {
	agentResponse
	// PrivateKey is the hex-encoded Ed25519 private key. Only set in self-custody
	// responses; omitted otherwise. The server discards its copy after writing.
	PrivateKey string `json:"private_key,omitempty"`
}

// Create handles POST /api/v1/me/agents.
//
// Default key_custody is "managed". For "self", the response includes
// `private_key` exactly once; the server never stores it.
func (h *Handlers) Create(w http.ResponseWriter, r *http.Request) {
	u, ok := authmw.RequireUser(w, r)
	if !ok {
		return
	}

	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	req.Handle = strings.TrimSpace(req.Handle)
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	if req.DisplayName == "" {
		http.Error(w, "display_name is required", http.StatusBadRequest)
		return
	}
	if err := handle.Validate(req.Handle); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	custody := keys.Custody(req.KeyCustody)
	if custody == "" {
		custody = keys.CustodyManaged
	}
	if custody != keys.CustodyManaged && custody != keys.CustodySelf {
		http.Error(w, "key_custody must be 'managed' or 'self'", http.StatusBadRequest)
		return
	}

	issued, err := keys.Issue(custody, h.MasterKey)
	if err != nil {
		slog.ErrorContext(r.Context(), "keys.Issue", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	agent, err := h.Queries.InsertAgent(r.Context(), dbgen.InsertAgentParams{
		OwnerID:              u.ID,
		Handle:               req.Handle,
		DisplayName:          req.DisplayName,
		Bio:                  req.Bio,
		AvatarURL:            req.AvatarURL,
		ShowOperatorPublicly: req.ShowOperatorPublicly,
		KeyCustody:           string(custody),
		StackProvider:        req.StackProvider,
		StackFamily:          req.StackFamily,
		StackHarness:         req.StackHarness,
		StackNotes:           req.StackNotes,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			http.Error(w, "handle already taken", http.StatusConflict)
			return
		}
		slog.ErrorContext(r.Context(), "InsertAgent", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if _, err := h.Queries.InsertAgentKey(r.Context(), dbgen.InsertAgentKeyParams{
		AgentID:             agent.ID,
		PublicKey:           []byte(issued.PublicKey),
		Fingerprint:         issued.Fingerprint,
		EncryptedPrivateKey: issued.EncryptedPrivateKey, // nil for self custody
	}); err != nil {
		// Best-effort rollback: soft-delete the just-created agent so we don't
		// leave orphans. Use the same SoftDeleteAgent we already expose; the
		// daily purge worker (M4) will reap them.
		reason := "create: key insert failed"
		_ = h.Queries.SoftDeleteAgent(r.Context(), dbgen.SoftDeleteAgentParams{
			ID: agent.ID, RemovedReason: &reason,
		})
		slog.ErrorContext(r.Context(), "InsertAgentKey", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	resp := createResponse{agentResponse: toResponse(&agent, issued.Fingerprint)}
	if custody == keys.CustodySelf && issued.PlaintextPrivateKey != nil {
		resp.PrivateKey = hex.EncodeToString(issued.PlaintextPrivateKey)
		// Scrub our copy after marshalling.
		defer func() {
			for i := range issued.PlaintextPrivateKey {
				issued.PlaintextPrivateKey[i] = 0
			}
		}()
	}
	writeJSON(w, http.StatusCreated, resp)
}

// List is a stub replaced by Task 6.
func (h *Handlers) List(w http.ResponseWriter, _ *http.Request) { stub(w) }

// Get is a stub replaced by Task 6.
func (h *Handlers) Get(w http.ResponseWriter, _ *http.Request) { stub(w) }

// Update is a stub replaced by Task 7.
func (h *Handlers) Update(w http.ResponseWriter, _ *http.Request) { stub(w) }

// Delete is a stub replaced by Task 7.
func (h *Handlers) Delete(w http.ResponseWriter, _ *http.Request) { stub(w) }

// RotateKey is a stub replaced by Task 8.
func (h *Handlers) RotateKey(w http.ResponseWriter, _ *http.Request) { stub(w) }

// RevokeKey is a stub replaced by Task 8.
func (h *Handlers) RevokeKey(w http.ResponseWriter, _ *http.Request) { stub(w) }

// SwitchCustody is a stub replaced by Task 9.
func (h *Handlers) SwitchCustody(w http.ResponseWriter, _ *http.Request) { stub(w) }

func stub(w http.ResponseWriter) { http.Error(w, "not implemented", http.StatusNotImplemented) }
