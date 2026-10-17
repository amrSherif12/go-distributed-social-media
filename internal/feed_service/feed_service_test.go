package feed_service

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"go_distributed_social_media/api/pb"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

type fakeFollowClient struct {
	celebrityIDs []string
	normalIDs    []string
	followerIDs  []string
	celebrity    bool
}

func (f *fakeFollowClient) GetFollowers(context.Context, *pb.GetFollowersRequest, ...grpc.CallOption) (*pb.UserListResponse, error) {
	return &pb.UserListResponse{UserIds: f.followerIDs}, nil
}

func (f *fakeFollowClient) GetCelebrityFollows(context.Context, *pb.UserRequest, ...grpc.CallOption) (*pb.UserListResponse, error) {
	return &pb.UserListResponse{UserIds: f.celebrityIDs}, nil
}

func (f *fakeFollowClient) GetNonCelebrityFollows(context.Context, *pb.UserRequest, ...grpc.CallOption) (*pb.UserListResponse, error) {
	return &pb.UserListResponse{UserIds: f.normalIDs}, nil
}

func (f *fakeFollowClient) IsCelebrity(context.Context, *pb.UserRequest, ...grpc.CallOption) (*pb.CelebrityResponse, error) {
	return &pb.CelebrityResponse{IsCelebrity: f.celebrity}, nil
}

type fakePostClient struct {
	posts         []*pb.GetPostResponse
	backfillCalls []*pb.GetPostsByUsersRequest
}

func (p *fakePostClient) GetPost(_ context.Context, request *pb.GetPostRequest, _ ...grpc.CallOption) (*pb.GetPostResponse, error) {
	for _, post := range p.posts {
		if post.GetPostId() == request.GetPostId() {
			return post, nil
		}
	}
	return nil, fmt.Errorf("post %q not found", request.GetPostId())
}

func (p *fakePostClient) GetPostsByUsers(_ context.Context, request *pb.GetPostsByUsersRequest, _ ...grpc.CallOption) (*pb.GetPostsByUsersResponse, error) {
	p.backfillCalls = append(p.backfillCalls, request)
	users := make(map[string]struct{}, len(request.GetUserIds()))
	for _, userID := range request.GetUserIds() {
		users[userID] = struct{}{}
	}
	posts := make([]*pb.GetPostResponse, 0)
	for _, post := range p.posts {
		if _, ok := users[post.GetUserId()]; !ok {
			continue
		}
		if request.GetBeforeCreatedAt() > 0 {
			if post.GetCreatedAt() > request.GetBeforeCreatedAt() ||
				(post.GetCreatedAt() == request.GetBeforeCreatedAt() &&
					(request.GetBeforePostId() == "" || post.GetPostId() >= request.GetBeforePostId())) {
				continue
			}
		}
		posts = append(posts, post)
	}
	sort.Slice(posts, func(i, j int) bool {
		if posts[i].GetCreatedAt() == posts[j].GetCreatedAt() {
			return posts[i].GetPostId() > posts[j].GetPostId()
		}
		return posts[i].GetCreatedAt() > posts[j].GetCreatedAt()
	})
	if len(posts) > 10 {
		posts = posts[:10]
	}
	return &pb.GetPostsByUsersResponse{Posts: posts}, nil
}

func newTestCache(t *testing.T) *Cache {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewCache(client)
}

func TestCacheEnforcesFeedCaps(t *testing.T) {
	cache := newTestCache(t)
	ctx := context.Background()
	for index := 1; index <= UserFeedLimit+5; index++ {
		require.NoError(t, cache.AddUserPost(ctx, "viewer", CachedPost{
			PostID:    fmt.Sprintf("post-%03d", index),
			UserID:    "author",
			CreatedAt: int64(index),
		}))
	}
	userPosts, err := cache.GetUserPosts(ctx, "viewer")
	require.NoError(t, err)
	require.Len(t, userPosts, UserFeedLimit)
	assert.Equal(t, "post-055", userPosts[0].PostID)
	assert.Equal(t, "post-006", userPosts[len(userPosts)-1].PostID)

	for index := 1; index <= CelebrityPostLimit+3; index++ {
		require.NoError(t, cache.AddCelebrityPost(ctx, "celebrity", CachedPost{
			PostID:    fmt.Sprintf("celebrity-post-%03d", index),
			UserID:    "celebrity",
			CreatedAt: int64(index),
		}))
	}
	celebrityPosts, err := cache.GetCelebrityPosts(ctx, []string{"celebrity"})
	require.NoError(t, err)
	require.Len(t, celebrityPosts, CelebrityPostLimit)
	assert.Equal(t, "celebrity-post-013", celebrityPosts[0].PostID)
	assert.Equal(t, "celebrity-post-004", celebrityPosts[len(celebrityPosts)-1].PostID)
}

