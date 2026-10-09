-- +goose Up
CREATE TABLE users
(
    id       BINARY(36)   NOT NULL PRIMARY KEY,
    name     VARCHAR(20)  NOT NULL,
    email    VARCHAR(255) NOT NULL UNIQUE,
    password VARCHAR(60)  NOT NULL
);

-- +goose Down
DROP TABLE users;
