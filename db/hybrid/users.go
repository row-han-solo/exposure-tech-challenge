package hybrid

import (
	"context"
	"encoding/json"
	"log"

	"github.com/redis/go-redis/v9"

	"github.com/ctrl-hub/challenge/data"
)

// GetUser checks Valkey first. On a cache miss it fetches from the primary store
// and warms the cache. Valkey errors are non-fatal; the primary store is always
// the source of truth.
func (c *Client) GetUser(ctx context.Context, id string) (*data.User, error) {
	raw, err := c.rdb.Get(ctx, userKey(id)).Bytes()
	if err == nil {
		var u data.User
		if jsonErr := json.Unmarshal(raw, &u); jsonErr == nil {
			return &u, nil
		}
	} else if err != redis.Nil {
		log.Printf("hybrid: Valkey GET %s: %v", userKey(id), err)
	}

	u, err := c.primary.GetUser(ctx, id)
	if err != nil {
		return nil, err
	}

	if b, jsonErr := json.Marshal(u); jsonErr == nil {
		if cacheErr := c.rdb.Set(ctx, userKey(id), b, ttlUser).Err(); cacheErr != nil {
			log.Printf("hybrid: Valkey SET %s: %v", userKey(id), cacheErr)
		}
	}

	return u, nil
}
