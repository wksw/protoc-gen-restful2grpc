package restful2grpc

import (
	"encoding/json"
	"fmt"
	"path"
	"strconv"
	"strings"

	"gitee.com/paasport/protos-repo/restful"
	"github.com/golang/protobuf/proto"
	pb "github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/wksw/protoc-gen-restful2grpc/generator"
)

// Paths for packages used by code generated in this file,
// relative to the import_prefix of the generator.Generator.
const (
	// corePkgPath     = "github.com/go-restful2grpc/go-restful2grpc/core"
	// commonPkgPath   = "github.com/go-restful2grpc/go-restful2grpc/core/common"
	// contextPkgPath  = "context"
	// clientPkgPath   = "github.com/go-restful2grpc/go-restful2grpc-protocol/client/grpc"
	// metadataPkgPath = "google.golang.org/grpc/metadata"
	restfulPkgPath = "github.com/wksw/protoc-gen-restful2grpc/restful"
	httpPkgPath    = "net/http"
)

func init() {
	generator.RegisterPlugin(new(restful2grpc))
}

// restful2grpc is an implementation of the Go protocol buffer compiler's
// plugin architecture.  It generates bindings for go-restful2grpc support.
type restful2grpc struct {
	gen *generator.Generator
}

// Name returns the name of this plugin, "restful2grpc".
func (g *restful2grpc) Name() string {
	return "restful2grpc"
}

// The names for packages imported in the generated code.
// They may vary from the final path component of the import path
// if the name is used by other packages.
var (
	corePkg     string
	commonPkg   string
	contextPkg  string
	clientPkg   string
	metadataPkg string
	pkgImports  map[generator.GoPackageName]bool
	restfulPkg  string
	httpPkg     string
)

// Init initializes the plugin.
func (g *restful2grpc) Init(gen *generator.Generator) {
	g.gen = gen
	corePkg = generator.RegisterUniquePackageName("core", nil)
	commonPkg = generator.RegisterUniquePackageName("common", nil)
	contextPkg = generator.RegisterUniquePackageName("context", nil)
	metadataPkg = generator.RegisterUniquePackageName("metadata", nil)
	restfulPkg = generator.RegisterUniquePackageName("rf", nil)
	httpPkg = generator.RegisterUniquePackageName("", nil)
}

// Given a type name defined in a .proto, return its object.
// Also record that we're using it, to guarantee the associated import.
func (g *restful2grpc) objectNamed(name string) generator.Object {
	g.gen.RecordTypeUse(name)
	return g.gen.ObjectNamed(name)
}

// Given a type name defined in a .proto, return its name as we will print it.
func (g *restful2grpc) typeName(str string) string {
	return g.gen.TypeName(g.objectNamed(str))
}

// P forwards to g.gen.P.
func (g *restful2grpc) P(args ...interface{}) { g.gen.P(args...) }

// Generate generates code for the services in the given file.
func (g *restful2grpc) Generate(file *generator.FileDescriptor) {
	if len(file.FileDescriptorProto.Service) == 0 {
		return
	}
	g.P("// Reference imports to suppress errors if they are not otherwise used.")
	g.P("var _ = http.MethodGet")
	g.P("var _ = rf.Name")
	g.P()

	for i, service := range file.FileDescriptorProto.Service {
		g.generateService(file, service, i)
	}
}

// GenerateImports generates the import declaration for this file.
func (g *restful2grpc) GenerateImports(file *generator.FileDescriptor, imports map[generator.GoImportPath]generator.GoPackageName) {
	if len(file.FileDescriptorProto.Service) == 0 {
		return
	}
	g.P("import (")
	g.P("rf", " ", strconv.Quote(path.Join(g.gen.ImportPrefix, restfulPkgPath)))
	g.P("", " ", strconv.Quote(path.Join(g.gen.ImportPrefix, httpPkgPath)))
	g.P(")")
	g.P()

	// We need to keep track of imported packages to make sure we don't produce
	// a name collision when generating types.
	pkgImports = make(map[generator.GoPackageName]bool)
	for _, name := range imports {
		pkgImports[name] = true
	}
}

// reservedClientName records whether a client name is reserved on the client side.
var reservedClientName = map[string]bool{
	// TODO: do we need any in go-restful2grpc?
}

func unexport(s string) string {
	if len(s) == 0 {
		return ""
	}
	name := strings.ToLower(s[:1]) + s[1:]
	if pkgImports[generator.GoPackageName(name)] {
		return name + "_"
	}
	return name
}

