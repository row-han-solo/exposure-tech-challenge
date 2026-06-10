package hybrid

import (
	"context"
	"encoding/json"
	"log"

	"github.com/redis/go-redis/v9"

	"github.com/ctrl-hub/challenge/data"
)

const equipmentListKey = "equipment:all"

// GetEquipment checks Valkey first. On a cache miss it fetches from the primary
// store and warms the cache. Equipment changes very rarely, so a 1h TTL is safe.
func (c *Client) GetEquipment(ctx context.Context, id string) (*data.EquipmentItem, error) {
	raw, err := c.rdb.Get(ctx, equipmentKey(id)).Bytes()
	if err == nil {
		var eq data.EquipmentItem
		if jsonErr := json.Unmarshal(raw, &eq); jsonErr == nil {
			return &eq, nil
		}
	} else if err != redis.Nil {
		log.Printf("hybrid: Valkey GET %s: %v", equipmentKey(id), err)
	}

	eq, err := c.primary.GetEquipment(ctx, id)
	if err != nil {
		return nil, err
	}

	if b, jsonErr := json.Marshal(eq); jsonErr == nil {
		if cacheErr := c.rdb.Set(ctx, equipmentKey(id), b, ttlEquipment).Err(); cacheErr != nil {
			log.Printf("hybrid: Valkey SET %s: %v", equipmentKey(id), cacheErr)
		}
	}

	return eq, nil
}

// ListEquipment checks a single JSON-array cache key. The TTL is kept short
// (5 min) since new equipment items could be added out-of-band.
func (c *Client) ListEquipment(ctx context.Context) ([]*data.EquipmentItem, error) {
	raw, err := c.rdb.Get(ctx, equipmentListKey).Bytes()
	if err == nil {
		var items []*data.EquipmentItem
		if jsonErr := json.Unmarshal(raw, &items); jsonErr == nil {
			return items, nil
		}
	} else if err != redis.Nil {
		log.Printf("hybrid: Valkey GET %s: %v", equipmentListKey, err)
	}

	items, err := c.primary.ListEquipment(ctx)
	if err != nil {
		return nil, err
	}

	if b, jsonErr := json.Marshal(items); jsonErr == nil {
		if cacheErr := c.rdb.Set(ctx, equipmentListKey, b, ttlEquipmentAll).Err(); cacheErr != nil {
			log.Printf("hybrid: Valkey SET %s: %v", equipmentListKey, cacheErr)
		}
	}

	return items, nil
}
