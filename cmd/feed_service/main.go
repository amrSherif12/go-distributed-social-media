package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go_distributed_social_media/api/pb"
	"go_distributed_social_media/internal/common/config"
	"go_distributed_social_media/internal/feed_service"
	"go_distributed_social_media/internal/gateway"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cacheClient := redis.NewClient(&redis.Options{
		Addr: net.JoinHostPort(config.Required(config.FeedCacheHost), config.Required(config.FeedCachePort)),
	})
	defer func() {
		if err := cacheClient.Close(); err != nil {
			log.Print(err)
		}
	}()
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	err := cacheClient.Ping(pingCtx).Err()
	cancel()
	if err != nil {
		return fmt.Errorf("connect to feed cache: %w", err)
	}

	followConn, err := gateway.Dial(config.Required(config.FollowSrvHost), config.Required(config.FollowSrvPort))
	if err != nil {
		return fmt.Errorf("connect to follow service: %w", err)
	}
	defer func() { _ = followConn.Close() }()
	postConn, err := gateway.Dial(config.Required(config.PostSrvHost), config.Required(config.PostSrvPort))
	if err != nil {
		return fmt.Errorf("connect to post service: %w", err)
	}
	defer func() { _ = postConn.Close() }()

	rabbitURL := url.URL{
		Scheme: "amqp",
		User: url.UserPassword(
			config.Required(config.PostEventBrokerUser),
			config.Required(config.PostEventBrokerPass),
		),
		Host: net.JoinHostPort(config.Required(config.PostEventBrokerHost), config.Required(config.PostEventBrokerPort)),
		Path: "/",
	}
	consumer, err := feed_service.NewRabbitMQConsumer(rabbitURL.String(), config.Required(config.PostEventQueue))
	if err != nil {
		return fmt.Errorf("initialize post event consumer: %w", err)
	}
	defer func() {
		if err := consumer.Close(); err != nil {
			log.Print(err)
		}
	}()

	service := feed_service.NewService(
		pb.NewFollowServiceClient(followConn),
		pb.NewPostServiceClient(postConn),
		feed_service.NewCache(cacheClient),
	)
	listener, err := net.Listen("tcp", ":"+config.Required(config.FeedSrvPort))
	if err != nil {
		return fmt.Errorf("listen for feed RPCs: %w", err)
	}
	defer func() { _ = listener.Close() }()

	server := grpc.NewServer()
	pb.RegisterFeedServiceServer(server, feed_service.NewHandler(service))
	serverErrors := make(chan error, 1)
	consumerErrors := make(chan error, 1)
	go func() { serverErrors <- server.Serve(listener) }()
	go func() { consumerErrors <- consumer.Run(ctx, service.HandlePostCreated) }()

	select {
	case <-ctx.Done():
		server.GracefulStop()
		return nil
	case err := <-serverErrors:
		return fmt.Errorf("serve feed RPCs: %w", err)
	case err := <-consumerErrors:
		if err == nil {
			return fmt.Errorf("post event consumer stopped unexpectedly")
		}
		return fmt.Errorf("consume post-created events: %w", err)
	}
}
