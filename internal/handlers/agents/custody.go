package agents

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/diarion/diarion-core/internal/agents/keys"
	"github.com/diarion/diarion-core/internal/db/dbgen"
	"github.com/jackc/pgx/v5"
)

type switchCustodyRequest struct {
	To string `json:"to"`
}

type switchCustodyResponse struct {
	agentResponse
	PrivateKey string `json:"private_key,omitempty"`
}

// SwitchCustody handles POST /api/v1/me/agents/{handle}/custody/switch.
//
// Rotates to a fresh keypair in the target custody mode. The body is
// `{"to":"self"}` or `{"to":"managed"}`. Switching to self returns the
// plaintext private key exactly once.
func (h *Handlers) SwitchCustody(w http.ResponseWriter, r *http.Request) {
	a, ok := h.loadOwnedAgent(w, r)
	if !ok {
		return
	}
	var req switchCustodyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	target := keys.Custody(req.To)
	if target != keys.CustodyManaged && target != keys.CustodySelf {
		http.Error(w, "to must be 'managed' or 'self'", http.StatusBadRequest)
		return
	}
	if string(target) == a.KeyCustody {
		http.Error(w, "agent is already in that custody mode", http.StatusBadRequest)
		return
	}

	issued, err := keys.Issue(target, h.MasterKey)
	if err != nil {
		slog.ErrorContext(r.Context(), "keys.Issue (switch)", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := h.Queries.RevokeAllActiveKeysForAgent(r.Context(), a.ID); err != nil {
		slog.ErrorContext(r.Context(), "RevokeAllActiveKeysForAgent (switch)", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if _, err := h.Queries.InsertAgentKey(r.Context(), dbgen.InsertAgentKeyParams{
		AgentID:             a.ID,
		PublicKey:           []byte(issued.PublicKey),
		Fingerprint:         issued.Fingerprint,
		EncryptedPrivateKey: issued.EncryptedPrivateKey,
	}); err != nil {
		slog.ErrorContext(r.Context(), "InsertAgentKey (switch)", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	updated, err := h.Queries.UpdateAgentCustody(r.Context(), dbgen.UpdateAgentCustodyParams{
		ID:         a.ID,
		KeyCustody: string(target),
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "UpdateAgentCustody", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if _, err := h.Queries.GetActiveKeyForAgent(r.Context(), updated.ID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		slog.WarnContext(r.Context(), "GetActiveKeyForAgent (switch)", "err", err, "agent_id", updated.ID)
	}

	resp := switchCustodyResponse{agentResponse: toResponse(&updated, issued.Fingerprint)}
	if target == keys.CustodySelf && issued.PlaintextPrivateKey != nil {
		resp.PrivateKey = hex.EncodeToString(issued.PlaintextPrivateKey)
		defer func() {
			for i := range issued.PlaintextPrivateKey {
				issued.PlaintextPrivateKey[i] = 0
			}
		}()
	}
	writeJSON(w, http.StatusOK, resp)
}
