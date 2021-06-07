package restful

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"reflect"
	"strings"
	"sync"

	"github.com/go-chassis/go-chassis/core/registry"
	"github.com/go-chassis/go-chassis/pkg/util/iputil"

	"github.com/go-chassis/go-archaius"
	"github.com/go-chassis/go-chassis/core/common"
	"github.com/go-chassis/go-chassis/core/invocation"
	"github.com/go-chassis/go-chassis/core/lager"
	"github.com/go-chassis/go-chassis/core/server"

	"os"
	"path/filepath"

	"github.com/emicklei/go-restful"
	globalconfig "github.com/go-chassis/go-chassis/core/config"
	"github.com/go-chassis/go-chassis/core/config/schema"
	"github.com/go-chassis/go-chassis/pkg/metrics"
	"github.com/go-chassis/go-chassis/pkg/runtime"
	swagger "github.com/go-chassis/go-restful-swagger20"
	"github.com/go-mesh/openlogging"
)

// constants for metric path and name
const (
	//Name is a variable of type string which indicates the protocol being used
	Name              = "rest"
	DefaultMetricPath = "metrics"
	MimeFile          = "application/octet-stream"
	MimeMult          = "multipart/form-data"
)

func init() {
	server.InstallPlugin(Name, NewRestfulServer)
}

// RestfulServer restful实现
type RestfulServer struct {
	microServiceName string
	container        *restful.Container
	// 根据不同的路由版本写入多个webservice
	ws     []*restful.WebService
	opts   server.Options
	mux    sync.RWMutex
	exit   chan chan error
	server *http.Server
}

// NewRestfulServer 新的restful服务初始化
func NewRestfulServer(opts server.Options) server.ProtocolServer {
	ws := new(restful.WebService)
	if archaius.GetBool("cse.metrics.enable", false) {
		metricPath := archaius.GetString("cse.metrics.apiPath", DefaultMetricPath)
		if !strings.HasPrefix(metricPath, "/") {
			metricPath = "/" + metricPath
		}
		openlogging.Info("Enabled metrics API on " + metricPath)
		ws.Route(ws.GET(metricPath).To(metrics.HTTPHandleFunc))
	}
	return &RestfulServer{
		opts:      opts,
		container: restful.NewContainer(),
		ws:        []*restful.WebService{ws},
	}
}

// HTTPRequest2Invocation convert http request to uniform invocation data format
func HTTPRequest2Invocation(req *restful.Request, schema, operation string) (*invocation.Invocation, error) {
	inv := &invocation.Invocation{
		MicroServiceName:   runtime.ServiceName,
		SourceMicroService: common.GetXCSEContext(common.HeaderSourceName, req.Request),
		Args:               req,
		Protocol:           common.ProtocolRest,
		SchemaID:           schema,
		OperationID:        operation,
		URLPathFormat:      req.Request.URL.Path,
		Metadata: map[string]interface{}{
			common.RestMethod: req.Request.Method,
		},
	}
	//set headers to Ctx, then user do not  need to consider about protocol in handlers
	m := make(map[string]string, 0)
	inv.Ctx = context.WithValue(context.Background(), common.ContextHeaderKey{}, m)
	for k := range req.Request.Header {
		m[k] = req.Request.Header.Get(k)
	}
	return inv, nil
}

// Register registe restfule server into go-chassis
func (r *RestfulServer) Register(schema interface{}, options ...server.RegisterOption) (string, error) {
	openlogging.Info("register rest server")
	opts := server.RegisterOptions{}
	r.mux.Lock()
	defer r.mux.Unlock()
	for _, o := range options {
		o(&opts)
	}

	routes, err := GetRouteSpecs(schema)
	if err != nil {
		return "", err
	}
	schemaType := reflect.TypeOf(schema)
	schemaValue := reflect.ValueOf(schema)
	var schemaName string
	tokens := strings.Split(schemaType.String(), ".")
	if len(tokens) >= 1 {
		schemaName = tokens[len(tokens)-1]
	}
	lager.Logger.Infof("schema registered is [%s]", schemaName)
	for _, route := range routes {
		handler, err := WrapHandlerChain(route, schemaType, schemaValue, schemaName, r.opts)
		if err != nil {
			return "", err
		}
		if err := r.registe2GoRestful(route, handler); err != nil {
			// if err := Register2GoRestful(route, r.ws, handler); err != nil {
			return "", err
		}
	}
	for _, ws := range r.ws {
		lager.Logger.Debugf("root path '%s' routes %+v", ws.RootPath(), ws.Routes())
	}
	return reflect.TypeOf(schema).String(), nil
}

