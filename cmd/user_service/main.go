package main

import (
	"go_distributed_social_media/api/pb"
	"go_distributed_social_media/internal/common/config"
	"go_distributed_social_media/internal/common/database"
	"go_distributed_social_media/internal/common/database/user_db_migrations"
	"go_distributed_social_media/internal/user_service"
	"log"
	"net"

	"google.golang.org/grpc"
)

func main() {
	db, err := database.NewDB(config.Required(config.UserDBName),
		config.Required(config.UserDBHost),
		config.Required(config.UserDBPort))
	if err != nil {
		log.Fatal(err)
	}
	user_db_migrations.MigrateUserDB(db)


	server := grpc.NewServer()
	listener, err := net.Listen("tcp", ":"+config.Required(config.UserSrvPort))
	if err != nil {
		log.Fatal(err)
	}

	repo := user_service.NewSQLRepo(db)
	service := user_service.NewService(repo)
	pb.RegisterUserServiceServer(server, user_service.NewHandler(service))

	err = server.Serve(listener)
	if err != nil {
		log.Fatal(err)
	}
}
