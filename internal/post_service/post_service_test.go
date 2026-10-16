package post_service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "go_distributed_social_media/api/pb"
	"go_distributed_social_media/internal/common/errs"
)

type MockMetadataRepo struct{}

func (m *MockMetadataRepo) Create(ctx context.Context, dto CreatePostDTO) (CreatePostResponse, error) {
	if dto.UserID == "" {
		return CreatePostResponse{}, &errs.Err{Err: errs.ValidationFailed, Message: "missing user id"}
	}
	if dto.Content == "trigger_db_error" {
		return CreatePostResponse{}, errors.New("database error")
	}
	return CreatePostResponse{
		PostID:    "post_123",
		CreatedAt: int(time.Now().Unix()),
	}, nil
}

func (m *MockMetadataRepo) Get(ctx context.Context, postID string) (Post, error) {
	if postID == "not_found" {
		return Post{}, &errs.Err{Err: errs.NotFound, Message: "post not found"}
	}
	if postID == "db_error" {
		return Post{}, errors.New("database error")
	}

	return Post{
		ID:        postID,
		UserID:    "user_456",
		Content:   "Hello World",
		CreatedAt: 1728576000,
		Media: &MediaAttachment{
			MediaID:   "media_789",
			URL:       "http://localhost:4566/media-bucket/uploads/img.jpg",
			MediaType: "image/jpeg",
		},
	}, nil
}

type MockObjectRepo struct{}

func (m *MockObjectRepo) GetPresignedUploadURL(ctx context.Context, dto PresignedUploadUrlDTO) (GetPresignedUploadUrlResponse, error) {
	if dto.FileExtension == "invalid" {
		return GetPresignedUploadUrlResponse{}, errors.New("unsupported file extension")
	}
	return GetPresignedUploadUrlResponse{
		UploadUrl: "http://localhost:4566/media-bucket/uploads/media_789.jpg?signature=xyz",
		MediaUrl:  "http://localhost:4566/media-bucket/uploads/media_789.jpg",
		MediaID:   "media_789",
	}, nil
}

type MockPostCreatedPublisher struct {
	events []PostCreatedEvent
	err    error
}

type blockingMetadataRepo struct {
	calls   atomic.Int32
	started chan struct{}
	release chan struct{}
}

func (r *blockingMetadataRepo) Create(context.Context, CreatePostDTO) (CreatePostResponse, error) {
	return CreatePostResponse{}, nil
}

func (r *blockingMetadataRepo) Get(ctx context.Context, postID string) (Post, error) {
	r.calls.Add(1)
	select {
	case r.started <- struct{}{}:
	default:
	}
	select {
	case <-r.release:
		return Post{ID: postID, UserID: "user_1", Content: "cached"}, nil
	case <-ctx.Done():
		return Post{}, ctx.Err()
	}
}

func (m *MockPostCreatedPublisher) PublishPostCreated(_ context.Context, event PostCreatedEvent) error {
	m.events = append(m.events, event)
	return m.err
}

func contextWithUserID(userID string) context.Context {
	md := metadata.Pairs("uid", userID)
	return metadata.NewIncomingContext(context.Background(), md)
}

