package restful

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"

	"github.com/go-chassis/go-chassis/core/lager"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var outHead = map[string]string{
	"Token": Header_x_auth_token,
}

type errBody struct {
	Code          int32  `json:"code"`           // 返回码
	ErrCode       int    `json:"err_code"`       // 错误码
	Message       string `json:"message"`        // 错误信息
	RequestId     string `json:"request_id"`     // 请求ID
	RequestMethod string `json:"request_method"` // 请求方法
}

type RespBody struct {
	ErrCode       int         `json:"errCode"`        // 错误码
	Message       string      `json:"errMessage"`     // 错误信息
	Status        int         `json:"status"`         // http状态码
	Data          interface{} `json:"data,omitempty"` // 返回数据
	RequestId     string      `json:"request_id"`     // 请求ID
	RequestMethod string      `json:"request_method"` // 请求方法
	Success       bool        `json:"success"`
}

/*
 格式化错误
 错误格式为:
 	(错误码)错误概要
 例如:
	 (10401)internal server error
*/
func formatError(err error) (codes.Code, int, error) {
	statusCode, errCode, perr := ParseError(err)
	if perr != nil {
		return statusCode, errCode, grpc.Errorf(statusCode, perr.Error())
	}
	return statusCode, errCode, perr
}

func ParseError(err error) (codes.Code, int, error) {
	// 是否能解析错误
	s, ok := status.FromError(err)
	if !ok {
		s = status.New(codes.InvalidArgument, err.Error())
	}
	if s.Code() == codes.OK {
		return codes.OK, 0, nil
	}
	// 是否是指定的错误格式
	rex := regexp.MustCompile(`\(([^)]+)\)`)
	out := rex.FindAllStringSubmatch(s.Message(), -1)
	errCode := 0
	statusCode := s.Code()
	message := s.Message()
	if len(out) >= 1 && len(out[0]) >= 2 {
		errCode, _ = strconv.Atoi(out[0][1])
	}
	if errCode == 0 {
		message = fmt.Sprintf("(%d)%s", INVALID_ERR_FORMAT_ERR, message)
		statusCode = codes.InvalidArgument
		errCode = INVALID_ERR_FORMAT_ERR
	}
	return statusCode, errCode, fmt.Errorf(message)
}

/*
	如果query参数中携带onebox参数且不为空则返回消息体和错误消息体一并返回
*/
func Response(b *Context, resp interface{}, err error) {
	lager.Logger.Debugf("response: %v", resp)
	// 将指定字段解析到头域中
	t := reflect.TypeOf(resp).Elem()
	value := reflect.ValueOf(resp).Elem()
	for k, v := range outHead {
		for i := 0; i < t.NumField(); i++ {
			if k == t.Field(i).Name {
				b.AddHeader(v, value.Field(i).Interface().(string))
				b.AddHeader(Header_auth, value.Field(i).Interface().(string))
			}
		}
	}
	isonebox := true
	lager.Logger.Debugf("get onebox parameter '%s'", b.ReadQueryParameter(BODY_INONEBOX_PARAM))
	if onebox := b.ReadQueryParameter(BODY_INONEBOX_PARAM); onebox == "" {
		isonebox = false
	}
	statusCode, errCode, formatErr := formatError(err)
	httpCode := HTTPStatusFromCode(b, statusCode)
	respBody := RespBody{
		Message:       "SUCCESS",
		Data:          resp,
		Status:        httpCode,
		RequestId:     b.ReadResponseWriter().Header().Get(Header_trace),
		RequestMethod: b.ReadResponseWriter().Header().Get(Header_method),
		Success:       true,
	}
	if err != nil {
		lager.Logger.Errorf("request on '%s' with method '%s' got error[%s]",
			b.ReadRequest().URL.String(),
			b.ReadRequest().Method,
			err.Error())
		respBody.ErrCode = errCode
		respBody.Message = status.Convert(formatErr).Message()
		respBody.Success = false
		if !isonebox {
			b.WriteHeaderAndJSON(HTTPStatusFromCode(b, statusCode),
				errBody{
					Code:          int32(statusCode),
					ErrCode:       respBody.ErrCode,
					Message:       respBody.Message,
					RequestId:     b.ReadResponseWriter().Header().Get(Header_trace),
					RequestMethod: b.ReadResponseWriter().Header().Get(Header_method),
				},
				"application/json;charset=utf-8")
			return
		}
	}
	if isonebox {
		b.WriteHeaderAndJSON(httpCode,
			respBody,
			"application/json;charset=utf-8")
		return
	}
	b.WriteHeaderAndJSON(httpCode,
		resp,
		"application/json;charset=utf-8")
	return
}