// generateService generates all the code for the named service.
func (g *restful2grpc) generateService(file *generator.FileDescriptor, service *pb.ServiceDescriptorProto, index int) {
	// path := fmt.Sprintf("6,%d", index) // 6 means service.

	origServName := service.GetName()
	serviceName := strings.ToLower(service.GetName())
	if pkg := file.GetPackage(); pkg != "" {
		serviceName = pkg
	}
	servName := generator.CamelCase(origServName)
	servAlias := servName + "Server"
	servHttpAlias := servName + "HttpHandler"

	g.P("type ", servHttpAlias, " struct {")
	g.P("GrpcHandler ", servAlias)
	g.P("}")
	var methodIndex, streamIndex int
	serviceDescVar := "_" + servName + "_serviceDesc"
	// Client method implementations.
	// http转grpc协议实现
	var routes []string
	for _, method := range service.Method {
		var descExpr string
		if !method.GetServerStreaming() {
			// Unary RPC method
			descExpr = fmt.Sprintf("&%s.Methods[%d]", serviceDescVar, methodIndex)
			methodIndex++
		} else {
			// Streaming RPC method
			descExpr = fmt.Sprintf("&%s.Streams[%d]", serviceDescVar, streamIndex)
			streamIndex++
		}
		if route := g.generateClientMethod(serviceName, servName, serviceDescVar, method, descExpr); route != "" {
			routes = append(routes, route)
		}

	}
	// http路由
	g.P("func (h *", servHttpAlias, ") URLPatterns() []rf.Route {")
	g.P("var routes []rf.Route")
	for _, route := range routes {
		g.P("routes = append(routes, h.", route, "URLPatterns())")
	}
	g.P("return routes")
	g.P("}")
}

// generateClientSignature returns the client-side signature for a method.
func (g *restful2grpc) generateClientSignature(servName string, method *pb.MethodDescriptorProto) string {
	return ""
}

func (g *restful2grpc) generateClientMethod(reqServ, servName, serviceDescVar string, method *pb.MethodDescriptorProto, descExpr string) string {
	methName := generator.CamelCase(method.GetName())
	inType := g.typeName(method.GetInputType())
	servAlias := servName + "HttpHandler"

	g.P("func (h *", servAlias, ")get", methName, "Req(ctx *rf.Context)", "(*", inType, ",error){")
	g.P("var req ", inType)
	g.P("err := ctx.Read(&req)")
	g.P("return &req, err")
	g.P("}")

	routeName := ""
	ext, err := proto.GetExtension(method.Options, restful.E_Http)
	if err == nil {
		path := ""
		reqMethod := ""

		httpRule := ext.(*restful.HttpRule)
		switch httpRule.GetPattern().(type) {
		case *restful.HttpRule_Get:
			path = httpRule.GetGet()
			reqMethod = "http.MethodGet"
		case *restful.HttpRule_Put:
			path = httpRule.GetPut()
			reqMethod = "http.MethodPut"
		case *restful.HttpRule_Head:
			path = httpRule.GetHead()
			reqMethod = "http.MethodHead"
		case *restful.HttpRule_Patch:
			path = httpRule.GetPatch()
			reqMethod = "http.MethodPatch"
		case *restful.HttpRule_Post:
			path = httpRule.GetPost()
			reqMethod = "http.MethodPost"
		case *restful.HttpRule_Delete:
			path = httpRule.GetDelete()
			reqMethod = "http.MethodDelete"
		}

		if path != "" {
			g.P("func (h *", servAlias, " )", methName, "URLPatterns() rf.Route {")
			metadata := make(map[string]string)
			if len(httpRule.Metadata) != 0 {
				for _, value := range httpRule.Metadata {
					if value.Field != "" && value.Value != "" {
						metadata[value.Field] = value.Value
					}
				}
			}
			metadataByte, _ := json.Marshal(metadata)
			g.P("return rf.Route{",
				"Method: ", reqMethod, ",",
				`Path: "`, path, `",`,
				`FuncDesc: "`, httpRule.Doc, `",`,
				`ResourceFuncName: "`, methName, `",`,
				`Version: "`, httpRule.Version, `",`,
				"Metadata: map[string]string", string(metadataByte), ",",
				"}")
			g.P("}")

			g.P("func (h *", servAlias, " )", methName, " (ctx *rf.Context) {")
			g.P("req, err := h.get", methName, "Req(ctx)")
			g.P("if err != nil {")
			g.P("rf.Response(ctx, nil, err)")
			g.P("return")
			g.P("}")
			g.P("resp, err := h.GrpcHandler.", methName, "(ctx.Ctx, req)")
			g.P("rf.Response(ctx, resp, err)")
			g.P("return")
			g.P("}")
			routeName = methName
		}
	}
	return routeName
}
