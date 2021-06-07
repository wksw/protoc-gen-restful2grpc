package restful2grpc

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	"gitee.com/paasport/protos-repo/restful"
	pb "github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/wksw/protobuf/proto"
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
)

// Init initializes the plugin.
func (g *restful2grpc) Init(gen *generator.Generator) {
	g.gen = gen
	corePkg = generator.RegisterUniquePackageName("core", nil)
	commonPkg = generator.RegisterUniquePackageName("common", nil)
	contextPkg = generator.RegisterUniquePackageName("context", nil)
	metadataPkg = generator.RegisterUniquePackageName("metadata", nil)
	restfulPkg = generator.RegisterUniquePackageName("rf", nil)
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
	// g.P(corePkg, " ", strconv.Quote(path.Join(g.gen.ImportPrefix, corePkgPath)))
	// g.P(contextPkg, " ", strconv.Quote(path.Join(g.gen.ImportPrefix, contextPkgPath)))
	// g.P("_", " ", strconv.Quote(path.Join(g.gen.ImportPrefix, clientPkgPath)))
	g.P("rf", " ", strconv.Quote(path.Join(g.gen.ImportPrefix, restfulPkgPath)))
	// g.P(commonPkg, " ", strconv.Quote(path.Join(g.gen.ImportPrefix, commonPkgPath)))
	// g.P(metadataPkg, " ", strconv.Quote(path.Join(g.gen.ImportPrefix, metadataPkgPath)))
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

	// Client interface.
	// g.P("type ", servAlias, " interface {")
	// for i, method := range service.Method {
	// 	g.gen.PrintComments(fmt.Sprintf("%s,2,%d", path, i)) // 2 means method in a service.
	// 	g.P(g.generateClientSignature(servName, method))
	// }
	// g.P("}")
	// g.P()

	// // Client structure.
	// g.P("type ", unexport(servAlias), " struct {")
	// g.P("RPCInvoker *", corePkg, ".RPCInvoker")
	// g.P("context  ", contextPkg, ".Context")
	// g.P("serviceName ", "string")
	// g.P("}")
	// g.P()

	// g.P("var _ ", servAlias, " = &", unexport(servAlias), "{}")

	// newCotext
	/*
		func newContext(ctx context.Context) context.Context {
			md, _ := metadata.FromIncomingContext(ctx)
			var header = make(map[string]string)
			for key, value := range md {
				if len(value) > 0 {
					header[key] = value[0]
				}
			}
			return common.NewContext(header)
		}
	*/
	// g.P("func newContext(ctx ", contextPkg, ".Context", ") ", contextPkg, ".Context {")
	// g.P("md, _ := ", metadataPkg, ".FromIncomingContext(ctx)")
	// g.P("var header = make(map[string]string)")
	// g.P("for key, value := range md {")
	// g.P("if len(value)>0 { header[key]=value[0] }")
	// g.P("}")
	// g.P("return ", commonPkg, ".NewContext(header)")
	// g.P("}")

	// NewClient factory.
	/*
		func NewAccountService(ctx context.Context, opt ...core.Option) AccountService {
			return &accountService{
				RPCInvoker:  core.NewRPCInvoker(opt...),
				context:     newContext(ctx),
				serviceName: serviceName,
			}
		}
	*/
	// g.P("func New", servAlias, " (ctx ", contextPkg, ".Context, ", "serviceName string, ", "opt ...", corePkg, ".Option) ", servAlias, " {")
	// g.P("return &", unexport(servAlias), "{")
	// g.P("RPCInvoker: ", corePkg, ".NewRPCInvoker(opt...),")
	// g.P("context: newContext(ctx),")
	// g.P("serviceName: serviceName,")
	// g.P("}")
	// g.P("}")
	// g.P()
	var methodIndex, streamIndex int
	serviceDescVar := "_" + servName + "_serviceDesc"
	// Client method implementations.
	// http转grpc协议实现
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
		g.generateClientMethod(serviceName, servName, serviceDescVar, method, descExpr)
	}
	// http路由
	g.P("func (h *", servHttpAlias, ") URLPatterns() []rf.Route {")
	g.P("return []rf.Route{")
	for _, method := range service.Method {
		ext, err := proto.GetExtension(method.Options, restful.E_Http)
		if err != nil {
			continue
		}
		httpRule := ext.(*restful.HttpRule)
		g.P("{")
		g.P("FuncDesc: ", httpRule.Doc)
		g.P("}")
	}
	g.P("}")
	g.P("}")

	// g.P("// Server API for ", servName, " service")
	// g.P()
}