// registe2GoRestful 注册到go-restful中
func (r *RestfulServer) registe2GoRestful(routeSpec Route, handler restful.RouteFunction) error {
	ws, err := r.getWsByRouteVersion(routeSpec.Version)
	if err != nil {
		lager.Logger.Warnf("root path '%s' webservice not found, create new", routeSpec.Version)
		ws = new(restful.WebService)
		ws = ws.ApiVersion(routeSpec.Version)
		ws = ws.Path(routeSpec.Version)
		r.ws = append(r.ws, ws)
	}
	var rb *restful.RouteBuilder
	switch routeSpec.Method {
	case http.MethodGet:
		rb = ws.GET(routeSpec.Path)
	case http.MethodPost:
		rb = ws.POST(routeSpec.Path)
	case http.MethodHead:
		rb = ws.HEAD(routeSpec.Path)
	case http.MethodPut:
		rb = ws.PUT(routeSpec.Path)
	case http.MethodPatch:
		rb = ws.PATCH(routeSpec.Path)
	case http.MethodDelete:
		rb = ws.DELETE(routeSpec.Path)
	default:
		return errors.New("method [" + routeSpec.Method + "] do not support")
	}
	rb = fillParam(routeSpec, rb)

	for _, r := range routeSpec.Returns {
		rb = rb.Returns(r.Code, r.Message, r.Model)
	}
	if routeSpec.Read != nil {
		rb = rb.Reads(routeSpec.Read)
	}

	if len(routeSpec.Consumes) > 0 {
		rb = rb.Consumes(routeSpec.Consumes...)
	} else {
		rb = rb.Consumes("*/*")
	}
	if len(routeSpec.Produces) > 0 {
		rb = rb.Produces(routeSpec.Produces...)
	} else {
		rb = rb.Produces("*/*")
	}
	ws.Route(rb.To(handler).Doc(routeSpec.FuncDesc).Operation(routeSpec.ResourceFuncName))

	return nil
}

// getWsByRouteVersion 根据不同的route版本选取不同的webservice
func (r *RestfulServer) getWsByRouteVersion(version string) (*restful.WebService, error) {
	for _, each := range r.ws {
		lager.Logger.Debugf("get root path '%s'", each.RootPath())
		if version != "" {
			if each.RootPath() == version {
				return each, nil
			}
		} else {
			if each.RootPath() == version || each.RootPath() == "/" {
				return each, nil
			}
		}
	}
	return nil, fmt.Errorf("version '%s' webservice not found", version)
}

// AddRoute 添加路由
func (r *RestfulServer) AddRoute(schema interface{}, route Route) error {
	lager.Logger.Debugf("-----add route---")
	schemaType := reflect.TypeOf(schema)
	schemaValue := reflect.ValueOf(schema)
	var schemaName string
	tokens := strings.Split(schemaType.String(), ".")
	if len(tokens) >= 1 {
		schemaName = tokens[len(tokens)-1]
	}
	handler, err := WrapHandlerChain(route, schemaType, schemaValue, schemaName, r.opts)
	if err != nil {
		return err
	}
	if err := r.registe2GoRestful(route, handler); err != nil {
		// if err := Register2GoRestful(route, r.ws, handler); err != nil {
		return err
	}
	for _, ws := range r.ws {
		lager.Logger.Debugf("root path '%s' routes %+v", ws.RootPath(), ws.Routes())
	}
	return nil
}

// DelRoute 删除路由
// @version 路由版本
// @path 路由路径
// @method 路由方式
func (r *RestfulServer) DelRoute(version, path, method string) error {
	ws, err := r.getWsByRouteVersion(version)
	if err != nil {
		return err
	}
	if err := ws.RemoveRoute(path, method); err != nil {
		return err
	}
	return nil
}

// GetRoute 获取路由
// @version 路由版本
// @path 路由路径
// @method 路由方式
func (r *RestfulServer) GetRoute(version, path, method string) (Route, error) {
	return Route{}, nil
}

// GetWebService 获取webservice
func (r *RestfulServer) GetWebService() []*restful.WebService {
	return r.ws
}

// Invocation2HTTPRequest convert invocation back to http request, set down all meta data
func Invocation2HTTPRequest(inv *invocation.Invocation, req *restful.Request) {
	for k, v := range inv.Metadata {
		req.SetAttribute(k, v.(string))
	}
	m := common.FromContext(inv.Ctx)
	for k, v := range m {
		req.Request.Header.Set(k, v)
	}

}

//Register2GoRestful register http handler to go-restful framework
// func Register2GoRestful(routeSpec Route, ws *restful.WebService, handler restful.RouteFunction) error {
// 	var rb *restful.RouteBuilder
// 	switch routeSpec.Method {
// 	case http.MethodGet:
// 		rb = ws.GET(routeSpec.Path)
// 	case http.MethodPost:
// 		rb = ws.POST(routeSpec.Path)
// 	case http.MethodHead:
// 		rb = ws.HEAD(routeSpec.Path)
// 	case http.MethodPut:
// 		rb = ws.PUT(routeSpec.Path)
// 	case http.MethodPatch:
// 		rb = ws.PATCH(routeSpec.Path)
// 	case http.MethodDelete:
// 		rb = ws.DELETE(routeSpec.Path)
// 	default:
// 		return errors.New("method [" + routeSpec.Method + "] do not support")
// 	}
// 	rb = fillParam(routeSpec, rb)

