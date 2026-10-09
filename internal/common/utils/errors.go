package utils

import (
	"go_distributed_social_media/internal/common/errs"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func ParseError(err error) error {
	customErr := errs.IsErr(err)
	if customErr == nil {
		return status.Error(codes.Internal, err.Error())
	}

	switch customErr.Err {
	case errs.ValidationFailed:
		return status.Error(codes.InvalidArgument, customErr.Error())
	case errs.NotFound, errs.AuthUnregisteredEmail:
		return status.Error(codes.NotFound, customErr.Error())
	case errs.AuthEmailAlreadyRegistered:
		return status.Error(codes.AlreadyExists, customErr.Error())
	case errs.AuthWrongPassword:
		return status.Error(codes.Unauthenticated, customErr.Error())
	default:
		return status.Error(codes.Internal, customErr.Error())
	}
}

func MapGRPCErrorToHTTP(w http.ResponseWriter, err error) {
	st, ok := status.FromError(err)
	if !ok {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	switch st.Code() {
	case codes.InvalidArgument:
		http.Error(w, st.Message(), http.StatusBadRequest) // 400
	case codes.NotFound:
		http.Error(w, st.Message(), http.StatusNotFound) // 404
	case codes.AlreadyExists:
		http.Error(w, st.Message(), http.StatusConflict) // 409
	case codes.Unauthenticated:
		http.Error(w, st.Message(), http.StatusUnauthorized) // 401
	case codes.PermissionDenied:
		http.Error(w, st.Message(), http.StatusForbidden) // 403
	case codes.DeadlineExceeded:
		http.Error(w, "request timed out", http.StatusGatewayTimeout) // 504
	default:
		http.Error(w, st.Message(), http.StatusInternalServerError) // 500
	}
}
