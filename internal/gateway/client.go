package gateway

import (
	"context"
	"fmt"
	"go_distributed_social_media/internal/common/utils"
	"io"
	"log"
	"net/http"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func Dial(host, port string) (*grpc.ClientConn, error) {
	target := fmt.Sprintf("dns:///%s:%s", host, port)
	serviceConfig := `{"loadBalancingConfig": [{"round_robin": {}}]}`

	return grpc.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(serviceConfig),
	)
}

func Init[T any](host, port string, newClient func(grpc.ClientConnInterface) T) (T, error) {
	conn, err := Dial(host, port)
	if err != nil {
		var zero T
		return zero, err
	}
	return newClient(conn), nil
}

func Forward[Req proto.Message, Resp proto.Message](
	w http.ResponseWriter,
	r *http.Request,
	req Req,
	grpcCall func(ctx context.Context, in Req, opts ...grpc.CallOption) (Resp, error),
) {
	ForwardWithMarshalOptions(w, r, req, grpcCall, protojson.MarshalOptions{})
}

func ForwardWithMarshalOptions[Req proto.Message, Resp proto.Message](
	w http.ResponseWriter,
	r *http.Request,
	req Req,
	grpcCall func(ctx context.Context, in Req, opts ...grpc.CallOption) (Resp, error),
	marshalOptions protojson.MarshalOptions,
) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	unmarshaler := protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}

	if len(body) > 0 {
		if err := unmarshaler.Unmarshal(body, req); err != nil {
			http.Error(w, "invalid json payload", http.StatusBadRequest)
			return
		}
	}

	ctx := r.Context()
	if uid, ok := ctx.Value("uid").(string); ok && uid != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "uid", uid)
	}

	var headerMD metadata.MD
	resp, err := grpcCall(ctx, req, grpc.Header(&headerMD))
	if err != nil {
		utils.MapGRPCErrorToHTTP(w, err)
		return
	}

	if cookies := headerMD.Get("set-cookie"); len(cookies) > 0 {
		for _, cookieStr := range cookies {
			w.Header().Add("Set-Cookie", cookieStr)
		}
	}

	out, err := marshalOptions.Marshal(resp)
	if err != nil {
		log.Print(err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(out)
}
