package stream

import (
	"context"
	pb "delivery/api/gen"
	"delivery/internal/server/env"

	"delivery/internal/logger"
)

var log = logger.New(env.LogLevel)

type Stream interface {
	Send(response *pb.DeployResponse) error
	Context() context.Context
}
