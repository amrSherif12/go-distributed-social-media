package database

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Store struct {
	Client        *s3.Client
	PresignClient *s3.PresignClient
	Bucket        string
	PublicBaseURL string
}

func NewObjectStore(host, port, publicHost, region, accessKey, secretKey, bucket string) (*S3Store, error) {
	internalEndpoint := fmt.Sprintf("http://%s:%s", host, port)
	if publicHost == "" {
		publicHost = host
	}
	publicEndpoint := fmt.Sprintf("http://%s:%s", publicHost, port)

	awsCfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load object store config: %w", err)
	}

	newClient := func(endpoint string) *s3.Client {
		return s3.NewFromConfig(awsCfg, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true
			o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
			o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
		})
	}

	internalClient := newClient(internalEndpoint)
	presignClient := s3.NewPresignClient(newClient(publicEndpoint))

	var lastErr error
	for attempt := 0; attempt < 30; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_, err := internalClient.HeadBucket(ctx, &s3.HeadBucketInput{
			Bucket: aws.String(bucket),
		})
		cancel()
		if err == nil {
			lastErr = nil
			break
		}

		ctx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
		_, createErr := internalClient.CreateBucket(ctx, &s3.CreateBucketInput{
			Bucket: aws.String(bucket),
		})
		cancel()
		if createErr == nil {
			lastErr = nil
			break
		}

		lastErr = createErr
		time.Sleep(time.Second)
	}
	if lastErr != nil {
		return nil, fmt.Errorf("failed to ensure bucket %s exists: %w", bucket, lastErr)
	}

	return &S3Store{
		Client:        internalClient,
		PresignClient: presignClient,
		Bucket:        bucket,
		PublicBaseURL: fmt.Sprintf("%s/%s", publicEndpoint, bucket),
	}, nil
}
