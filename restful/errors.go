package restful

import (
	"net/http"

	"google.golang.org/grpc/codes"
)

const (
	INTERNAL_ERR             = 10401 // 服务器内部错误
	HEADER_MISSING_ERR       = 10402 //缺少头域
	TOKEN_ISEMPTY_ERR        = 10403 // token为空
	DECODE_TOKEN_FAIL        = 10404 // token解码失败
	PARSE_TOKEN_ERR          = 10405 // token解析失败
	DECODE_CLAIM_FAIL        = 10406 // token claim解码失败
	PARSE_CLAIM_FAIL         = 10407 // token claim解析失败
	INVALID_ERR_FORMAT_ERR   = 10408 // 无效的错误格式
	INVALID_PATH_ARG_ERR     = 10409 // 无效的路径参数
	MAINTENANCE_ERR          = 10410 // 服务维护中
	INVALID_GRAPHQL_BODY_ERR = 10411 // 无效的graphql请求体
)

// HTTPStatusFromCode converts a gRPC error code into the corresponding HTTP response status.
// See: https://github.com/googleapis/googleapis/blob/master/google/rpc/code.proto
// 如果query参数中携带ihc参数且不为空则忽略http状态码
func HTTPStatusFromCode(b *Context, code codes.Code) int {
	ignoreHttpCode := false
	if ihc := b.ReadQueryParameter(IGNORE_HTTP_CODE_PARAM); ihc != "" {
		ignoreHttpCode = true
	}
	if ignoreHttpCode {
		return http.StatusOK
	}
	switch code {
	case codes.OK:
		return http.StatusOK
	case codes.Canceled:
		return http.StatusRequestTimeout
	case codes.Unknown:
		return http.StatusInternalServerError
	case codes.InvalidArgument:
		return http.StatusBadRequest
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists:
		return http.StatusConflict
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	case codes.FailedPrecondition:
		return http.StatusPreconditionFailed
	case codes.Aborted:
		return http.StatusConflict
	case codes.OutOfRange:
		return http.StatusBadRequest
	case codes.Unimplemented:
		return http.StatusNotImplemented
	case codes.Internal:
		return http.StatusInternalServerError
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	case codes.DataLoss:
		return http.StatusInternalServerError
	}
	return http.StatusInternalServerError
}
