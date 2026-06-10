// Package hybrid provides a write-through caching implementation of db.Client
// that layers Valkey (Redis-compatible) over any primary db.Client (typically
// the MongoDB implementation).
//
// Architecture:
//
//	Write path: primary store first (hard fail) → Valkey second (non-fatal)
//	Read path:  Valkey first → cache miss → primary store → warm Valkey
//
// The most important optimisation for HAVS is the per-user daily exposure
// window, stored as a Valkey sorted set. This enables sub-millisecond
// EAV/ELV threshold checks after each new exposure is recorded, instead
// of hitting MongoDB on every POST /exposure.
//
// Key schema:
//
//	user:{id}                                  → JSON (TTL 1h)
//	equipment:{id}                             → JSON (TTL 1h)
//	equipment:all                              → JSON array (TTL 5m)
//	exposure:{id}                              → JSON (TTL 24h)
//	exposures:daily:{userID}:{YYYY-MM-DD}      → Sorted Set, score=unix_ts, member=exposureID (TTL 25h)
//	exposures:daily:{userID}:{YYYY-MM-DD}:ready → sentinel — only set after a full MongoDB warm (TTL 25h)
package hybrid

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/ctrl-hub/challenge/db"
)

const (
	ttlUser         = time.Hour
	ttlEquipment    = time.Hour
	ttlEquipmentAll = 5 * time.Minute
	ttlExposure     = 24 * time.Hour
	ttlDailyWindow  = 25 * time.Hour // slightly longer than a day to avoid boundary races
)

// Client wraps a primary db.Client with a Valkey write-through cache.
// It satisfies the db.Client interface and can be swapped in for the
// MongoDB-only client without changing any callers.
type Client struct {
	primary db.Client
	rdb     *redis.Client
}

// New connects to Valkey at valkeyAddr, wraps primary, and returns a Client.
// If Valkey is unreachable the constructor fails fast — if you want a
// degraded-mode startup, wrap primary in a nil-safe fallback instead.
func New(ctx context.Context, primary db.Client, valkeyAddr string) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{Addr: valkeyAddr})
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("hybrid: connecting to Valkey at %s: %w", valkeyAddr, err)
	}
	return &Client{primary: primary, rdb: rdb}, nil
}

// userKey returns the Valkey key for a cached user.
func userKey(id string) string { return "user:" + id }

// equipmentKey returns the Valkey key for a cached equipment item.
func equipmentKey(id string) string { return "equipment:" + id }

// exposureKey returns the Valkey key for a cached exposure record.
func exposureKey(id string) string { return "exposure:" + id }

// dailySetKey returns the sorted-set key for a user's exposure window on a given UTC day.
func dailySetKey(userID string, t time.Time) string {
	return fmt.Sprintf("exposures:daily:%s:%s", userID, t.UTC().Format("2006-01-02"))
}

// dailyReadyKey returns the sentinel key that signals a daily sorted set has
// been fully warmed from MongoDB (as opposed to partially built by CreateExposure calls).
func dailyReadyKey(userID string, t time.Time) string {
	return dailySetKey(userID, t) + ":ready"
}
