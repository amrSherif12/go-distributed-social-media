package user_service

import (
	"go_distributed_social_media/internal/common/config"
	"go_distributed_social_media/internal/common/errs"
	"go_distributed_social_media/internal/common/utils"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Service struct {
	userRepo Repository
}

func NewService(repo Repository) *Service {
	return &Service{
		userRepo: repo,
	}
}

func (s *Service) Register(dto RegisterDTO) error {
	if err := utils.Validate.Struct(dto); err != nil {
		return &errs.Err{Err: errs.ValidationFailed,
			Message: utils.FormatError(err)}
	}

	_, err := s.userRepo.GetByEmail(dto.Email)
	if err == nil {
		return &errs.Err{Err: errs.AuthEmailAlreadyRegistered,
			Message: "email already registered"}
	}

	if !errs.Is(err, errs.NotFound) {
		return err
	}

	hashPass, err := utils.HashPassword(dto.Password)
	if err != nil {
		return err
	}

	return s.userRepo.Create(CreateUserDTO{
		Name:     dto.Name,
		Email:    dto.Email,
		Password: hashPass,
	})
}

func (s *Service) Login(dto LoginDTO) (string, error) {
	if err := utils.Validate.Struct(dto); err != nil {
		return "", &errs.Err{Err: errs.ValidationFailed,
			Message: utils.FormatError(err)}
	}

	usr, err := s.userRepo.GetByEmail(dto.Email)
	if err != nil {
		if errs.Is(err, errs.NotFound) {
			return "", &errs.Err{Err: errs.AuthUnregisteredEmail,
				Message: "unregistered email"}
		}
		return "", err
	}
	if !utils.CheckPasswordHash(dto.Password, usr.Password) {
		return "", &errs.Err{Err: errs.AuthWrongPassword,
			Message: "wrong password"}
	}

	t := jwt.NewWithClaims(jwt.SigningMethodHS256,
		jwt.MapClaims{
			"sub": usr.ID,
			"exp": time.Now().Add(time.Hour * 24 * 30).Unix(),
		})

	key := []byte(config.Required(config.JWTKey))
	tokenStr, err := t.SignedString(key)

	if err != nil {
		return "", err
	}

	return tokenStr, nil
}

func (s *Service) Get(id string) (User, error) {
	usr, err := s.userRepo.Get(id)
	if err != nil {
		if errs.Is(err, errs.NotFound) {
			return User{}, &errs.Err{Err: errs.NotFound, Message: "user not found"}
		}
		return User{}, err
	}
	return usr, nil
}
