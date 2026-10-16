package handlers

import (
	"context"
	"go_distributed_social_media/api/pb"
	"go_distributed_social_media/internal/gateway"
	"net/http"

	"github.com/gorilla/mux"
	"google.golang.org/grpc"
)

type PostHandler struct {
	router     *mux.Router
	postClient pb.PostServiceClient
}

func NewPostHandler(router *mux.Router, grpcClient pb.PostServiceClient) *PostHandler {
	return &PostHandler{
		router:     router,
		postClient: grpcClient,
	}
}

func (h *PostHandler) RegisterRoutes() {
	h.router.HandleFunc("/posts/uploads", h.handlePresignedUploadURL).Methods("POST")
	h.router.HandleFunc("/posts", h.handleCreate).Methods("POST")
	h.router.HandleFunc("/posts/{id}", h.handleGet).Methods("GET")
}

func (h *PostHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	gateway.Forward(w, r, &pb.CreatePostRequest{}, func(ctx context.Context, req *pb.CreatePostRequest, opts ...grpc.CallOption) (*pb.CreatePostResponse, error) {
		return h.postClient.CreatePost(ctx, req, opts...)
	})
}

func (h *PostHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	gateway.Forward(w, r, &pb.GetPostRequest{PostId: mux.Vars(r)["id"]}, func(ctx context.Context, req *pb.GetPostRequest, opts ...grpc.CallOption) (*pb.GetPostResponse, error) {
		return h.postClient.GetPost(ctx, req, opts...)
	})
}

func (h *PostHandler) handlePresignedUploadURL(w http.ResponseWriter, r *http.Request) {
	gateway.Forward(w, r, &pb.GetPresignedUploadUrlRequest{}, func(ctx context.Context, req *pb.GetPresignedUploadUrlRequest, opts ...grpc.CallOption) (*pb.GetPresignedUploadUrlResponse, error) {
		return h.postClient.GetPresignedUploadUrl(ctx, req, opts...)
	})
}
