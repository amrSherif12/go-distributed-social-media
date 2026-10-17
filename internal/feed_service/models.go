package feed_service

import "go_distributed_social_media/api/pb"

const (
	FeedPageSize       = 10
	UserFeedLimit      = 50
	CelebrityPostLimit = 10
)

type PostCreatedEvent struct {
	PostID    string `json:"post_id"`
	UserID    string `json:"uid"`
	Timestamp int64  `json:"timestamp"`
}

type CachedPost struct {
	PostID    string
	UserID    string
	CreatedAt int64
}

type FeedCursor struct {
	CreatedAt int64
	PostID    string
}

type FeedPage struct {
	Posts      []*pb.GetPostResponse
	NextCursor FeedCursor
}
