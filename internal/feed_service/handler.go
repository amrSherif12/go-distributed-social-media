package feed_service

import (
	"context"
	"strings"

	"go_distributed_social_media/api/pb"
	"go_distributed_social_media/internal/common/errs"
	"go_distributed_social_media/internal/common/utils"

	"google.golang.org/grpc/metadata"
)

type Handler struct {
	pb.UnimplementedFeedServiceServer
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) GetFeed(ctx context.Context, request *pb.GetFeedRequest) (*pb.GetFeedResponse, error) {
	userID := userIDFromContext(ctx)
	if strings.TrimSpace(userID) == "" {
		return nil, utils.ParseError(&errs.Err{Err: errs.ValidationFailed, Message: "missing user id"})
	}
	page, err := h.service.GetFeed(ctx, userID, FeedCursor{
		CreatedAt: request.GetBeforeCreatedAt(),
		PostID:    request.GetBeforePostId(),
	})
	if err != nil {
		return nil, utils.ParseError(err)
	}
	return &pb.GetFeedResponse{
		Posts:               page.Posts,
		NextBeforeCreatedAt: page.NextCursor.CreatedAt,
		NextBeforePostId:    page.NextCursor.PostID,
	}, nil
}

func userIDFromContext(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	userIDs := md.Get("uid")
	if len(userIDs) == 0 {
		return ""
	}
	return userIDs[0]
}
