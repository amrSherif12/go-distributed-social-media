package post_service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

type CachedMetadataRepo struct {
	next  MetadataRepo
	cache redis.UniversalClient
	ttl   time.Duration
	loads singleflight.Group
}

func NewCachedMetadataRepo(next MetadataRepo, cache redis.UniversalClient, ttl time.Duration) *CachedMetadataRepo {
	return &CachedMetadataRepo{next: next, cache: cache, ttl: ttl}
}

func (r *CachedMetadataRepo) Create(ctx context.Context, dto CreatePostDTO) (CreatePostResponse, error) {
	return r.next.Create(ctx, dto)
}

func (r *CachedMetadataRepo) Get(ctx context.Context, postID string) (Post, error) {
	key := "post:" + postID
	if post, ok := r.getCached(ctx, key); ok {
		return post, nil
	}

	result, err, _ := r.loads.Do(key, func() (any, error) {
		if post, ok := r.getCached(ctx, key); ok {
			return post, nil
		}

		post, err := r.next.Get(ctx, postID)
		if err != nil {
			return Post{}, err
		}

		encoded, err := json.Marshal(post)
		if err == nil && r.ttl > 0 {
			_ = r.cache.Set(ctx, key, encoded, r.ttl).Err()
		}
		return post, nil
	})
	if err != nil {
		return Post{}, err
	}
	return result.(Post), nil
}

func (r *CachedMetadataRepo) GetByUsers(ctx context.Context, dto GetPostsByUsersDTO) ([]Post, error) {
	return r.next.GetByUsers(ctx, dto)
}

func (r *CachedMetadataRepo) getCached(ctx context.Context, key string) (Post, bool) {
	value, err := r.cache.Get(ctx, key).Result()
	if err != nil {
		return Post{}, false
	}
	var post Post
	if err := json.Unmarshal([]byte(value), &post); err != nil {
		_ = r.cache.Del(ctx, key).Err()
		return Post{}, false
	}
	return post, true
}