// generateClientSignature returns the client-side signature for a method.
func (g *restful2grpc) generateClientSignature(servName string, method *pb.MethodDescriptorProto) string {
	return ""
	// origMethName := method.GetName()
	// methName := generator.CamelCase(origMethName)
	// if reservedClientName[methName] {
	// 	methName += "_"
	// }
	// reqArg := " in *" + g.typeName(method.GetInputType())
	// if method.GetClientStreaming() {
	// 	reqArg = ""
	// }
	// respName := "*" + g.typeName(method.GetOutputType())
	// if method.GetServerStreaming() || method.GetClientStreaming() {
	// 	respName = servName + "_" + generator.CamelCase(origMethName) + "Service"
	// }

	// return fmt.Sprintf("%s) (rf.Context)", methName)
}

func (g *restful2grpc) generateClientMethod(reqServ, servName, serviceDescVar string, method *pb.MethodDescriptorProto, descExpr string) {
	// reqMethod := fmt.Sprintf("%s.%s", servName, method.GetName())
	// schema := fmt.Sprintf("%s.%s", reqServ, servName)
	methName := generator.CamelCase(method.GetName())
	inType := g.typeName(method.GetInputType())
	// outType := g.typeName(method.GetOutputType())

	servAlias := servName + "HttpHandler"
	g.P("func (h *", servAlias, " )", methName, " (ctx *rf.Context) {")
	g.P("var req ", inType)
	g.P("if err := rf.ReadEntry(&req); err != nil {")
	g.P("rf.Response(ctx, nil, err)")
	g.P("return")
	g.P("}")
	g.P("resp, err := h.GrpcHandler.", methName, "(ctx, &req)")
	g.P("rf.Response(ctx, resp, err)")
	g.P("return")
	g.P("}")

	// g.P("func (c *", unexport(servAlias), ") ", g.generateClientSignature(servName, method), "{")
	// if !method.GetServerStreaming() && !method.GetClientStreaming() {
	// 	g.P("out := new(", outType, ")")
	// 	g.P(`err := c.RPCInvoker.Invoke(c.context, c.serviceName,"`, schema, `", "`, method.GetName(), `", in, out,`, corePkg, `.WithProtocol("grpc"))`)
	// 	// // TODO: Pass descExpr to Invoke.
	// 	// g.P("err := ", `c.c.Call(ctx, req, out, opts...)`)
	// 	// g.P("if err != nil { return nil, err }")
	// 	g.P("return out, err")
	// 	g.P("}")
	// 	// g.P()
	// 	return
	// }
	// streamType := unexport(servAlias) + methName
	// g.P(`req := c.RPCInvoker1.Invoke(c.context, "`, reqMethod, `", &`, inType, `{})`)
	// g.P("stream, err := c.c.Stream(ctx, req, opts...)")
	// g.P("if err != nil { return nil, err }")

	// if !method.GetClientStreaming() {
	// 	g.P("if err := stream.Send(in); err != nil { return nil, err }")
	// }

	// g.P("return &", streamType, "{stream}, nil")
	// g.P("}")
	// g.P()

	// genSend := method.GetClientStreaming()
	// genRecv := method.GetServerStreaming()

	// Stream auxiliary types and methods.
	// g.P("type ", servName, "_", methName, "Service interface {")
	// g.P("Context() context.Context")
	// g.P("SendMsg(interface{}) error")
	// g.P("RecvMsg(interface{}) error")
	// g.P("Close() error")

	// if genSend {
	// 	g.P("Send(*", inType, ") error")
	// }
	// if genRecv {
	// 	g.P("Recv() (*", outType, ", error)")
	// }
	// g.P("}")
	// g.P()

	// g.P("type ", streamType, " struct {")
	// g.P("stream ", clientPkg, ".Stream")
	// g.P("}")
	// g.P()

	// g.P("func (x *", streamType, ") Close() error {")
	// g.P("return x.stream.Close()")
	// g.P("}")
	// g.P()

	// g.P("func (x *", streamType, ") Context() context.Context {")
	// g.P("return x.stream.Context()")
	// g.P("}")
	// g.P()

	// g.P("func (x *", streamType, ") SendMsg(m interface{}) error {")
	// g.P("return x.stream.Send(m)")
	// g.P("}")
	// g.P()

	// g.P("func (x *", streamType, ") RecvMsg(m interface{}) error {")
	// g.P("return x.stream.Recv(m)")
	// g.P("}")
	// g.P()

	// if genSend {
	// 	g.P("func (x *", streamType, ") Send(m *", inType, ") error {")
	// 	g.P("return x.stream.Send(m)")
	// 	g.P("}")
	// 	g.P()

	// }

	// if genRecv {
	// 	g.P("func (x *", streamType, ") Recv() (*", outType, ", error) {")
	// 	g.P("m := new(", outType, ")")
	// 	g.P("err := x.stream.Recv(m)")
	// 	g.P("if err != nil {")
	// 	g.P("return nil, err")
	// 	g.P("}")
	// 	g.P("return m, nil")
	// 	g.P("}")
	// 	g.P()
	// }
}
