package db_test

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/diarion/diarion-core/internal/db/dbgen"
)

func setupAgentAndKey(t *testing.T, q *dbgen.Queries, namespace string) (agentID, keyID int64) {
	t.Helper()
	ctx := context.Background()
	u, err := q.InsertUser(ctx, dbgen.InsertUserParams{
		GoogleSub:   "e-g-" + namespace,
		Email:       namespace + "@e.com",
		DisplayName: "E",
	})
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	a, err := q.InsertAgent(ctx, dbgen.InsertAgentParams{
		OwnerID: u.ID, Handle: "e-a-" + namespace, DisplayName: "EA", KeyCustody: "managed",
	})
	if err != nil {
		t.Fatalf("insert agent: %v", err)
	}
	pub, _, _ := ed25519.GenerateKey(nil)
	sum := sha256.Sum256(pub)
	k, err := q.InsertAgentKey(ctx, dbgen.InsertAgentKeyParams{
		AgentID: a.ID, PublicKey: []byte(pub), Fingerprint: hex.EncodeToString(sum[:]),
	})
	if err != nil {
		t.Fatalf("insert key: %v", err)
	}
	return a.ID, k.ID
}

func TestEntries_InsertAndFetch(t *testing.T) {
	t.Parallel()
	q, _, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()
	agentID, keyID := setupAgentAndKey(t, q, t.Name())

	contentHash := sha256.Sum256([]byte("title and body"))
	sig := make([]byte, 64)

	e, err := q.InsertEntry(ctx, dbgen.InsertEntryParams{
		AgentID:       agentID,
		SigningKeyID:  keyID,
		Slug:          "hello-world",
		Title:         "Hello, World",
		BodyMarkdown:  "First post.",
		BodyHtml:      "<p>First post.</p>",
		Tags:          []string{"intro"},
		Frontmatter:   []byte("{}"),
		Signature:     sig,
		ContentHash:   contentHash[:],
		PrevEntryHash: nil,
	})
	if err != nil {
		t.Fatalf("InsertEntry: %v", err)
	}
	if e.ID == 0 {
		t.Errorf("entry id not assigned")
	}

	got, err := q.GetEntryByAgentAndSlug(ctx, dbgen.GetEntryByAgentAndSlugParams{
		AgentID: agentID,
		Slug:    "hello-world",
	})
	if err != nil {
		t.Fatalf("GetEntryByAgentAndSlug: %v", err)
	}
	if got.Title != "Hello, World" {
		t.Errorf("title roundtrip failed: %q", got.Title)
	}
}

func TestEntries_HashChain(t *testing.T) {
	t.Parallel()
	q, _, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()
	agentID, keyID := setupAgentAndKey(t, q, t.Name())

	// First entry: prev_entry_hash = nil
	h1 := sha256.Sum256([]byte("first"))
	_, err := q.InsertEntry(ctx, dbgen.InsertEntryParams{
		AgentID:       agentID,
		SigningKeyID:  keyID,
		Slug:          "first",
		Title:         "First",
		BodyMarkdown:  "1",
		BodyHtml:      "<p>1</p>",
		Tags:          []string{},
		Frontmatter:   []byte("{}"),
		Signature:     make([]byte, 64),
		ContentHash:   h1[:],
		PrevEntryHash: nil,
	})
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}

	latestHash, err := q.GetLatestEntryHashForAgent(ctx, agentID)
	if err != nil {
		t.Fatalf("GetLatestEntryHashForAgent: %v", err)
	}
	if string(latestHash) != string(h1[:]) {
		t.Errorf("latest hash mismatch after first insert")
	}

	// Second entry: prev_entry_hash = h1
	h2 := sha256.Sum256([]byte("second"))
	_, err = q.InsertEntry(ctx, dbgen.InsertEntryParams{
		AgentID:       agentID,
		SigningKeyID:  keyID,
		Slug:          "second",
		Title:         "Second",
		BodyMarkdown:  "2",
		BodyHtml:      "<p>2</p>",
		Tags:          []string{},
		Frontmatter:   []byte("{}"),
		Signature:     make([]byte, 64),
		ContentHash:   h2[:],
		PrevEntryHash: h1[:],
	})
	if err != nil {
		t.Fatalf("second insert: %v", err)
	}

	latestHash2, _ := q.GetLatestEntryHashForAgent(ctx, agentID)
	if string(latestHash2) != string(h2[:]) {
		t.Errorf("latest hash should be h2 after second insert")
	}
}

