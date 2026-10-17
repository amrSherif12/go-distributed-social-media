package follow_service

import (
	"context"

	"go_distributed_social_media/api/pb"
	"go_distributed_social_media/internal/common/errs"
	"go_distributed_social_media/internal/common/utils"

	"google.golang.org/grpc/metadata"
)

type Handler struct {
	pb.UnimplementedFollowServiceServer
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) FollowUser(ctx context.Context, req *pb.FollowUserRequest) (*pb.FollowActionResponse, error) {
	followerID := uidFromContext(ctx)
	if followerID == "" {
		return nil, utils.ParseError(&errs.Err{Err: errs.ValidationFailed, Message: "missing user id"})
	}
	changed, err := h.service.Follow(ctx, followerID, req.GetFolloweeId())
	if err != nil {
		return nil, utils.ParseError(err)
	}
	return &pb.FollowActionResponse{Changed: changed}, nil
}

func (h *Handler) UnfollowUser(ctx context.Context, req *pb.FollowUserRequest) (*pb.FollowActionResponse, error) {
	followerID := uidFromContext(ctx)
	if followerID == "" {
		return nil, utils.ParseError(&errs.Err{Err: errs.ValidationFailed, Message: "missing user id"})
	}
	changed, err := h.service.Unfollow(ctx, followerID, req.GetFolloweeId())
	if err != nil {
		return nil, utils.ParseError(err)
	}
	return &pb.FollowActionResponse{Changed: changed}, nil
}

func (h *Handler) GetFollows(ctx context.Context, req *pb.GetFollowsRequest) (*pb.UserListResponse, error) {
	userIDs, err := h.service.GetFollows(ctx, req.GetUserId())
	if err != nil {
		return nil, utils.ParseError(err)
	}
	return &pb.UserListResponse{UserIds: userIDs}, nil
}

func (h *Handler) GetFollowers(ctx context.Context, req *pb.GetFollowersRequest) (*pb.UserListResponse, error) {
	userIDs, err := h.service.GetFollowers(ctx, req.GetUserId())
	if err != nil {
		return nil, utils.ParseError(err)
	}
	return &pb.UserListResponse{UserIds: userIDs}, nil
}

func (h *Handler) IsCelebrity(ctx context.Context, req *pb.UserRequest) (*pb.CelebrityResponse, error) {
	isCelebrity, err := h.service.IsCelebrity(ctx, req.GetUserId())
	if err != nil {
		return nil, utils.ParseError(err)
	}
	return &pb.CelebrityResponse{IsCelebrity: isCelebrity}, nil
}

func (h *Handler) GetCelebrityFollows(ctx context.Context, req *pb.UserRequest) (*pb.UserListResponse, error) {
	userIDs, err := h.service.GetCelebrityFollows(ctx, req.GetUserId())
	if err != nil {
		return nil, utils.ParseError(err)
	}
	return &pb.UserListResponse{UserIds: userIDs}, nil
}

func (h *Handler) GetNonCelebrityFollows(ctx context.Context, req *pb.UserRequest) (*pb.UserListResponse, error) {
	userIDs, err := h.service.GetNonCelebrityFollows(ctx, req.GetUserId())
	if err != nil {
		return nil, utils.ParseError(err)
	}
	return &pb.UserListResponse{UserIds: userIDs}, nil
}

func uidFromContext(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	uids := md.Get("uid")
	if len(uids) == 0 {
		return ""
	}
	return uids[0]
}
