FROM golang:1.27-alpine AS builder
WORKDIR /go_distributed_social_media

ARG SRV_NAME

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o service ./cmd/${SRV_NAME}/

FROM alpine:latest
WORKDIR /go_distributed_social_media

COPY --from=builder /go_distributed_social_media/service .

CMD ["./service"]

