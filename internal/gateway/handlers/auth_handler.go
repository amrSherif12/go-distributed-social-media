package handlers

import (
	"context"
	"go_distributed_social_media/api/pb"
	"go_distributed_social_media/internal/gateway"
	"net/http"

	"github.com/gorilla/mux"
	"google.golang.org/grpc"
)

type AuthHandler struct {
	router     *mux.Router
	userClient pb.UserServiceClient
}

func NewAuthHandler(router *mux.Router, grpcClient pb.UserServiceClient) *AuthHandler {
	return &AuthHandler{
		router:     router,
		userClient: grpcClient,
	}
}

func (h *AuthHandler) RegisterRoutes() {
	h.router.HandleFunc("/auth/login", h.handleLogin).Methods("POST")
	h.router.HandleFunc("/auth/register", h.handleRegister).Methods("POST")
}

func (h *AuthHandler) handleLogin(w http.ResponseWriter, r *http.Request) {
	gateway.Forward(w, r, &pb.LoginRequest{}, func(ctx context.Context, req *pb.LoginRequest, opts ...grpc.CallOption) (*pb.EmptyResponse, error) {
		return h.userClient.Login(ctx, req, opts...)
	})
}

func (h *AuthHandler) handleRegister(w http.ResponseWriter, r *http.Request) {
	gateway.Forward(w, r, &pb.RegisterRequest{}, func(ctx context.Context, req *pb.RegisterRequest, opts ...grpc.CallOption) (*pb.EmptyResponse, error) {
		return h.userClient.Register(ctx, req)
	})
}
