package user_service

import (
	"database/sql"
	"errors"
	"go_distributed_social_media/internal/common/errs"
	"uuid"
)

type Repository interface {
	Create(dto CreateUserDTO) error
	Get(id string) (User, error)
	GetByEmail(email string) (User, error)
}

type SQLRepo struct {
	db *sql.DB
}

func NewSQLRepo(db *sql.DB) *SQLRepo {
	return &SQLRepo{
		db: db,
	}
}

func (r *SQLRepo) GetByEmail(email string) (User, error) {
	row := r.db.QueryRow("SELECT * FROM users WHERE email = ?", email)
	var user User

	err := row.Scan(&user.ID, &user.Name, &user.Email, &user.Password)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, &errs.Err{Err: errs.NotFound}
		}
		return User{}, err
	}
	return user, nil
}

func (r *SQLRepo) Get(id string) (User, error) {
	row := r.db.QueryRow("SELECT * FROM users WHERE id = ?", id)
	var user User

	err := row.Scan(&user.ID, &user.Name, &user.Email, &user.Password)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, &errs.Err{Err: errs.NotFound}
		}
		return User{}, err
	}
	return user, nil
}

func (r *SQLRepo) Create(dto CreateUserDTO) error {
	_, err := r.db.Exec("INSERT INTO users (id, name, email, password) VALUES (?, ?, ?, ?)",
		uuid.New().String(), dto.Name, dto.Email, dto.Password)
	if err != nil {
		return err
	}
	return nil
}
