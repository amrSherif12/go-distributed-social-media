package handlers

import (
	"context"
	"net/http"

	"go_distributed_social_media/api/pb"
	"go_distributed_social_media/internal/gateway"

	"github.com/gorilla/mux"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
)

type FollowHandler struct {
	router       *mux.Router
	followClient pb.FollowServiceClient
}

func NewFollowHandler(router *mux.Router, grpcClient pb.FollowServiceClient) *FollowHandler {
	return &FollowHandler{router: router, followClient: grpcClient}
}

func (h *FollowHandler) RegisterRoutes() {
	h.router.HandleFunc("/users/{id}/follow", h.handleFollow).Methods("POST")
	h.router.HandleFunc("/users/{id}/follow", h.handleUnfollow).Methods("DELETE")
	h.router.HandleFunc("/users/{id}/follows", h.handleGetFollows).Methods("GET")
	h.router.HandleFunc("/users/{id}/followers", h.handleGetFollowers).Methods("GET")
}

func pathBoundRequest(r *http.Request) *http.Request {
	request := r.Clone(r.Context())
	request.Body = http.NoBody
	return request
}

func (h *FollowHandler) handleFollow(w http.ResponseWriter, r *http.Request) {
	gateway.ForwardWithMarshalOptions(w, pathBoundRequest(r), &pb.FollowUserRequest{FolloweeId: mux.Vars(r)["id"]},
		func(ctx context.Context, req *pb.FollowUserRequest, opts ...grpc.CallOption) (*pb.FollowActionResponse, error) {
			return h.followClient.FollowUser(ctx, req, opts...)
		}, protojson.MarshalOptions{EmitUnpopulated: true})
}

func (h *FollowHandler) handleUnfollow(w http.ResponseWriter, r *http.Request) {
	gateway.ForwardWithMarshalOptions(w, pathBoundRequest(r), &pb.FollowUserRequest{FolloweeId: mux.Vars(r)["id"]},
		func(ctx context.Context, req *pb.FollowUserRequest, opts ...grpc.CallOption) (*pb.FollowActionResponse, error) {
			return h.followClient.UnfollowUser(ctx, req, opts...)
		}, protojson.MarshalOptions{EmitUnpopulated: true})
}

func (h *FollowHandler) handleGetFollows(w http.ResponseWriter, r *http.Request) {
	gateway.ForwardWithMarshalOptions(w, pathBoundRequest(r), &pb.GetFollowsRequest{UserId: mux.Vars(r)["id"]},
		func(ctx context.Context, req *pb.GetFollowsRequest, opts ...grpc.CallOption) (*pb.UserListResponse, error) {
			return h.followClient.GetFollows(ctx, req, opts...)
		}, protojson.MarshalOptions{EmitUnpopulated: true})
}

func (h *FollowHandler) handleGetFollowers(w http.ResponseWriter, r *http.Request) {
	gateway.ForwardWithMarshalOptions(w, pathBoundRequest(r), &pb.GetFollowersRequest{UserId: mux.Vars(r)["id"]},
		func(ctx context.Context, req *pb.GetFollowersRequest, opts ...grpc.CallOption) (*pb.UserListResponse, error) {
			return h.followClient.GetFollowers(ctx, req, opts...)
		}, protojson.MarshalOptions{EmitUnpopulated: true})
}
