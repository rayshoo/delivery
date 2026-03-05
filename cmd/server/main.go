package main

import (
	"context"
	"encoding/json"
	pb "delivery/api/gen"
	"delivery/internal/server/env"
	"delivery/internal/server/grpc"
	_ "delivery/internal/server/repo"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"delivery/internal/logger"
	gRPC "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

var version string

func main() {
	if len(os.Args) > 1 {
		if os.Args[1] == "version" || os.Args[1] == "--version" || os.Args[1] == "-v" {
			fmt.Printf("deploy server version: %s\n", version)
			return
		}
	}

	log := logger.New(env.LogLevel)

	grpcAddr := fmt.Sprintf("%s:%s", env.Addr, env.Port)
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Panicln(err.Error())
	}

	s := grpc.NewServer()
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	// HTTP gateway
	httpAddr := fmt.Sprintf("%s:%s", env.Addr, env.HTTPPort)
	var httpServer *http.Server
	if env.HTTPPort != "" {
		go func() {
			mux := runtime.NewServeMux()
			opts := []gRPC.DialOption{gRPC.WithTransportCredentials(insecure.NewCredentials())}
			if err := pb.RegisterDeployHandlerFromEndpoint(context.Background(), mux, grpcAddr, opts); err != nil {
				log.Errorf("failed to register deploy gateway: %v", err)
				return
			}

			// REST health check endpoint
			mux.HandlePath("GET", "/api/v1/health", func(w http.ResponseWriter, r *http.Request, _ map[string]string) {
				conn, err := gRPC.NewClient(grpcAddr, gRPC.WithTransportCredentials(insecure.NewCredentials()))
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				defer conn.Close()
				resp, err := healthpb.NewHealthClient(conn).Check(r.Context(), &healthpb.HealthCheckRequest{Service: "deploy"})
				if err != nil {
					http.Error(w, err.Error(), http.StatusServiceUnavailable)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"status": resp.Status.String()})
			})

			httpServer = &http.Server{Addr: httpAddr, Handler: mux}
			log.Infof("http gateway started on %s", httpAddr)
			if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Errorf("http gateway error: %v", err)
			}
		}()
	}

	go func() {
		<-sigs
		log.Warnln("the program received the sigterm signal. try a graceful shutdown.")
		if httpServer != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = httpServer.Shutdown(ctx)
		}
		go s.GracefulStop()
		time.AfterFunc(30*time.Second, s.Stop)
	}()

	grpc.PrintLogo(&grpcAddr)
	if err := s.Serve(lis); err != nil {
		log.Panicln(err.Error())
	}
	log.Infoln("program shutdown with code zero")
}
