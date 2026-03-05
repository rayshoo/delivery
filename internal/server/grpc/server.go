package grpc

import (
	pb "delivery/api/gen"
	"fmt"

	"github.com/fatih/color"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// PrintLogo 는 gRPC 서버 시작을 알리는 터미널 출력을 하는 함수 입니다.
func PrintLogo(fullAddr *string) {
	logo := "          ____  ____  ______\n   ____ _/ __ \\/ __ \\/ ____/\n  / __ `/ /_/ / /_/ / /     \n / /_/ / _, _/ ____/ /___   \n \\__, /_/ |_/_/    \\____/   \n/____/                      "
	url := "https://grpc.io"

	fmt.Printf("%s\nA high performance, open source universal RPC framework\n", logo)
	color.Blue(url)
	fmt.Print("_____________________________________________\n\n⇨ grpc server started on ")
	color.Magenta(*fullAddr)
}

// NewServer 는 gRPC 서버 객체를 생성해서 반환합니다.
func NewServer() *grpc.Server {
	s := grpc.NewServer()
	pb.RegisterDeployServer(s, &deployServer{})

	healthServer := health.NewServer()
	healthServer.SetServingStatus("deploy", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(s, healthServer)

	return s
}