// 	for _, r := range routeSpec.Returns {
// 		rb = rb.Returns(r.Code, r.Message, r.Model)
// 	}
// 	if routeSpec.Read != nil {
// 		rb = rb.Reads(routeSpec.Read)
// 	}

// 	if len(routeSpec.Consumes) > 0 {
// 		rb = rb.Consumes(routeSpec.Consumes...)
// 	} else {
// 		rb = rb.Consumes("*/*")
// 	}
// 	if len(routeSpec.Produces) > 0 {
// 		rb = rb.Produces(routeSpec.Produces...)
// 	} else {
// 		rb = rb.Produces("*/*")
// 	}
// 	ws.Route(rb.To(handler).Doc(routeSpec.FuncDesc).Operation(routeSpec.ResourceFuncName))

// 	return nil
// }

//fillParam is for handle parameter by type
func fillParam(routeSpec Route, rb *restful.RouteBuilder) *restful.RouteBuilder {
	for _, param := range routeSpec.Parameters {
		switch param.ParamType {
		case restful.QueryParameterKind:
			rb = rb.Param(restful.QueryParameter(param.Name, param.Desc).DataType(param.DataType))
		case restful.PathParameterKind:
			rb = rb.Param(restful.PathParameter(param.Name, param.Desc).DataType(param.DataType))
		case restful.HeaderParameterKind:
			rb = rb.Param(restful.HeaderParameter(param.Name, param.Desc).DataType(param.DataType))
		case restful.BodyParameterKind:
			rb = rb.Param(restful.BodyParameter(param.Name, param.Desc).DataType(param.DataType))
		case restful.FormParameterKind:
			rb = rb.Param(restful.FormParameter(param.Name, param.Desc).DataType(param.DataType))

		}
	}
	return rb
}

// Start start restful server
func (r *RestfulServer) Start() error {
	var err error
	config := r.opts
	r.mux.Lock()
	r.opts.Address = config.Address
	r.mux.Unlock()
	for _, ws := range r.ws {
		r.container.Add(ws)
	}
	if r.opts.TLSConfig != nil {
		r.server = &http.Server{Addr: config.Address, Handler: r.container, TLSConfig: r.opts.TLSConfig}
	} else {
		r.server = &http.Server{Addr: config.Address, Handler: r.container}
	}
	// create schema
	err = r.CreateLocalSchema(config)
	if err != nil {
		return err
	}
	l, lIP, lPort, err := iputil.StartListener(config.Address, config.TLSConfig)

	if err != nil {
		return fmt.Errorf("failed to start listener: %s", err.Error())
	}

	registry.InstanceEndpoints[config.ProtocolServerName] = net.JoinHostPort(lIP, lPort)

	go func() {
		err = r.server.Serve(l)
		if err != nil {
			openlogging.Error("http server err: " + err.Error())
			server.ErrRuntime <- err
		}

	}()

	lager.Logger.Infof("Restful server listening on: %s", registry.InstanceEndpoints[config.ProtocolServerName])
	return nil
}

// CreateLocalSchema register to swagger ui,Whether to create a schema, you need to refer to the configuration.
func (r *RestfulServer) CreateLocalSchema(config server.Options) error {
	if globalconfig.GlobalDefinition.Cse.NoRefreshSchema == true {
		openlogging.Info("will not create schema file. if you want to change it, please update chassis.yaml->NoRefreshSchema=true")
		return nil
	}
	var path string
	if path = schema.GetSchemaPath(runtime.ServiceName); path == "" {
		return errors.New("schema path is empty")
	}
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("failed to generate swagger doc: %s", err.Error())
	}
	if err := os.MkdirAll(path, 0760); err != nil {
		return fmt.Errorf("failed to generate swagger doc: %s", err.Error())
	}
	swagger.LogInfo = func(format string, v ...interface{}) {
		openlogging.GetLogger().Infof(format, v...)
	}
	swaggerConfig := swagger.Config{
		WebServices:     r.container.RegisteredWebServices(),
		WebServicesUrl:  config.Address,
		ApiPath:         "/apidocs.json",
		FileStyle:       "yaml",
		SwaggerFilePath: filepath.Join(path, runtime.ServiceName+".yaml")}
	sws := swagger.RegisterSwaggerService(swaggerConfig, r.container)
	openlogging.Info("The schema has been created successfully. path:" + path)
	//set schema information when create local schema file
	err := schema.SetSchemaInfo(sws)
	if err != nil {
		return fmt.Errorf("set schema information,%s", err.Error())
	}
	return nil
}

// Stop stop restful server
func (r *RestfulServer) Stop() error {
	if r.server == nil {
		openlogging.Info("http server never started")
		return nil
	}
	//only golang 1.8 support graceful shutdown.
	if err := r.server.Shutdown(context.TODO()); err != nil {
		openlogging.Warn("http shutdown error: " + err.Error())
		return err // failure/timeout shutting down the server gracefully
	}
	return nil
}

// String get server name
func (r *RestfulServer) String() string {
	return Name
}
