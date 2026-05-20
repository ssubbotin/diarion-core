// Package purge runs an hourly background job that hard-deletes entries,
// agents, and users whose 30-day bin window has elapsed.
//
// Cascade order matters: entries → agents → users. RESTRICT FKs in the
// schema prevent skipping a layer.
package purge

import (
	"context"
	"log/slog"
	"time"

	"github.com/diarion/diarion-core/internal/db/dbgen"
)

// DefaultInterval is the production cadence (hourly).
const DefaultInterval = 1 * time.Hour

// Run loops on a ticker, invoking RunOnce each interval until ctx is done.
// Errors are logged but do not stop the loop.
func Run(ctx context.Context, q dbgen.Querier, interval time.Duration, logger *slog.Logger) {
	t := time.NewTicker(interval)
	defer t.Stop()
	logger.Info("purge worker started", "interval", interval.String())
	// Run once immediately so a fresh boot doesn't wait an hour.
	if err := RunOnce(ctx, q, logger); err != nil {
		logger.ErrorContext(ctx, "purge tick failed", "err", err)
	}
	for {
		select {
		case <-ctx.Done():
			logger.Info("purge worker stopping")
			return
		case <-t.C:
			if err := RunOnce(ctx, q, logger); err != nil {
				logger.ErrorContext(ctx, "purge tick failed", "err", err)
			}
		}
	}
}

// RunOnce executes a single purge pass.
func RunOnce(ctx context.Context, q dbgen.Querier, logger *slog.Logger) error {
	// 1. Entries belonging to about-to-purge agents.
	entriesFromAgents, err := q.HardDeleteEntriesForExpiredAgents(ctx)
	if err != nil {
		return err
	}
	// 2. Entries belonging to about-to-purge users (catches their non-binned agents too).
	entriesFromUsers, err := q.HardDeleteEntriesForExpiredUsers(ctx)
	if err != nil {
		return err
	}
	// 3. Standalone-expired entries.
	entriesDirect, err := q.PurgeExpiredEntries(ctx)
	if err != nil {
		return err
	}
	// 4. Agents belonging to about-to-purge users.
	agentsFromUsers, err := q.HardDeleteAgentsForExpiredUsers(ctx)
	if err != nil {
		return err
	}
	// 5. Standalone-expired agents.
	agentsDirect, err := q.PurgeExpiredAgents(ctx)
	if err != nil {
		return err
	}
	// 6. Expired users themselves (sessions + PATs cascade).
	users, err := q.PurgeExpiredUsers(ctx)
	if err != nil {
		return err
	}

	logger.InfoContext(ctx, "purge tick",
		"entries_via_agents", entriesFromAgents,
		"entries_via_users", entriesFromUsers,
		"entries_direct", entriesDirect,
		"agents_via_users", agentsFromUsers,
		"agents_direct", agentsDirect,
		"users", users,
	)
	return nil
}
