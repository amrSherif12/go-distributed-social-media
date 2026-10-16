package follow_service

import (
	"context"
	"strings"

	"go_distributed_social_media/internal/common/errs"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Follow(ctx context.Context, followerID, followeeID string) (bool, error) {
	if err := validateUserID(followerID); err != nil {
		return false, err
	}
	if err := validateUserID(followeeID); err != nil {
		return false, err
	}
	if followerID == followeeID {
		return false, &errs.Err{Err: errs.ValidationFailed, Message: "users cannot follow themselves"}
	}
	return s.repo.Follow(ctx, followerID, followeeID)
}

func (s *Service) Unfollow(ctx context.Context, followerID, followeeID string) (bool, error) {
	if err := validateUserID(followerID); err != nil {
		return false, err
	}
	if err := validateUserID(followeeID); err != nil {
		return false, err
	}
	if followerID == followeeID {
		return false, &errs.Err{Err: errs.ValidationFailed, Message: "users cannot unfollow themselves"}
	}
	return s.repo.Unfollow(ctx, followerID, followeeID)
}

func (s *Service) GetFollows(ctx context.Context, userID string) ([]string, error) {
	if err := validateUserID(userID); err != nil {
		return nil, err
	}
	return s.repo.GetFollows(ctx, userID)
}

func (s *Service) GetFollowers(ctx context.Context, userID string) ([]string, error) {
	if err := validateUserID(userID); err != nil {
		return nil, err
	}
	return s.repo.GetFollowers(ctx, userID)
}

func (s *Service) IsCelebrity(ctx context.Context, userID string) (bool, error) {
	if err := validateUserID(userID); err != nil {
		return false, err
	}
	followers, err := s.repo.GetFollowers(ctx, userID)
	if err != nil {
		return false, err
	}
	return len(followers) >= CelebrityFollowerThreshold, nil
}

func (s *Service) GetCelebrityFollows(ctx context.Context, userID string) ([]string, error) {
	if err := validateUserID(userID); err != nil {
		return nil, err
	}
	return s.repo.GetCelebrityFollows(ctx, userID)
}

func validateUserID(userID string) error {
	if strings.TrimSpace(userID) == "" {
		return &errs.Err{Err: errs.ValidationFailed, Message: "missing user id"}
	}
	return nil
}
