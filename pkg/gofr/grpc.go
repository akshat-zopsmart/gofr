package gofr

import (
	"net"
	"strconv"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	grpc2 "github.com/vikash/gofr/pkg/gofr/grpc"

	"google.golang.org/grpc"
)

type grpcServer struct {
	server *grpc.Server
	port   int
}

func newGRPCServer(container *Container, port int) *grpcServer {
	return &grpcServer{
		server: grpc.NewServer(
			grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
				grpc_recovery.UnaryServerInterceptor(),
				grpc2.LoggingInterceptor(container.Logger),
			))),
		port: port,
	}
}

func (g *grpcServer) Run(container *Container) {
	addr := ":" + strconv.Itoa(g.port)

	container.Logger.Infof("starting grpc server at %s", addr)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		container.Logger.Errorf("error in starting grpc server at %s: %s", addr, err)
		return
	}

	if err := g.server.Serve(listener); err != nil {
		container.Logger.Errorf("error in starting grpc server at %s: %s", addr, err)
		return
	}
}
