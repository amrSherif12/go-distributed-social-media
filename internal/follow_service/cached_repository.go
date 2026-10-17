package follow_service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type CachedRepository struct {
	next  Repository
	cache redis.UniversalClient
	ttl   time.Duration
}

func NewCachedRepository(next Repository, cache redis.UniversalClient, ttl time.Duration) *CachedRepository {
	return &CachedRepository{next: next, cache: cache, ttl: ttl}
}

func (r *CachedRepository) Follow(ctx context.Context, followerID, followeeID string) (bool, error) {
	changed, err := r.next.Follow(ctx, followerID, followeeID)
	if err != nil || !changed {
		return changed, err
	}
	if err := r.invalidateRelationship(ctx, followerID, followeeID); err != nil {
		return false, err
	}
	return true, nil
}

func (r *CachedRepository) Unfollow(ctx context.Context, followerID, followeeID string) (bool, error) {
	changed, err := r.next.Unfollow(ctx, followerID, followeeID)
	if err != nil || !changed {
		return changed, err
	}
	if err := r.invalidateRelationship(ctx, followerID, followeeID); err != nil {
		return false, err
	}
	return true, nil
}

func (r *CachedRepository) GetFollows(ctx context.Context, userID string) ([]string, error) {
	return r.getIDs(ctx, "follow:follows:"+userID, func() ([]string, error) {
		return r.next.GetFollows(ctx, userID)
	})
}

func (r *CachedRepository) GetFollowers(ctx context.Context, userID string) ([]string, error) {
	return r.getIDs(ctx, "follow:followers:"+userID, func() ([]string, error) {
		return r.next.GetFollowers(ctx, userID)
	})
}

func (r *CachedRepository) GetCelebrityFollows(ctx context.Context, userID string) ([]string, error) {
	return r.next.GetCelebrityFollows(ctx, userID)
}

func (r *CachedRepository) GetNonCelebrityFollows(ctx context.Context, userID string) ([]string, error) {
	return r.next.GetNonCelebrityFollows(ctx, userID)
}

func (r *CachedRepository) getIDs(ctx context.Context, key string, load func() ([]string, error)) ([]string, error) {
	value, err := r.cache.Get(ctx, key).Result()
	if err == nil {
		var ids []string
		if decodeErr := json.Unmarshal([]byte(value), &ids); decodeErr == nil {
			return ids, nil
		}
		if err := r.cache.Del(ctx, key).Err(); err != nil {
			return nil, fmt.Errorf("delete invalid follow-list cache entry: %w", err)
		}
	} else if !errors.Is(err, redis.Nil) {
		return nil, fmt.Errorf("read follow-list cache: %w", err)
	}

	ids, err := load()
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(ids)
	if err != nil {
		return nil, fmt.Errorf("encode follow list: %w", err)
	}
	if err := r.cache.Set(ctx, key, encoded, r.ttl).Err(); err != nil {
		return nil, fmt.Errorf("cache follow list: %w", err)
	}
	return ids, nil
}

func (r *CachedRepository) invalidateRelationship(ctx context.Context, followerID, followeeID string) error {
	err := r.cache.Del(ctx,
		"follow:follows:"+followerID,
		"follow:followers:"+followeeID,
	).Err()
	if err != nil {
		return fmt.Errorf("invalidate follow cache: %w", err)
	}
	return nil
}
