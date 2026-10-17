package feed_service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"go_distributed_social_media/api/pb"
	"go_distributed_social_media/internal/common/errs"

	"google.golang.org/grpc"
)

type FollowClient interface {
	GetFollowers(context.Context, *pb.GetFollowersRequest, ...grpc.CallOption) (*pb.UserListResponse, error)
	GetCelebrityFollows(context.Context, *pb.UserRequest, ...grpc.CallOption) (*pb.UserListResponse, error)
	GetNonCelebrityFollows(context.Context, *pb.UserRequest, ...grpc.CallOption) (*pb.UserListResponse, error)
	IsCelebrity(context.Context, *pb.UserRequest, ...grpc.CallOption) (*pb.CelebrityResponse, error)
}

type PostClient interface {
	GetPost(context.Context, *pb.GetPostRequest, ...grpc.CallOption) (*pb.GetPostResponse, error)
	GetPostsByUsers(context.Context, *pb.GetPostsByUsersRequest, ...grpc.CallOption) (*pb.GetPostsByUsersResponse, error)
}

type FeedCache interface {
	AddUserPost(context.Context, string, CachedPost) error
	AddCelebrityPost(context.Context, string, CachedPost) error
	GetUserPosts(context.Context, string) ([]CachedPost, error)
	GetCelebrityPosts(context.Context, []string) ([]CachedPost, error)
}

type Service struct {
	follow FollowClient
	posts  PostClient
	cache  FeedCache
}

func NewService(follow FollowClient, posts PostClient, cache FeedCache) *Service {
	return &Service{follow: follow, posts: posts, cache: cache}
}

func (s *Service) HandlePostCreated(ctx context.Context, event PostCreatedEvent) error {
	if strings.TrimSpace(event.PostID) == "" || strings.TrimSpace(event.UserID) == "" || event.Timestamp <= 0 {
		return fmt.Errorf("post-created event is missing a post id, user id, or timestamp")
	}

	celebrity, err := s.follow.IsCelebrity(ctx, &pb.UserRequest{UserId: event.UserID})
	if err != nil {
		return fmt.Errorf("check post author celebrity status: %w", err)
	}
	post := CachedPost{PostID: event.PostID, UserID: event.UserID, CreatedAt: event.Timestamp}
	if celebrity.GetIsCelebrity() {
		if err := s.cache.AddCelebrityPost(ctx, event.UserID, post); err != nil {
			return fmt.Errorf("cache celebrity post: %w", err)
		}
		return nil
	}

	followers, err := s.follow.GetFollowers(ctx, &pb.GetFollowersRequest{UserId: event.UserID})
	if err != nil {
		return fmt.Errorf("get post author's followers: %w", err)
	}
	for _, followerID := range followers.GetUserIds() {
		if err := s.cache.AddUserPost(ctx, followerID, post); err != nil {
			return fmt.Errorf("fan out post to user %q: %w", followerID, err)
		}
	}
	return nil
}

