package main

import (
	"context"
	"go_distributed_social_media/api/pb"
	"go_distributed_social_media/internal/common/config"
	"go_distributed_social_media/internal/common/database"
	"go_distributed_social_media/internal/post_service"
	"log"
	"net"
	"net/url"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
)

func main() {
	metadataDB, err := database.NewNosqlDB(
		config.Required(config.PostDBHost),
		config.Required(config.PostDBPort),
		config.Required(config.PostDBUser),
		config.Required(config.PostDBPass),
	)
	if err != nil {
		log.Fatalf("Error initializing post metadata db: %s", err)
	}

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := metadataDB.Disconnect(ctx); err != nil {
			log.Print(err)
		}
	}()

	cacheHost := config.Get(config.PostCacheHost)
	cachePort := config.Get(config.PostCachePort)
	cacheClient := redis.NewClient(&redis.Options{
		Addr: net.JoinHostPort(cacheHost, cachePort),
	})
	defer func() {
		if err := cacheClient.Close(); err != nil {
			log.Print(err)
		}
	}()

	objectStore, err := database.NewObjectStore(
		config.Required(config.PostObjDBHost),
		config.Required(config.PostObjDBPort),
		config.Required(config.PostObjDBPublicHost),
		config.Required(config.PostObjDBRegion),
		config.Required(config.PostObjDBUser),
		config.Required(config.PostObjDBPass),
		config.Required(config.PostObjDBBucket),
	)
	if err != nil {
		log.Fatalf("Error initializing post object store: %v", err)
	}

	metadataRepo := post_service.NewCachedMetadataRepo(
		post_service.NewMongoRepo(metadataDB.Database(config.Required(config.PostDBName))),
		cacheClient,
		5*time.Minute,
	)
	objectRepo := post_service.NewS3ObjectRepo(
		objectStore.Client,
		objectStore.PresignClient,
		objectStore.Bucket,
		objectStore.PublicBaseURL,
	)
	rabbitURL := url.URL{
		Scheme: "amqp",
		User:   url.UserPassword(config.Required(config.PostEventBrokerUser), config.Required(config.PostEventBrokerPass)),
		Host:   net.JoinHostPort(config.Required(config.PostEventBrokerHost), config.Required(config.PostEventBrokerPort)),
		Path:   "/",
	}
	eventPublisher, err := post_service.NewRabbitMQPublisher(rabbitURL.String(), config.Required(config.PostEventQueue))
	if err != nil {
		log.Fatalf("Error initializing post event publisher: %v", err)
	}
	defer func() {
		if err := eventPublisher.Close(); err != nil {
			log.Print(err)
		}
	}()

	service := post_service.NewServiceWithPublisher(objectRepo, metadataRepo, eventPublisher)

	server := grpc.NewServer()
	listener, err := net.Listen("tcp", ":"+config.Required(config.PostSrvPort))
	if err != nil {
		log.Fatal(err)
	}

	pb.RegisterPostServiceServer(server, post_service.NewHandler(service))

	if err := server.Serve(listener); err != nil {
		log.Fatal(err)
	}
}
