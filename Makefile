-include deployment/.env
export

SHELL := cmd.exe

.PHONY: proto proto-user proto-post dock-user-srv dock-gateway goose-user-db\
 compose compose-dow compose-ps it-test it-test-b it-test-log compose-b compose-log

define docker-build
	docker build -t $(1) --build-arg SRV_NAME=$(1) ./deployment
endef

define goose-create
	goose -dir "./internal/common/database/$(1)" -s create $(name) sql
endef

define integration
    docker compose -f .\deployment\docker-compose.yaml -f .\deployment\docker-compose.test.yaml down -v
	docker compose -f .\deployment\docker-compose.yaml -f .\deployment\docker-compose.test.yaml $(1)
    go test -v -tags=integration ./tests/integration/
    docker compose -f .\deployment\docker-compose.yaml -f .\deployment\docker-compose.test.yaml down -v
endef

define proto
	@if not exist api\pb mkdir api\pb
	protoc --proto_path=api/protobuf --go_out=api/pb --go_opt=paths=source_relative\
 	--go-grpc_out=api/pb --go-grpc_opt=paths=source_relative api/protobuf/$(1).proto
endef

proto-user:
	$(call proto,user)

proto-post:
	$(call proto,post)

goose-user-db:
	$(call goose-create,user_db_migrations)

compose:
	docker compose -f .\deployment\docker-compose.yaml up -d

compose-down:
	docker compose -f .\deployment\docker-compose.yaml down

compose-b:
	docker compose -f .\deployment\docker-compose.yaml up -d --build

compose-log:
	docker compose -f .\deployment\docker-compose.yaml logs

compose-ps:
	docker compose -f .\deployment\docker-compose.yaml ps

it-test:
	$(call integration,up -d --wait)

it-test-b:
	$(call integration,up -d --build --wait)

it-test-log:
	docker compose -f .\deployment\docker-compose.yaml -f .\deployment\docker-compose.test.yaml logs
