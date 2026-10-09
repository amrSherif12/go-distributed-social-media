package user_service

import (
	"context"
	"errors"
	"go_distributed_social_media/api/pb"
	"go_distributed_social_media/internal/common/errs"
	"go_distributed_social_media/internal/common/utils"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestMain(m *testing.M) {
	os.Setenv("JWT_KEY", "yRpuW4VG75ABOKtgmaUj4xPmes7K6GHLRvysObWuzxh")
	code := m.Run()
	os.Exit(code)
}

type MockRepository struct{}

func (m *MockRepository) Create(dto CreateUserDTO) error {
	if dto.Email == "db_error@example.com" {
		return errors.New("database connection failed")
	}
	return nil
}

func (m *MockRepository) Get(id string) (User, error) {
	if id == "not_found" {
		return User{}, &errs.Err{Err: errs.NotFound, Message: "user not found"}
	}
	return User{
		ID:    id,
		Name:  "Test User",
		Email: "test@example.com",
	}, nil
}

func (m *MockRepository) GetByEmail(email string) (User, error) {
	if email == "existing@example.com" {
		hashed, _ := utils.HashPassword("password123")
		return User{
			ID:       "1",
			Name:     "Existing User",
			Email:    email,
			Password: hashed,
		}, nil
	}
	return User{}, &errs.Err{Err: errs.NotFound, Message: "not found"}
}

type mockServerTransportStream struct {
	grpc.ServerTransportStream
	header metadata.MD
}

func (m *mockServerTransportStream) Method() string {
	return "/user_service.UserService/Login"
}

func (m *mockServerTransportStream) SetHeader(md metadata.MD) error {
	if m.header == nil {
		m.header = metadata.MD{}
	}
	m.header = metadata.Join(m.header, md)
	return nil
}

func (m *mockServerTransportStream) SendHeader(md metadata.MD) error {
	return m.SetHeader(md)
}

func TestHandler_GetUser(t *testing.T) {
	repo := &MockRepository{}
	service := NewService(repo)
	handler := NewHandler(service)

	t.Run("Success", func(t *testing.T) {
		req := &pb.GetUserRequest{Id: "123"}
		resp, err := handler.GetUser(context.Background(), req)

		require.NoError(t, err)
		assert.Equal(t, "123", resp.GetId())
	})

	t.Run("NotFound", func(t *testing.T) {
		req := &pb.GetUserRequest{Id: "not_found"}
		_, err := handler.GetUser(context.Background(), req)

		require.Error(t, err)
		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.NotFound, st.Code())
	})
}

func TestHandler_Register(t *testing.T) {
	repo := &MockRepository{}
	service := NewService(repo)
	handler := NewHandler(service)

	t.Run("Success", func(t *testing.T) {
		req := &pb.RegisterRequest{
			Name:         "Amr",
			Email:        "new@example.com",
			Password:     "password123",
			PasswordConf: "password123",
		}
		_, err := handler.Register(context.Background(), req)
		assert.NoError(t, err)
	})

	t.Run("ValidationFailed", func(t *testing.T) {
		req := &pb.RegisterRequest{
			Name:     "",
			Email:    "invalid-email",
			Password: "123",
		}
		_, err := handler.Register(context.Background(), req)

		require.Error(t, err)
		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("AlreadyRegistered", func(t *testing.T) {
		req := &pb.RegisterRequest{
			Name:         "Amr",
			Email:        "existing@example.com",
			Password:     "password123",
			PasswordConf: "password123",
		}
		_, err := handler.Register(context.Background(), req)

		require.Error(t, err)
		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.AlreadyExists, st.Code())
	})
}

func TestHandler_Login(t *testing.T) {
	repo := &MockRepository{}
	service := NewService(repo)
	handler := NewHandler(service)

	t.Run("Success", func(t *testing.T) {
		req := &pb.LoginRequest{
			Email:    "existing@example.com",
			Password: "password123",
		}
		stream := &mockServerTransportStream{}
		ctx := grpc.NewContextWithServerTransportStream(context.Background(), stream)

		_, err := handler.Login(ctx, req)
		require.NoError(t, err)

		cookies := stream.header.Get("set-cookie")
		assert.NotEmpty(t, cookies)
	})

	t.Run("UnregisteredEmail", func(t *testing.T) {
		req := &pb.LoginRequest{
			Email:    "unknown@example.com",
			Password: "password123",
		}
		_, err := handler.Login(context.Background(), req)

		require.Error(t, err)
		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.NotFound, st.Code())
	})

	t.Run("WrongPassword", func(t *testing.T) {
		req := &pb.LoginRequest{
			Email:    "existing@example.com",
			Password: "wrongpassword",
		}
		_, err := handler.Login(context.Background(), req)

		require.Error(t, err)
		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.Unauthenticated, st.Code())
	})
}
