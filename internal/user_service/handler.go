package user_service

import (
	"context"
	"go_distributed_social_media/api/pb"
	"go_distributed_social_media/internal/common/utils"
	"net/http"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type Handler struct {
	pb.UnimplementedUserServiceServer
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
	}
}

func (h *Handler) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
	id := req.GetId()
	if id == "" {
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if uids := md.Get("uid"); len(uids) > 0 {
				id = uids[0]
			}
		}
	}

	usr, err := h.service.Get(id)
	if err != nil {
		return nil, utils.ParseError(err)
	}

	return &pb.User{
		Id:    usr.ID,
		Name:  usr.Name,
		Email: usr.Email,
	}, nil
}

func (h *Handler) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.EmptyResponse, error) {
	dto := RegisterDTO{
		Name:     req.GetName(),
		Email:    req.GetEmail(),
		Password: req.GetPassword(),
		PassConf: req.GetPasswordConf(),
	}

	if err := h.service.Register(dto); err != nil {
		return nil, utils.ParseError(err)
	}

	return &pb.EmptyResponse{}, nil
}

func (h *Handler) Login(ctx context.Context, req *pb.LoginRequest) (*pb.EmptyResponse, error) {
	dto := LoginDTO{
		Email:    req.GetEmail(),
		Password: req.GetPassword(),
	}

	token, err := h.service.Login(dto)
	if err != nil {
		return nil, utils.ParseError(err)
	}

	cookie := &http.Cookie{
		Name:     "auth",
		Value:    token,
		HttpOnly: true,
		Path:     "/",
	}

	header := metadata.Pairs("Set-Cookie", cookie.String())
	if err := grpc.SendHeader(ctx, header); err != nil {
		return nil, utils.ParseError(err)
	}

	return &pb.EmptyResponse{}, nil
}
