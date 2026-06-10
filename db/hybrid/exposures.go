package hybrid

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/ctrl-hub/challenge/data"
)

// CreateExposure writes to the primary store first (hard fail on error), then
// updates Valkey asynchronously:
//
//  1. Cache the individual exposure by ID for fast GetExposure lookups.
//  2. Add its ID to the per-user daily sorted set so that subsequent
//     GetExposuresByUser calls for today can be served entirely from Valkey.
//
// The daily sorted set is created here even on a cold cache, but it is NOT
// marked "ready" until a full MongoDB warm has happened. This prevents
// GetExposuresByUser from returning a partial result if the set only contains
// exposures added since the last service restart.
func (c *Client) CreateExposure(ctx context.Context, e *data.Exposure) error {
	if err := c.primary.CreateExposure(ctx, e); err != nil {
		return err
	}

	// Cache the individual exposure record.
	if b, err := json.Marshal(e); err == nil {
		if cacheErr := c.rdb.Set(ctx, exposureKey(e.ID), b, ttlExposure).Err(); cacheErr != nil {
			log.Printf("hybrid: Valkey SET %s: %v", exposureKey(e.ID), cacheErr)
		}
	}

	// Add to the daily sorted set and refresh its TTL. Score = Unix timestamp
	// so ZRANGEBYSCORE can efficiently filter by time window.
	setKey := dailySetKey(e.User.ID, e.CreatedAt)
	pipe := c.rdb.Pipeline()
	pipe.ZAdd(ctx, setKey, redis.Z{
		Score:  float64(e.CreatedAt.Unix()),
		Member: e.ID,
	})
	pipe.Expire(ctx, setKey, ttlDailyWindow)
	if _, err := pipe.Exec(ctx); err != nil {
		log.Printf("hybrid: Valkey ZADD daily set for user %s: %v", e.User.ID, err)
	}

	return nil
}

// GetExposure checks Valkey first and falls through to the primary store on a miss.
func (c *Client) GetExposure(ctx context.Context, id string) (*data.Exposure, error) {
	raw, err := c.rdb.Get(ctx, exposureKey(id)).Bytes()
	if err == nil {
		var e data.Exposure
		if jsonErr := json.Unmarshal(raw, &e); jsonErr == nil {
			return &e, nil
		}
	} else if err != redis.Nil {
		log.Printf("hybrid: Valkey GET %s: %v", exposureKey(id), err)
	}

	e, err := c.primary.GetExposure(ctx, id)
	if err != nil {
		return nil, err
	}

	if b, jsonErr := json.Marshal(e); jsonErr == nil {
		if cacheErr := c.rdb.Set(ctx, exposureKey(id), b, ttlExposure).Err(); cacheErr != nil {
			log.Printf("hybrid: Valkey SET %s: %v", exposureKey(id), cacheErr)
		}
	}

	return e, nil
}

// ListExposures always reads from the primary store. This is a broad admin
// operation where caching provides little benefit and risks serving stale data.
func (c *Client) ListExposures(ctx context.Context) ([]*data.Exposure, error) {
	return c.primary.ListExposures(ctx)
}

// GetExposuresByUser uses the Valkey daily sorted set for single-day window
// queries — the hot path called by exposure.GetSummary on every POST /exposure
// to evaluate EAV/ELV thresholds.
//
// For all other queries (no time filter, multi-day range) it falls through to
// the primary store directly.
//
// Single-day cache flow:
//  1. Check the ":ready" sentinel — if absent, the sorted set has not been fully
//     warmed from MongoDB yet. Fall through to MongoDB and warm on the way back.
//  2. ZRANGEBYSCORE to get exposure IDs within the window.
//  3. Pipeline GET all individual exposure keys.
//  4. If every key hits: reconstruct and return from cache.
//  5. If any key misses: fall through to MongoDB and re-warm the cache.
func (c *Client) GetExposuresByUser(ctx context.Context, userID string, start, end *time.Time) ([]*data.Exposure, error) {
	if start != nil && end != nil && isSingleDayWindow(*start, *end) {
		if exposures, ok, err := c.getDailyFromCache(ctx, userID, *start, *end); err != nil {
			log.Printf("hybrid: daily cache read for user %s: %v", userID, err)
		} else if ok {
			return exposures, nil
		}
	}

	exposures, err := c.primary.GetExposuresByUser(ctx, userID, start, end)
	if err != nil {
		return nil, err
	}

	// Warm the daily sorted set after a MongoDB fallback so subsequent calls
	// within the same day are served from cache.
	if start != nil && end != nil && isSingleDayWindow(*start, *end) {
		c.warmDailyCache(ctx, userID, *start, exposures)
	}

	return exposures, nil
}

