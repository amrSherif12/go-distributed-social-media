package follow_service

import (
	"context"
	"sort"
	"strconv"
	"testing"
	"time"

	"go_distributed_social_media/api/pb"
	"go_distributed_social_media/internal/common/utils"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type testRepository struct {
	follows           map[string]map[string]struct{}
	celebrities       map[string]bool
	followerReadCount int
}

func newTestRepository() *testRepository {
	return &testRepository{
		follows:     make(map[string]map[string]struct{}),
		celebrities: make(map[string]bool),
	}
}

func (r *testRepository) Follow(_ context.Context, followerID, followeeID string) (bool, error) {
	if r.follows[followerID] == nil {
		r.follows[followerID] = make(map[string]struct{})
	}
	if _, exists := r.follows[followerID][followeeID]; exists {
		return false, nil
	}
	r.follows[followerID][followeeID] = struct{}{}
	return true, nil
}

func (r *testRepository) Unfollow(_ context.Context, followerID, followeeID string) (bool, error) {
	if _, exists := r.follows[followerID][followeeID]; !exists {
		return false, nil
	}
	delete(r.follows[followerID], followeeID)
	return true, nil
}

func (r *testRepository) GetFollows(_ context.Context, userID string) ([]string, error) {
	ids := make([]string, 0, len(r.follows[userID]))
	for id := range r.follows[userID] {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

func (r *testRepository) GetFollowers(_ context.Context, userID string) ([]string, error) {
	r.followerReadCount++
	ids := make([]string, 0)
	for followerID, follows := range r.follows {
		if _, exists := follows[userID]; exists {
			ids = append(ids, followerID)
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func (r *testRepository) GetCelebrityFollows(ctx context.Context, userID string) ([]string, error) {
	follows, err := r.GetFollows(ctx, userID)
	if err != nil {
		return nil, err
	}
	celebrities := make([]string, 0)
	for _, followeeID := range follows {
		if r.celebrities[followeeID] {
			celebrities = append(celebrities, followeeID)
		}
	}
	return celebrities, nil
}

func (r *testRepository) GetNonCelebrityFollows(ctx context.Context, userID string) ([]string, error) {
	follows, err := r.GetFollows(ctx, userID)
	if err != nil {
		return nil, err
	}
	nonCelebrities := make([]string, 0)
	for _, followeeID := range follows {
		if !r.celebrities[followeeID] {
			nonCelebrities = append(nonCelebrities, followeeID)
		}
	}
	return nonCelebrities, nil
}

func TestServiceFollowAndUnfollow(t *testing.T) {
	service := NewService(newTestRepository())
	ctx := context.Background()

	changed, err := service.Follow(ctx, "follower", "followee")
	require.NoError(t, err)
	assert.True(t, changed)

	changed, err = service.Follow(ctx, "follower", "followee")
	require.NoError(t, err)
	assert.False(t, changed)

	follows, err := service.GetFollows(ctx, "follower")
	require.NoError(t, err)
	assert.Equal(t, []string{"followee"}, follows)

	followers, err := service.GetFollowers(ctx, "followee")
	require.NoError(t, err)
	assert.Equal(t, []string{"follower"}, followers)

	changed, err = service.Unfollow(ctx, "follower", "followee")
	require.NoError(t, err)
	assert.True(t, changed)

	changed, err = service.Unfollow(ctx, "follower", "followee")
	require.NoError(t, err)
	assert.False(t, changed)
}

func TestServiceRejectsInvalidRelationships(t *testing.T) {
	service := NewService(newTestRepository())

	_, err := service.Follow(context.Background(), "", "followee")
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(utils.ParseError(err)))

	_, err = service.Follow(context.Background(), "same-user", "same-user")
	require.Error(t, err)
	assert.Equal(t, "users cannot follow themselves", err.Error())
}

func TestHandlerFollowRequiresAuthenticatedUser(t *testing.T) {
	handler := NewHandler(NewService(newTestRepository()))

	_, err := handler.FollowUser(context.Background(), &pb.FollowUserRequest{FolloweeId: "followee"})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestHandlerInternalCelebrityMethods(t *testing.T) {
	repo := newTestRepository()
	repo.celebrities["celebrity"] = true
	repo.celebrities["regular"] = false
	_, err := repo.Follow(context.Background(), "viewer", "celebrity")
	require.NoError(t, err)
	_, err = repo.Follow(context.Background(), "viewer", "regular")
	require.NoError(t, err)
	handler := NewHandler(NewService(repo))

	response, err := handler.IsCelebrity(context.Background(), &pb.UserRequest{UserId: "celebrity"})
	require.NoError(t, err)
	assert.False(t, response.GetIsCelebrity())

	following, err := handler.GetCelebrityFollows(context.Background(), &pb.UserRequest{UserId: "viewer"})
	require.NoError(t, err)
	assert.Equal(t, []string{"celebrity"}, following.GetUserIds())

	nonCelebrityFollowing, err := handler.GetNonCelebrityFollows(context.Background(), &pb.UserRequest{UserId: "viewer"})
	require.NoError(t, err)
	assert.Equal(t, []string{"regular"}, nonCelebrityFollowing.GetUserIds())
}

func TestCachedRepositoryInvalidatesChangedRelationships(t *testing.T) {
	server := miniredis.RunT(t)
	cache := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, cache.Close()) })

	next := newTestRepository()
	repo := NewCachedRepository(next, cache, time.Minute)
	ctx := context.Background()

	follows, err := repo.GetFollows(ctx, "viewer")
	require.NoError(t, err)
	assert.Empty(t, follows)

	followers, err := repo.GetFollowers(ctx, "celebrity")
	require.NoError(t, err)
	assert.Empty(t, followers)

	service := NewService(repo)
	isCelebrity, err := service.IsCelebrity(ctx, "celebrity")
	require.NoError(t, err)
	assert.False(t, isCelebrity)
	followerReadsAfterInitialStatus := next.followerReadCount

	changed, err := repo.Follow(ctx, "viewer", "celebrity")
	require.NoError(t, err)
	assert.True(t, changed)

	follows, err = repo.GetFollows(ctx, "viewer")
	require.NoError(t, err)
	assert.Equal(t, []string{"celebrity"}, follows)

	followers, err = repo.GetFollowers(ctx, "celebrity")
	require.NoError(t, err)
	assert.Equal(t, []string{"viewer"}, followers)

	isCelebrity, err = service.IsCelebrity(ctx, "celebrity")
	require.NoError(t, err)
	assert.False(t, isCelebrity)
	assert.Equal(t, followerReadsAfterInitialStatus+1, next.followerReadCount, "follow must invalidate the cached follower list")

	changed, err = repo.Unfollow(ctx, "viewer", "celebrity")
	require.NoError(t, err)
	assert.True(t, changed)

	follows, err = repo.GetFollows(ctx, "viewer")
	require.NoError(t, err)
	assert.Empty(t, follows)
}

func TestServiceIsCelebrityUsesCachedFollowerCount(t *testing.T) {
	repo := newTestRepository()
	for i := 0; i < CelebrityFollowerThreshold; i++ {
		repo.follows[strconv.Itoa(i)] = map[string]struct{}{"celebrity": {}}
	}
	service := NewService(repo)

	isCelebrity, err := service.IsCelebrity(context.Background(), "celebrity")
	require.NoError(t, err)
	assert.True(t, isCelebrity)
}

func TestHandlerFollowUsesUIDFromGRPCMetadata(t *testing.T) {
	handler := NewHandler(NewService(newTestRepository()))
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("uid", "follower"))

	response, err := handler.FollowUser(ctx, &pb.FollowUserRequest{FolloweeId: "followee"})
	require.NoError(t, err)
	assert.True(t, response.GetChanged())
}
