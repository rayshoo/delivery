package grpc

import (
	pb "delivery/api/gen"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (_ *deployServer) Deploy(in *pb.DeployRequest, stream pb.Deploy_DeployServer) error {
	ch := make(chan bool)
	newWork := work{
		commitSpec: in.CommitSpec,
		stream:     stream,
		specs:      in.DeploySpecs,
		ch:         &ch,
	}
	Worker <- newWork
	complete := <-ch

	if !complete {
		return status.Errorf(codes.Internal, "the grpc deploy request failed.")
	}
	return nil
}
