package redisinfra

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
)

type Cache struct{ client *redis.Client }

func NewCache(client *redis.Client) *Cache { return &Cache{client: client} }

func (c *Cache) SetStockSnapshot(ctx context.Context, sku string, payload []byte, ttl time.Duration) error {
	key := "stock:sku:" + sku
	return c.client.Set(ctx, key, payload, ttl).Err()
}

func (c *Cache) GetStockSnapshot(ctx context.Context, sku string) ([]byte, error) {
	key := "stock:sku:" + sku
	v, err := c.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	return v, err
}

func (c *Cache) DeleteStockSnapshot(ctx context.Context, sku string) error {
	key := "stock:sku:" + sku
	return c.client.Del(ctx, key).Err()
}

// Locker using simple SetNX
func (c *Cache) Lock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return c.client.SetNX(ctx, key, "1", ttl).Result()
}
func (c *Cache) Unlock(ctx context.Context, key string) error {
	_, err := c.client.Del(ctx, key).Result()
	return err
}
