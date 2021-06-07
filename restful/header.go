package restful

import (
	"crypto/md5"
	"encoding/hex"
	"net/url"
	"strings"

	"github.com/go-chassis/go-chassis/core/lager"
	"github.com/go-chassis/go-chassis/pkg/runtime"
)

const (
	Header_x_forwarded_for   = "x-forwarded-for"
	Header_x_forwarded_host  = "x-forwarded-host"
	Header_x_auth_token      = "x-auth-token"
	Header_x_sub_token       = "x-sub-token"
	Header_accept_language   = "accept-language"
	Header_app_id            = "paasport-app-id"
	Header_sub_app_id        = "paasport-sub-app-id"
	Header_device_id         = "paasport-device-id"
	Header_sub_device_id     = "paasport-sub-device-id"
	Header_device_name       = "paasport-device-name"
	Header_sub_device_name   = "paasport-sub-device-name"
	Header_region            = "paasport-region"
	Header_user_agent        = "user-agent"
	Header_client_user_agent = "client-user-agent"
	Header_auth              = "authorization"
	Header_trace             = "paasport-trace-id"
	Header_account_id        = "paasport-account-id"
	Header_terminal_type     = "paasport-terminal-type"
	Header_tenant_name       = "paasport-tenant-name"
	Header_sub_tenant_name   = "paasport-sub-tenant-name"
	Header_project_name      = "paasport-project-name"
	Header_x_b3              = "x-b3"
	Header_x_envoy           = "x-envoy"
	Header_x_request         = "x-request"
	Header_x_b3_trace_id     = "X-B3-Traceid"
	Header_referer           = "referer"
	Header_content_type      = "content-type"
	Header_method            = "paasport-request-method"
)

// 可通过的头域列表
var AllowHeaderList = []string{
	Header_x_forwarded_for,
	Header_x_forwarded_host,
	Header_x_auth_token,
	Header_x_sub_token,
	Header_app_id,
	Header_sub_app_id,
	Header_device_id,
	Header_sub_device_id,
	Header_device_name,
	Header_sub_device_name,
	Header_region,
	Header_user_agent,
	Header_auth,
	Header_accept_language,
	Header_terminal_type,
	Header_accept_language,
	Header_referer,
	Header_tenant_name,
	Header_sub_tenant_name,
}

// 需要签名的头域列表
var SignHeaderList = []string{
	Header_x_sub_token,
	Header_app_id,
	Header_device_id,
	Header_region,
	Header_sub_app_id,
	Header_sub_device_id,
	Header_terminal_type,
}

func IncommingHeaderMatcher(h string) bool {
	for _, val := range AllowHeaderList {
		if strings.ToLower(h) == val {
			return true
		}
	}
	if strings.HasPrefix(strings.ToLower(h), Header_x_b3) {
		return true
	}
	if strings.HasPrefix(strings.ToLower(h), Header_x_envoy) {
		return true
	}
	if strings.HasPrefix(strings.ToLower(h), Header_x_request) {
		return true
	}
	return false
}

func IncommingHeader(ctx *Context) map[string]string {
	lager.Logger.Debugf("incomming headers: %v", ctx.Req.Request.Header)
	var header = make(map[string]string)
	for key := range ctx.Req.Request.Header {
		if IncommingHeaderMatcher(key) {
			header[strings.ToLower(key)] = ctx.ReadHeader(key)
		}
	}

	// 注入token
	header[Header_x_auth_token] = ctx.ReadHeader(Header_x_auth_token)
	header[Header_method] = ctx.ReadHeader(Header_method)
	header[Header_trace] = ctx.ReadHeader(Header_trace)

	// 如果设备名称为空，则使用user-agent
	if ctx.ReadHeader(Header_device_name) == "" {
		header[Header_device_name] = ctx.ReadHeader(Header_user_agent)
	} else {
		// 如果device_name存在中文则解码
		deviceNameDecode, err := url.QueryUnescape(ctx.ReadHeader(Header_device_name))
		if err == nil {
			header[Header_device_name] = deviceNameDecode
		}
	}
	// 如果路径参数语言不为空
	// 则语言为路径语言
	if lang := ctx.ReadQueryParameter(LANGUAGE_QUERY_PARAM); lang != "" {
		header[Header_accept_language] = lang
	}

	// 默认前一个路由处理语言信息
	// header[Header_accept_language] = parseLang(header[Header_accept_language]).String()
	// 如果authorization头域不为空，则优先使用该头域
	if ctx.ReadHeader(Header_auth) != "" {
		header[Header_x_auth_token] = ctx.ReadHeader(Header_auth)
	}
	// 如果设备ID为空，则使用设备名称
	if header[Header_device_id] == "" && header[Header_device_name] != "" {
		header[Header_device_id] = Md5(header[Header_device_name])
	}
	// 客户端user-agent换个名字
	header[Header_client_user_agent] = ctx.ReadHeader(Header_user_agent)
	// 如果加默认值则会在签名的时候出现和客户端签名不一致的情况
	// if header[Header_sub_device_id] == "" {
	// 	header[Header_sub_device_id] = header[Header_device_id]
	// }
	// 注入当前项目APPID
	header[Header_project_name] = runtime.App
	return header
}

func Md5(data string) string {
	hash := md5.New()
	hash.Write([]byte(data))
	md := hash.Sum(nil)
	return hex.EncodeToString(md)
}
