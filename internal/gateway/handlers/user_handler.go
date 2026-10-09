package handlers

import (
	"context"
	"go_distributed_social_media/api/pb"
	"go_distributed_social_media/internal/gateway"
	"net/http"

	"github.com/gorilla/mux"
	"google.golang.org/grpc"
)

type UserHandler struct {
	router     *mux.Router
	userClient pb.UserServiceClient
}

func NewUserHandler(router *mux.Router, grpcClient pb.UserServiceClient) *UserHandler {
	return &UserHandler{
		router:     router,
		userClient: grpcClient,
	}
}

func (h *UserHandler) RegisterRoutes() {
	h.router.HandleFunc("/profile", h.handleProfile).Methods("GET")
}

func (h *UserHandler) handleProfile(w http.ResponseWriter, r *http.Request) {
	gateway.Forward(w, r, &pb.GetUserRequest{}, func(ctx context.Context, req *pb.GetUserRequest, opts ...grpc.CallOption) (*pb.User, error) {
		return h.userClient.GetUser(ctx, req)
	})
}