func (s *Service) GetFeed(ctx context.Context, userID string, cursor FeedCursor) (FeedPage, error) {
	if strings.TrimSpace(userID) == "" {
		return FeedPage{}, &errs.Err{Err: errs.ValidationFailed, Message: "missing user id"}
	}
	if cursor.CreatedAt < 0 || (cursor.CreatedAt == 0 && cursor.PostID != "") {
		return FeedPage{}, &errs.Err{Err: errs.ValidationFailed, Message: "invalid feed cursor"}
	}

	celebrityResponse, err := s.follow.GetCelebrityFollows(ctx, &pb.UserRequest{UserId: userID})
	if err != nil {
		return FeedPage{}, fmt.Errorf("get followed celebrities: %w", err)
	}
	normalResponse, err := s.follow.GetNonCelebrityFollows(ctx, &pb.UserRequest{UserId: userID})
	if err != nil {
		return FeedPage{}, fmt.Errorf("get non-celebrity follows: %w", err)
	}
	celebrityIDs := celebrityResponse.GetUserIds()
	normalIDs := normalResponse.GetUserIds()
	celebritySet := idSet(celebrityIDs)
	normalSet := idSet(normalIDs)

	userCached, err := s.cache.GetUserPosts(ctx, userID)
	if err != nil {
		return FeedPage{}, err
	}
	celebrityCached, err := s.cache.GetCelebrityPosts(ctx, celebrityIDs)
	if err != nil {
		return FeedPage{}, err
	}

	candidates := make(map[string]CachedPost, UserFeedLimit+len(celebrityIDs)*CelebrityPostLimit)
	postData := make(map[string]*pb.GetPostResponse)
	for _, post := range userCached {
		if _, followed := normalSet[post.UserID]; followed && isBefore(post, cursor) {
			candidates[post.PostID] = post
		}
	}
	for _, post := range celebrityCached {
		if _, followed := celebritySet[post.UserID]; followed && isBefore(post, cursor) {
			candidates[post.PostID] = post
		}
	}

	if countCandidates(candidates, normalSet) < FeedPageSize && len(normalIDs) > 0 {
		response, err := s.posts.GetPostsByUsers(ctx, &pb.GetPostsByUsersRequest{
			UserIds:         normalIDs,
			BeforeCreatedAt: cursor.CreatedAt,
			BeforePostId:    cursor.PostID,
		})
		if err != nil {
			return FeedPage{}, fmt.Errorf("backfill non-celebrity feed: %w", err)
		}
		for _, post := range response.GetPosts() {
			if _, followed := normalSet[post.GetUserId()]; !followed || !isBefore(CachedPost{PostID: post.GetPostId(), CreatedAt: post.GetCreatedAt()}, cursor) {
				continue
			}
			cached := CachedPost{PostID: post.GetPostId(), UserID: post.GetUserId(), CreatedAt: post.GetCreatedAt()}
			candidates[cached.PostID] = cached
			postData[cached.PostID] = post
			if err := s.cache.AddUserPost(ctx, userID, cached); err != nil {
				return FeedPage{}, fmt.Errorf("cache non-celebrity backfill: %w", err)
			}
		}
	}

	if countCandidates(candidates, celebritySet) < FeedPageSize && len(celebrityIDs) > 0 {
		response, err := s.posts.GetPostsByUsers(ctx, &pb.GetPostsByUsersRequest{
			UserIds:         celebrityIDs,
			BeforeCreatedAt: cursor.CreatedAt,
			BeforePostId:    cursor.PostID,
		})
		if err != nil {
			return FeedPage{}, fmt.Errorf("backfill celebrity feed: %w", err)
		}
		for _, post := range response.GetPosts() {
			if _, followed := celebritySet[post.GetUserId()]; !followed || !isBefore(CachedPost{PostID: post.GetPostId(), CreatedAt: post.GetCreatedAt()}, cursor) {
				continue
			}
			cached := CachedPost{PostID: post.GetPostId(), UserID: post.GetUserId(), CreatedAt: post.GetCreatedAt()}
			candidates[cached.PostID] = cached
			postData[cached.PostID] = post
			if err := s.cache.AddCelebrityPost(ctx, cached.UserID, cached); err != nil {
				return FeedPage{}, fmt.Errorf("cache celebrity backfill: %w", err)
			}
		}
	}

	ordered := make([]CachedPost, 0, len(candidates))
	for _, candidate := range candidates {
		ordered = append(ordered, candidate)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].CreatedAt == ordered[j].CreatedAt {
			return ordered[i].PostID > ordered[j].PostID
		}
		return ordered[i].CreatedAt > ordered[j].CreatedAt
	})
	if len(ordered) > FeedPageSize {
		ordered = ordered[:FeedPageSize]
	}

	page := FeedPage{Posts: make([]*pb.GetPostResponse, 0, len(ordered))}
	for _, candidate := range ordered {
		post := postData[candidate.PostID]
		if post == nil {
			post, err = s.posts.GetPost(ctx, &pb.GetPostRequest{PostId: candidate.PostID})
			if err != nil {
				return FeedPage{}, fmt.Errorf("load feed post %q: %w", candidate.PostID, err)
			}
		}
		if post.GetUserId() != candidate.UserID {
			return FeedPage{}, fmt.Errorf("feed cache author mismatch for post %q", candidate.PostID)
		}
		page.Posts = append(page.Posts, post)
	}
	if len(page.Posts) == FeedPageSize {
		last := page.Posts[len(page.Posts)-1]
		page.NextCursor = FeedCursor{CreatedAt: last.GetCreatedAt(), PostID: last.GetPostId()}
	}
	return page, nil
}

func idSet(ids []string) map[string]struct{} {
	set := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set
}

func isBefore(post CachedPost, cursor FeedCursor) bool {
	if cursor.CreatedAt == 0 {
		return true
	}
	if post.CreatedAt != cursor.CreatedAt {
		return post.CreatedAt < cursor.CreatedAt
	}
	return cursor.PostID != "" && post.PostID < cursor.PostID
}

func countCandidates(posts map[string]CachedPost, users map[string]struct{}) int {
	count := 0
	for _, post := range posts {
		if _, included := users[post.UserID]; included {
			count++
		}
	}
	return count
}