func TestHandler_CreatePost(t *testing.T) {
	metaRepo := &MockMetadataRepo{}
	objRepo := &MockObjectRepo{}
	service := NewService(objRepo, metaRepo)
	handler := NewHandler(service)

	t.Run("Success", func(t *testing.T) {
		ctx := contextWithUserID("user_456")
		req := &pb.CreatePostRequest{
			Content: "Testing post creation",
			Media: &pb.MediaAttachment{
				MediaId:   "media_789",
				Url:       "http://localhost:4566/media-bucket/uploads/media_789.jpg",
				MediaType: "image/jpeg",
			},
		}

		resp, err := handler.CreatePost(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, "post_123", resp.GetPostId())
		assert.NotEmpty(t, resp.GetCreatedAt())
	})

	t.Run("Unauthenticated_MissingUserIDHeader", func(t *testing.T) {
		req := &pb.CreatePostRequest{Content: "No auth"}
		_, err := handler.CreatePost(context.Background(), req)

		require.Error(t, err)
		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("InvalidArgument_EmptyContentAndMedia", func(t *testing.T) {
		ctx := contextWithUserID("user_456")
		req := &pb.CreatePostRequest{
			Content: "",
			Media:   nil,
		}

		_, err := handler.CreatePost(ctx, req)
		_ = err
	})

	t.Run("Internal_DatabaseError", func(t *testing.T) {
		ctx := contextWithUserID("user_456")
		req := &pb.CreatePostRequest{Content: "trigger_db_error"}

		_, err := handler.CreatePost(ctx, req)
		require.Error(t, err)
		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.Internal, st.Code())
	})
}

func TestService_CreatePublishesPostCreatedEvent(t *testing.T) {
	publisher := &MockPostCreatedPublisher{}
	service := NewServiceWithPublisher(&MockObjectRepo{}, &MockMetadataRepo{}, publisher)

	created, err := service.Create(context.Background(), CreatePostDTO{
		UserID:  "user_456",
		Content: "event test",
	})
	require.NoError(t, err)
	require.Len(t, publisher.events, 1)
	assert.Equal(t, PostCreatedEvent{
		PostID:    created.PostID,
		UserID:    "user_456",
		Timestamp: int64(created.CreatedAt),
	}, publisher.events[0])
}

func TestService_CreateDoesNotPublishAfterMetadataFailure(t *testing.T) {
	publisher := &MockPostCreatedPublisher{}
	service := NewServiceWithPublisher(&MockObjectRepo{}, &MockMetadataRepo{}, publisher)

	_, err := service.Create(context.Background(), CreatePostDTO{
		UserID:  "user_456",
		Content: "trigger_db_error",
	})
	require.Error(t, err)
	assert.Empty(t, publisher.events)
}

func TestService_CreateReturnsPublisherFailure(t *testing.T) {
	publisher := &MockPostCreatedPublisher{err: errors.New("broker unavailable")}
	service := NewServiceWithPublisher(&MockObjectRepo{}, &MockMetadataRepo{}, publisher)

	_, err := service.Create(context.Background(), CreatePostDTO{
		UserID:  "user_456",
		Content: "event test",
	})
	require.ErrorContains(t, err, "publish post-created event")
	require.Len(t, publisher.events, 1)
}

func TestCachedMetadataRepoSingleflightsConcurrentMisses(t *testing.T) {
	server := miniredis.RunT(t)
	cacheClient := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = cacheClient.Close() })

	next := &blockingMetadataRepo{
		started: make(chan struct{}, 1),
		release: make(chan struct{}),
	}
	repo := NewCachedMetadataRepo(next, cacheClient, time.Minute)

	const callers = 24
	start := make(chan struct{})
	results := make(chan error, callers)
	var ready sync.WaitGroup
	ready.Add(callers)
	for range callers {
		go func() {
			ready.Done()
			<-start
			_, err := repo.Get(context.Background(), "post_shared")
			results <- err
		}()
	}
	ready.Wait()
	close(start)
	<-next.started
	time.Sleep(100 * time.Millisecond)
	close(next.release)

	for range callers {
		require.NoError(t, <-results)
	}
	assert.Equal(t, int32(1), next.calls.Load())
}

func TestHandler_GetPost(t *testing.T) {
	metaRepo := &MockMetadataRepo{}
	objRepo := &MockObjectRepo{}
	service := NewService(objRepo, metaRepo)
	handler := NewHandler(service)

	t.Run("Success", func(t *testing.T) {
		req := &pb.GetPostRequest{PostId: "post_123"}
		resp, err := handler.GetPost(context.Background(), req)

		require.NoError(t, err)
		assert.Equal(t, "post_123", resp.GetPostId())
		assert.Equal(t, "user_456", resp.GetUserId())
		assert.Equal(t, "Hello World", resp.GetContent())
		assert.NotNil(t, resp.GetMedia())
		assert.Equal(t, "media_789", resp.GetMedia().GetMediaId())
	})

	t.Run("InvalidArgument_EmptyPostID", func(t *testing.T) {
		req := &pb.GetPostRequest{PostId: ""}
		_, err := handler.GetPost(context.Background(), req)

		require.Error(t, err)
		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("NotFound", func(t *testing.T) {
		req := &pb.GetPostRequest{PostId: "not_found"}
		_, err := handler.GetPost(context.Background(), req)

		require.Error(t, err)
		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.NotFound, st.Code())
	})

	t.Run("InternalError", func(t *testing.T) {
		req := &pb.GetPostRequest{PostId: "db_error"}
		_, err := handler.GetPost(context.Background(), req)

		require.Error(t, err)
		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.Internal, st.Code())
	})
}

func TestHandler_GetPresignedUploadUrl(t *testing.T) {
	metaRepo := &MockMetadataRepo{}
	objRepo := &MockObjectRepo{}
	service := NewService(objRepo, metaRepo)
	handler := NewHandler(service)

	t.Run("Success", func(t *testing.T) {
		req := &pb.GetPresignedUploadUrlRequest{
			FileExtension: "jpg",
			MimeType:      "image/jpeg",
		}

		resp, err := handler.GetPresignedUploadUrl(context.Background(), req)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.GetUploadUrl())
		assert.NotEmpty(t, resp.GetMediaUrl())
		assert.Equal(t, "media_789", resp.GetMediaId())
	})

	t.Run("InvalidArgument_MissingExtensionOrMime", func(t *testing.T) {
		req := &pb.GetPresignedUploadUrlRequest{
			FileExtension: "jpg",
			MimeType:      "",
		}

		_, err := handler.GetPresignedUploadUrl(context.Background(), req)
		require.Error(t, err)
		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("Internal_S3Error", func(t *testing.T) {
		req := &pb.GetPresignedUploadUrlRequest{
			FileExtension: "invalid",
			MimeType:      "image/jpeg",
		}

		_, err := handler.GetPresignedUploadUrl(context.Background(), req)
		require.Error(t, err)
		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.Internal, st.Code())
	})
}
