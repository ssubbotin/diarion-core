package agents

import (
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/diarion/diarion-core/internal/agents/keys"
	"github.com/diarion/diarion-core/internal/db/dbgen"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type rotateKeyResponse struct {
	ID          int64  `json:"id"`
	Fingerprint string `json:"fingerprint"`
	Custody     string `json:"key_custody"`
	PrivateKey  string `json:"private_key,omitempty"`
}

// RotateKey handles POST /api/v1/me/agents/{handle}/keys.
//
// Always generates a fresh keypair in the agent's current custody mode and
// revokes any previously active keys. For self-custody, the plaintext private
// key is returned exactly once.
func (h *Handlers) RotateKey(w http.ResponseWriter, r *http.Request) {
	a, ok := h.loadOwnedAgent(w, r)
	if !ok {
		return
	}
	custody := keys.Custody(a.KeyCustody)
	issued, err := keys.Issue(custody, h.MasterKey)
	if err != nil {
		slog.ErrorContext(r.Context(), "keys.Issue (rotate)", "err", err, "agent_id", a.ID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := h.Queries.RevokeAllActiveKeysForAgent(r.Context(), a.ID); err != nil {
		slog.ErrorContext(r.Context(), "RevokeAllActiveKeysForAgent", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	key, err := h.Queries.InsertAgentKey(r.Context(), dbgen.InsertAgentKeyParams{
		AgentID:             a.ID,
		PublicKey:           []byte(issued.PublicKey),
		Fingerprint:         issued.Fingerprint,
		EncryptedPrivateKey: issued.EncryptedPrivateKey,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "InsertAgentKey (rotate)", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	resp := rotateKeyResponse{
		ID:          key.ID,
		Fingerprint: issued.Fingerprint,
		Custody:     a.KeyCustody,
	}
	if custody == keys.CustodySelf && issued.PlaintextPrivateKey != nil {
		resp.PrivateKey = hex.EncodeToString(issued.PlaintextPrivateKey)
		defer func() {
			for i := range issued.PlaintextPrivateKey {
				issued.PlaintextPrivateKey[i] = 0
			}
		}()
	}
	writeJSON(w, http.StatusCreated, resp)
}

// RevokeKey handles DELETE /api/v1/me/agents/{handle}/keys/{key_id}.
//
// Idempotent: revoking an already-revoked key returns 204 silently.
func (h *Handlers) RevokeKey(w http.ResponseWriter, r *http.Request) {
	a, ok := h.loadOwnedAgent(w, r)
	if !ok {
		return
	}
	idStr := chi.URLParam(r, "key_id")
	keyID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid key id", http.StatusBadRequest)
		return
	}

	// Load the key and verify it belongs to this agent.
	all, err := h.Queries.ListKeysForAgent(r.Context(), a.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "key not found", http.StatusNotFound)
			return
		}
		slog.ErrorContext(r.Context(), "ListKeysForAgent", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	var found bool
	for _, k := range all {
		if k.ID == keyID {
			found = true
			break
		}
	}
	if !found {
		http.Error(w, "key not found", http.StatusNotFound)
		return
	}

	if err := h.Queries.RevokeAgentKey(r.Context(), keyID); err != nil {
		slog.ErrorContext(r.Context(), "RevokeAgentKey", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