// isSingleDayWindow returns true when start and end span at most one UTC
// calendar day — i.e. end falls within or exactly at the boundary of the day
// that start belongs to. This covers the standard 00:00→00:00+24h window
// used by publishThresholdEvents.
func isSingleDayWindow(start, end time.Time) bool {
	dayBoundary := start.UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)
	return !end.UTC().After(dayBoundary)
}

// getDailyFromCache attempts a full cache hit from the Valkey daily sorted set.
// Returns (exposures, true, nil) on a complete hit, (nil, false, nil) on any
// cache miss, or (nil, false, err) on a Valkey connectivity error.
func (c *Client) getDailyFromCache(ctx context.Context, userID string, start, end time.Time) ([]*data.Exposure, bool, error) {
	// Only trust the sorted set if it has been fully warmed from MongoDB.
	ready, err := c.rdb.Exists(ctx, dailyReadyKey(userID, start)).Result()
	if err != nil {
		return nil, false, err
	}
	if ready == 0 {
		return nil, false, nil // cold cache
	}

	// Fetch exposure IDs within the time range.
	ids, err := c.rdb.ZRangeByScore(ctx, dailySetKey(userID, start), &redis.ZRangeBy{
		Min: strconv.FormatInt(start.Unix(), 10),
		Max: strconv.FormatInt(end.Unix(), 10),
	}).Result()
	if err != nil {
		return nil, false, err
	}

	// An empty sorted set for a "ready" key means no exposures that day — valid result.
	if len(ids) == 0 {
		return []*data.Exposure{}, true, nil
	}

	// Pipeline GET all individual exposure objects to reconstruct the slice.
	pipe := c.rdb.Pipeline()
	cmds := make([]*redis.StringCmd, len(ids))
	for i, id := range ids {
		cmds[i] = pipe.Get(ctx, exposureKey(id))
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, false, fmt.Errorf("pipeline exec: %w", err)
	}

	exposures := make([]*data.Exposure, 0, len(ids))
	for _, cmd := range cmds {
		raw, err := cmd.Bytes()
		if err == redis.Nil {
			// Individual exposure evicted before the daily set expired.
			// Treat the whole query as a miss so MongoDB stays consistent.
			return nil, false, nil
		}
		if err != nil {
			return nil, false, err
		}
		var e data.Exposure
		if jsonErr := json.Unmarshal(raw, &e); jsonErr != nil {
			return nil, false, nil // corrupted entry — full miss
		}
		exposures = append(exposures, &e)
	}

	return exposures, true, nil
}

// warmDailyCache populates the Valkey sorted set and individual exposure keys
// from a slice of MongoDB results, then sets the ":ready" sentinel so future
// reads can trust the sorted set is complete. All operations are non-fatal.
func (c *Client) warmDailyCache(ctx context.Context, userID string, day time.Time, exposures []*data.Exposure) {
	setKey := dailySetKey(userID, day)
	readyKey := dailyReadyKey(userID, day)

	pipe := c.rdb.Pipeline()
	for _, e := range exposures {
		b, err := json.Marshal(e)
		if err != nil {
			continue
		}
		pipe.Set(ctx, exposureKey(e.ID), b, ttlExposure)
		pipe.ZAdd(ctx, setKey, redis.Z{
			Score:  float64(e.CreatedAt.Unix()),
			Member: e.ID,
		})
	}
	pipe.Expire(ctx, setKey, ttlDailyWindow)
	// Mark the sorted set as fully populated. This is set last so that a
	// partial pipeline failure cannot leave a "ready" flag over an incomplete set.
	pipe.Set(ctx, readyKey, "1", ttlDailyWindow)

	if _, err := pipe.Exec(ctx); err != nil {
		log.Printf("hybrid: warming daily cache for user %s: %v", userID, err)
	}
}
