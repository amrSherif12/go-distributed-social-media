package feed_service

import (
	"context"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
)

type Cache struct {
	client redis.UniversalClient
}

func NewCache(client redis.UniversalClient) *Cache {
	return &Cache{client: client}
}

func (c *Cache) AddUserPost(ctx context.Context, userID string, post CachedPost) error {
	if userID == "" || post.UserID == "" || post.PostID == "" {
		return fmt.Errorf("user feed cache requires user, author, and post ids")
	}
	member := post.PostID + "\x00" + post.UserID
	return c.addPost(ctx, userFeedKey(userID), member, post.CreatedAt, UserFeedLimit)
}

func (c *Cache) AddCelebrityPost(ctx context.Context, celebrityID string, post CachedPost) error {
	if celebrityID == "" || post.PostID == "" {
		return fmt.Errorf("celebrity cache requires celebrity and post ids")
	}
	return c.addPost(ctx, celebrityFeedKey(celebrityID), post.PostID, post.CreatedAt, CelebrityPostLimit)
}

func (c *Cache) GetUserPosts(ctx context.Context, userID string) ([]CachedPost, error) {
	entries, err := c.client.ZRevRangeWithScores(ctx, userFeedKey(userID), 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("read user feed cache: %w", err)
	}
	posts := make([]CachedPost, 0, len(entries))
	for _, entry := range entries {
		member, ok := entry.Member.(string)
		if !ok {
			return nil, fmt.Errorf("user feed cache contains a non-string member")
		}
		postID, authorID, ok := strings.Cut(member, "\x00")
		if !ok || postID == "" || authorID == "" {
			return nil, fmt.Errorf("user feed cache contains an invalid post entry")
		}
		posts = append(posts, CachedPost{
			PostID:    postID,
			UserID:    authorID,
			CreatedAt: int64(entry.Score),
		})
	}
	return posts, nil
}

func (c *Cache) GetCelebrityPosts(ctx context.Context, celebrityIDs []string) ([]CachedPost, error) {
	posts := make([]CachedPost, 0, len(celebrityIDs)*CelebrityPostLimit)
	for _, celebrityID := range celebrityIDs {
		entries, err := c.client.ZRevRangeWithScores(ctx, celebrityFeedKey(celebrityID), 0, -1).Result()
		if err != nil {
			return nil, fmt.Errorf("read celebrity post cache for %q: %w", celebrityID, err)
		}
		for _, entry := range entries {
			postID, ok := entry.Member.(string)
			if !ok || postID == "" {
				return nil, fmt.Errorf("celebrity post cache contains an invalid post entry")
			}
			posts = append(posts, CachedPost{
				PostID:    postID,
				UserID:    celebrityID,
				CreatedAt: int64(entry.Score),
			})
		}
	}
	return posts, nil
}

func (c *Cache) addPost(ctx context.Context, key, member string, createdAt int64, limit int64) error {
	const trimScript = `
		redis.call("ZADD", KEYS[1], ARGV[1], ARGV[2])
		local limit = tonumber(ARGV[3])
		local count = redis.call("ZCARD", KEYS[1])
		if count > limit then
			redis.call("ZREMRANGEBYRANK", KEYS[1], 0, -(limit + 1))
		end
		return count
	`
	if err := c.client.Eval(ctx, trimScript, []string{key}, createdAt, member, limit).Err(); err != nil {
		return fmt.Errorf("write bounded feed cache: %w", err)
	}
	return nil
}

func userFeedKey(userID string) string {
	return "feed:user:" + userID
}

func celebrityFeedKey(celebrityID string) string {
	return "feed:celebrity:" + celebrityID
}
