package handlers

import (
	"context"
	"net/http"
	"strconv"

	"go_distributed_social_media/api/pb"
	"go_distributed_social_media/internal/common/utils"
	"go_distributed_social_media/internal/gateway"

	"github.com/gorilla/mux"
	"google.golang.org/grpc"
)

type FeedHandler struct {
	router     *mux.Router
	feedClient pb.FeedServiceClient
}

func NewFeedHandler(router *mux.Router, feedClient pb.FeedServiceClient) *FeedHandler {
	return &FeedHandler{router: router, feedClient: feedClient}
}

func (h *FeedHandler) RegisterRoutes() {
	h.router.HandleFunc("/feed", h.handleGetFeed).Methods(http.MethodGet)
}

func (h *FeedHandler) handleGetFeed(w http.ResponseWriter, r *http.Request) {
	request := &pb.GetFeedRequest{BeforePostId: r.URL.Query().Get("before_post_id")}
	if value := r.URL.Query().Get("before_created_at"); value != "" {
		createdAt, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			_ = utils.WriteError(w, http.StatusBadRequest, err)
			return
		}
		request.BeforeCreatedAt = createdAt
	}

	forwardedRequest := r.Clone(r.Context())
	forwardedRequest.Body = http.NoBody
	gateway.Forward(w, forwardedRequest, request,
		func(ctx context.Context, req *pb.GetFeedRequest, opts ...grpc.CallOption) (*pb.GetFeedResponse, error) {
			return h.feedClient.GetFeed(ctx, req, opts...)
		})
}