func TestHandlePostCreatedRoutesToFanoutOrCelebrityCache(t *testing.T) {
	ctx := context.Background()
	cache := newTestCache(t)
	follow := &fakeFollowClient{followerIDs: []string{"viewer-a", "viewer-b"}}
	service := NewService(follow, &fakePostClient{}, cache)

	require.NoError(t, service.HandlePostCreated(ctx, PostCreatedEvent{
		PostID: "normal-post", UserID: "normal-author", Timestamp: 100,
	}))
	for _, userID := range follow.followerIDs {
		posts, err := cache.GetUserPosts(ctx, userID)
		require.NoError(t, err)
		require.Len(t, posts, 1)
		assert.Equal(t, "normal-post", posts[0].PostID)
	}

	follow.celebrity = true
	require.NoError(t, service.HandlePostCreated(ctx, PostCreatedEvent{
		PostID: "celebrity-post", UserID: "celebrity-author", Timestamp: 101,
	}))
	celebrityPosts, err := cache.GetCelebrityPosts(ctx, []string{"celebrity-author"})
	require.NoError(t, err)
	require.Len(t, celebrityPosts, 1)
	assert.Equal(t, "celebrity-post", celebrityPosts[0].PostID)
}

func TestGetFeedBackfillsAndPaginatesMergedSources(t *testing.T) {
	ctx := context.Background()
	cache := newTestCache(t)
	follow := &fakeFollowClient{
		celebrityIDs: []string{"celebrity"},
		normalIDs:    []string{"normal"},
	}
	posts := &fakePostClient{}
	for index := 1; index <= 15; index++ {
		posts.posts = append(posts.posts,
			&pb.GetPostResponse{
				PostId: fmt.Sprintf("normal-%03d", index), UserId: "normal",
				Content: "normal post", CreatedAt: int64(100 + index),
			},
			&pb.GetPostResponse{
				PostId: fmt.Sprintf("celebrity-%03d", index), UserId: "celebrity",
				Content: "celebrity post", CreatedAt: int64(100 + index),
			},
		)
	}
	service := NewService(follow, posts, cache)

	first, err := service.GetFeed(ctx, "viewer", FeedCursor{})
	require.NoError(t, err)
	require.Len(t, first.Posts, FeedPageSize)
	require.Equal(t, "normal-015", first.Posts[0].GetPostId())
	require.Equal(t, "celebrity-011", first.Posts[len(first.Posts)-1].GetPostId())
	require.Equal(t, int64(111), first.NextCursor.CreatedAt)
	require.Equal(t, "celebrity-011", first.NextCursor.PostID)
	require.Len(t, posts.backfillCalls, 2)

	second, err := service.GetFeed(ctx, "viewer", first.NextCursor)
	require.NoError(t, err)
	require.Len(t, second.Posts, FeedPageSize)
	assert.Equal(t, "normal-010", second.Posts[0].GetPostId())
	assert.Equal(t, "celebrity-006", second.Posts[len(second.Posts)-1].GetPostId())

	third, err := service.GetFeed(ctx, "viewer", second.NextCursor)
	require.NoError(t, err)
	require.Len(t, third.Posts, FeedPageSize)
	assert.Equal(t, "normal-005", third.Posts[0].GetPostId())
	assert.Equal(t, "celebrity-001", third.Posts[len(third.Posts)-1].GetPostId())
}

func TestGetFeedUsesWarmCacheWithoutBackfill(t *testing.T) {
	ctx := context.Background()
	cache := newTestCache(t)
	for index := 1; index <= FeedPageSize; index++ {
		require.NoError(t, cache.AddUserPost(ctx, "viewer", CachedPost{
			PostID: fmt.Sprintf("post-%02d", index), UserID: "normal", CreatedAt: int64(index),
		}))
	}
	follow := &fakeFollowClient{normalIDs: []string{"normal"}}
	posts := &fakePostClient{}
	for index := 1; index <= FeedPageSize; index++ {
		posts.posts = append(posts.posts, &pb.GetPostResponse{
			PostId: fmt.Sprintf("post-%02d", index), UserId: "normal", CreatedAt: int64(index),
		})
	}

	page, err := NewService(follow, posts, cache).GetFeed(ctx, "viewer", FeedCursor{})
	require.NoError(t, err)
	require.Len(t, page.Posts, FeedPageSize)
	assert.Empty(t, posts.backfillCalls)
}
