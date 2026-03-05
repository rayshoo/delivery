package grpc

import (
	pb "delivery/api/gen"
	"delivery/internal/server/env"

	"delivery/internal/logger"
)

var log = logger.New(env.LogLevel)

type deployServer struct {
	pb.UnimplementedDeployServer
}

type work struct {
	commitSpec *pb.CommitSpec
	stream     pb.Deploy_DeployServer
	ch         *chan bool
	specs      []*pb.DeploySpec
}
