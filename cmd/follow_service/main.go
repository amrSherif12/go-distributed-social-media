package main

import (
	"context"
	"log"
	"net"
	"time"

	"go_distributed_social_media/api/pb"
	"go_distributed_social_media/internal/common/config"
	"go_distributed_social_media/internal/follow_service"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	repo, err := follow_service.NewNeo4jRepo(
		ctx,
		config.Required(config.FollowDBHost),
		config.Required(config.FollowDBPort),
		config.Required(config.FollowDBUser),
		config.Required(config.FollowDBPass),
	)
	cancel()
	if err != nil {
		log.Fatalf("Error initializing follow database: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := repo.Close(ctx); err != nil {
			log.Print(err)
		}
	}()

	cacheClient := redis.NewClient(&redis.Options{
		Addr: net.JoinHostPort(config.Required(config.FollowCacheHost), config.Required(config.FollowCachePort)),
	})
	defer func() {
		if err := cacheClient.Close(); err != nil {
			log.Print(err)
		}
	}()

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	err = cacheClient.Ping(ctx).Err()
	cancel()
	if err != nil {
		log.Fatalf("Error connecting to follow cache: %v", err)
	}

	cachedRepo := follow_service.NewCachedRepository(repo, cacheClient, time.Minute)
	service := follow_service.NewService(cachedRepo)

	listener, err := net.Listen("tcp", ":"+config.Required(config.FollowSrvPort))
	if err != nil {
		log.Fatal(err)
	}

	server := grpc.NewServer()
	pb.RegisterFollowServiceServer(server, follow_service.NewHandler(service))
	if err := server.Serve(listener); err != nil {
		log.Fatal(err)
	}
}