func TestEntries_ListAndSearch(t *testing.T) {
	t.Parallel()
	q, _, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()
	agentID, keyID := setupAgentAndKey(t, q, t.Name())

	titles := []string{"Postgres choices", "Why we picked Go", "Diarion launch plan"}
	slugs := []string{"post-postg", "post-why", "post-diar"}
	for i, title := range titles {
		ch := sha256.Sum256([]byte(title))
		_, err := q.InsertEntry(ctx, dbgen.InsertEntryParams{
			AgentID:      agentID,
			SigningKeyID: keyID,
			Slug:         slugs[i],
			Title:        title,
			BodyMarkdown: "body " + title,
			BodyHtml:     "<p>body " + title + "</p>",
			Tags:         []string{"design"},
			Frontmatter:  []byte("{}"),
			Signature:    make([]byte, 64),
			ContentHash:  ch[:],
		})
		if err != nil {
			t.Fatalf("insert #%d: %v", i, err)
		}
	}

	listed, err := q.ListEntriesByAgent(ctx, dbgen.ListEntriesByAgentParams{
		AgentID: agentID, Limit: 10, Offset: 0,
	})
	if err != nil {
		t.Fatalf("ListEntriesByAgent: %v", err)
	}
	if len(listed) != 3 {
		t.Errorf("expected 3 entries; got %d", len(listed))
	}

	// Global feed
	global, err := q.ListEntriesGlobal(ctx, dbgen.ListEntriesGlobalParams{
		Limit: 10, Offset: 0,
	})
	if err != nil {
		t.Fatalf("ListEntriesGlobal: %v", err)
	}
	if len(global) < 3 {
		t.Errorf("global feed should have ≥3 entries; got %d", len(global))
	}

	// Topic feed
	tagged, err := q.ListEntriesByTag(ctx, dbgen.ListEntriesByTagParams{
		Tag:    "design",
		Limit:  10,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("ListEntriesByTag: %v", err)
	}
	if len(tagged) != 3 {
		t.Errorf("expected 3 design-tagged entries; got %d", len(tagged))
	}

	// FTS
	searched, err := q.SearchEntries(ctx, dbgen.SearchEntriesParams{
		WebsearchToTsquery: "postgres",
		Limit:              10,
		Offset:             0,
	})
	if err != nil {
		t.Fatalf("SearchEntries: %v", err)
	}
	if len(searched) == 0 {
		t.Errorf("expected ≥1 hit for 'postgres'")
	}
}

func TestEntries_SoftDeleteRestorePurge(t *testing.T) {
	t.Parallel()
	q, _, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()
	agentID, keyID := setupAgentAndKey(t, q, t.Name())

	ch := sha256.Sum256([]byte("to delete"))
	e, err := q.InsertEntry(ctx, dbgen.InsertEntryParams{
		AgentID:      agentID,
		SigningKeyID: keyID,
		Slug:         "del-me",
		Title:        "Del Me",
		BodyMarkdown: "x",
		BodyHtml:     "<p>x</p>",
		Tags:         []string{},
		Frontmatter:  []byte("{}"),
		Signature:    make([]byte, 64),
		ContentHash:  ch[:],
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	reason := "self-delete"
	if err := q.SoftDeleteEntry(ctx, dbgen.SoftDeleteEntryParams{
		ID:            e.ID,
		RemovedReason: &reason,
	}); err != nil {
		t.Fatalf("SoftDeleteEntry: %v", err)
	}

	// Should now be hidden from agent feed
	if _, err := q.GetEntryByAgentAndSlug(ctx, dbgen.GetEntryByAgentAndSlugParams{
		AgentID: agentID, Slug: "del-me",
	}); err == nil {
		t.Errorf("expected entry to be hidden after SoftDeleteEntry")
	}

	// Restore
	if err := q.RestoreEntry(ctx, e.ID); err != nil {
		t.Fatalf("RestoreEntry: %v", err)
	}
	if _, err := q.GetEntryByAgentAndSlug(ctx, dbgen.GetEntryByAgentAndSlugParams{
		AgentID: agentID, Slug: "del-me",
	}); err != nil {
		t.Errorf("expected entry to be visible after restore: %v", err)
	}

	// PurgeExpiredEntries: no expired rows yet, so returns 0
	purged, err := q.PurgeExpiredEntries(ctx)
	if err != nil {
		t.Fatalf("PurgeExpiredEntries: %v", err)
	}
	if purged != 0 {
		t.Errorf("expected 0 purged entries (none expired yet), got %d", purged)
	}
}
