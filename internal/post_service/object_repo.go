package post_service

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type ObjectRepo interface {
	GetPresignedUploadURL(ctx context.Context, dto PresignedUploadUrlDTO) (GetPresignedUploadUrlResponse, error)
}

type S3ObjectRepo struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	bucketName    string
	cdnBaseURL    string
}

func NewS3ObjectRepo(client *s3.Client, presignClient *s3.PresignClient, bucketName, cdnBaseURL string) *S3ObjectRepo {
	return &S3ObjectRepo{
		client:        client,
		presignClient: presignClient,
		bucketName:    bucketName,
		cdnBaseURL:    strings.TrimRight(cdnBaseURL, "/"),
	}
}

func (s *S3ObjectRepo) GetPresignedUploadURL(ctx context.Context, dto PresignedUploadUrlDTO) (GetPresignedUploadUrlResponse, error) {
	mediaID := bson.NewObjectID().Hex()

	ext := strings.TrimSpace(dto.FileExtension)
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	objectKey := fmt.Sprintf("uploads/%s%s", mediaID, filepath.Base(ext))

	presignedReq, err := s.presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucketName),
		Key:         aws.String(objectKey),
		ContentType: aws.String(dto.MimeType),
	}, s3.WithPresignExpires(15*time.Minute))
	if err != nil {
		return GetPresignedUploadUrlResponse{}, fmt.Errorf("failed to generate presigned url: %w", err)
	}

	return GetPresignedUploadUrlResponse{
		UploadUrl: presignedReq.URL,
		MediaUrl:  fmt.Sprintf("%s/%s", s.cdnBaseURL, objectKey),
		MediaID:   mediaID,
	}, nil
}
