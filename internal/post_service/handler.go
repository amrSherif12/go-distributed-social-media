package post_service

import (
	"context"
	"go_distributed_social_media/api/pb"
	"go_distributed_social_media/internal/common/errs"
	"go_distributed_social_media/internal/common/utils"

	"google.golang.org/grpc/metadata"
)

type Handler struct {
	pb.UnimplementedPostServiceServer
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
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

func mediaFromPB(media *pb.MediaAttachment) *MediaAttachment {
	if media == nil {
		return nil
	}
	return &MediaAttachment{
		MediaID:   media.GetMediaId(),
		URL:       media.GetUrl(),
		MediaType: media.GetMediaType(),
	}
}

func mediaToPB(media *MediaAttachment) *pb.MediaAttachment {
	if media == nil {
		return nil
	}
	return &pb.MediaAttachment{
		MediaId:   media.MediaID,
		Url:       media.URL,
		MediaType: media.MediaType,
	}
}

func (h *Handler) CreatePost(ctx context.Context, req *pb.CreatePostRequest) (*pb.CreatePostResponse, error) {
	uid := uidFromContext(ctx)
	if uid == "" {
		return nil, utils.ParseError(&errs.Err{Err: errs.ValidationFailed, Message: "missing user id"})
	}

	resp, err := h.service.Create(ctx, CreatePostDTO{
		UserID:  uid,
		Content: req.GetContent(),
		Media:   mediaFromPB(req.GetMedia()),
	})
	if err != nil {
		return nil, utils.ParseError(err)
	}

	return &pb.CreatePostResponse{
		PostId:    resp.PostID,
		CreatedAt: int64(resp.CreatedAt),
	}, nil
}

func (h *Handler) GetPost(ctx context.Context, req *pb.GetPostRequest) (*pb.GetPostResponse, error) {
	post, err := h.service.Get(ctx, req.GetPostId())
	if err != nil {
		return nil, utils.ParseError(err)
	}

	return postToPB(post), nil
}

func (h *Handler) GetPostsByUsers(ctx context.Context, req *pb.GetPostsByUsersRequest) (*pb.GetPostsByUsersResponse, error) {
	posts, err := h.service.GetByUsers(ctx, GetPostsByUsersDTO{
		UserIDs:         req.GetUserIds(),
		BeforeCreatedAt: req.GetBeforeCreatedAt(),
		BeforePostID:    req.GetBeforePostId(),
	})
	if err != nil {
		return nil, utils.ParseError(err)
	}

	response := &pb.GetPostsByUsersResponse{Posts: make([]*pb.GetPostResponse, len(posts))}
	for i, post := range posts {
		response.Posts[i] = postToPB(post)
	}
	return response, nil
}

func postToPB(post Post) *pb.GetPostResponse {
	return &pb.GetPostResponse{
		PostId:    post.ID,
		UserId:    post.UserID,
		Content:   post.Content,
		Media:     mediaToPB(post.Media),
		CreatedAt: post.CreatedAt,
	}
}

func (h *Handler) GetPresignedUploadUrl(ctx context.Context, req *pb.GetPresignedUploadUrlRequest) (*pb.GetPresignedUploadUrlResponse, error) {
	resp, err := h.service.GetPresignedUploadURL(ctx, PresignedUploadUrlDTO{
		FileExtension: req.GetFileExtension(),
		MimeType:      req.GetMimeType(),
	})
	if err != nil {
		return nil, utils.ParseError(err)
	}

	return &pb.GetPresignedUploadUrlResponse{
		UploadUrl: resp.UploadUrl,
		MediaUrl:  resp.MediaUrl,
		MediaId:   resp.MediaID,
	}, nil
}
