package main

import (
	"go_distributed_social_media/api/pb"
	"go_distributed_social_media/internal/common/config"
	"go_distributed_social_media/internal/gateway"
	"go_distributed_social_media/internal/gateway/handlers"
	"go_distributed_social_media/internal/gateway/middlewares"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

func main() {
	router := mux.NewRouter()

	userClient, err := gateway.Init(config.Required(config.UserSrvHost),
		config.Required(config.UserSrvPort), pb.NewUserServiceClient)
	if err != nil {
		log.Fatalf("failed to connect to user service: %v", err)
	}

	postClient, err := gateway.Init(config.Required(config.PostSrvHost),
		config.Required(config.PostSrvPort), pb.NewPostServiceClient)
	if err != nil {
		log.Fatalf("failed to connect to post service: %v", err)
	}

	followClient, err := gateway.Init(config.Required(config.FollowSrvHost),
		config.Required(config.FollowSrvPort), pb.NewFollowServiceClient)
	if err != nil {
		log.Fatalf("failed to connect to follow service: %v", err)
	}

	router.Use(middlewares.IPRateLimiterMiddleware)
	handlers.NewAuthHandler(router, userClient).RegisterRoutes()

	protected := router.PathPrefix("/").Subrouter()
	protected.Use(middlewares.AuthMiddleware)
	protected.Use(middlewares.UIDRateLimiterMiddleware)

	handlers.NewUserHandler(protected, userClient).RegisterRoutes()
	handlers.NewPostHandler(protected, postClient).RegisterRoutes()
	handlers.NewFollowHandler(protected, followClient).RegisterRoutes()

	err = http.ListenAndServe(":"+config.Required(config.GatewayPort), router)
	if err != nil {
		log.Fatal(err)
	}
}
