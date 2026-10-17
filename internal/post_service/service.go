package post_service

import (
	"context"
	"fmt"
	"go_distributed_social_media/internal/common/errs"
	"strings"
)

type PostCreatedEvent struct {
	PostID    string `json:"post_id"`
	UserID    string `json:"uid"`
	Timestamp int64  `json:"timestamp"`
}

type PostCreatedPublisher interface {
	PublishPostCreated(ctx context.Context, event PostCreatedEvent) error
}

type Service struct {
	objRepo   ObjectRepo
	metaRepo  MetadataRepo
	publisher PostCreatedPublisher
}

func NewService(objRepo ObjectRepo, metaRepo MetadataRepo) *Service {
	return &Service{
		objRepo:  objRepo,
		metaRepo: metaRepo,
	}
}

func NewServiceWithPublisher(objRepo ObjectRepo, metaRepo MetadataRepo, publisher PostCreatedPublisher) *Service {
	service := NewService(objRepo, metaRepo)
	service.publisher = publisher
	return service
}

func (s *Service) Create(ctx context.Context, dto CreatePostDTO) (CreatePostResponse, error) {
	if dto.UserID == "" {
		return CreatePostResponse{}, &errs.Err{Err: errs.ValidationFailed, Message: "missing user id"}
	}
	created, err := s.metaRepo.Create(ctx, dto)
	if err != nil {
		return CreatePostResponse{}, err
	}
	if s.publisher != nil {
		err := s.publisher.PublishPostCreated(ctx, PostCreatedEvent{
			PostID:    created.PostID,
			UserID:    dto.UserID,
			Timestamp: int64(created.CreatedAt),
		})
		if err != nil {
			return CreatePostResponse{}, fmt.Errorf("publish post-created event: %w", err)
		}
	}
	return created, nil
}

func (s *Service) Get(ctx context.Context, postID string) (Post, error) {
	if postID == "" {
		return Post{}, &errs.Err{Err: errs.ValidationFailed, Message: "missing post id"}
	}
	return s.metaRepo.Get(ctx, postID)
}

func (s *Service) GetByUsers(ctx context.Context, dto GetPostsByUsersDTO) ([]Post, error) {
	if dto.BeforeCreatedAt < 0 {
		return nil, &errs.Err{Err: errs.ValidationFailed, Message: "before_created_at cannot be negative"}
	}
	if dto.BeforeCreatedAt == 0 && dto.BeforePostID != "" {
		return nil, &errs.Err{Err: errs.ValidationFailed, Message: "before_post_id requires before_created_at"}
	}

	seen := make(map[string]struct{}, len(dto.UserIDs))
	userIDs := make([]string, 0, len(dto.UserIDs))
	for _, userID := range dto.UserIDs {
		userID = strings.TrimSpace(userID)
		if userID == "" {
			return nil, &errs.Err{Err: errs.ValidationFailed, Message: "user ids cannot be empty"}
		}
		if _, exists := seen[userID]; exists {
			continue
		}
		seen[userID] = struct{}{}
		userIDs = append(userIDs, userID)
	}
	if len(userIDs) == 0 {
		return []Post{}, nil
	}

	return s.metaRepo.GetByUsers(ctx, GetPostsByUsersDTO{
		UserIDs:         userIDs,
		BeforeCreatedAt: dto.BeforeCreatedAt,
		BeforePostID:    dto.BeforePostID,
	})
}

func (s *Service) GetPresignedUploadURL(ctx context.Context, dto PresignedUploadUrlDTO) (GetPresignedUploadUrlResponse, error) {
	if dto.MimeType == "" {
		return GetPresignedUploadUrlResponse{}, &errs.Err{Err: errs.ValidationFailed, Message: "missing mime type"}
	}
	return s.objRepo.GetPresignedUploadURL(ctx, dto)
}
