//go:build grpc

// Package grpc implements the AnomalyScorer gRPC service.
// This file requires generated protobuf stubs — run `make proto` first, then
// build with: go build -tags grpc ./...
package grpc

import (
	"context"
	"log"
	"net"

	pb "aegis/api-gateway/internal/pb"

	"google.golang.org/grpc"
)

// AnomalyScorerServer implements the gRPC AnomalyScorer service defined in
// proto/inference.proto. It is the internal entry point for the Go API Gateway
// to request anomaly scores from the inference layer.
type AnomalyScorerServer struct {
	pb.UnimplementedAnomalyScorerServer
}

// ScoreTelemetryBatch accepts a batch of PLC readings and returns an anomaly verdict.
func (s *AnomalyScorerServer) ScoreTelemetryBatch(
	ctx context.Context,
	req *pb.TelemetryBatch,
) (*pb.ScoreResponse, error) {
	log.Printf("grpc: ScoreTelemetryBatch vin=%s torque_readings=%d", req.Vin, len(req.TorqueReadings))
	// TODO: forward to inference-worker via NATS request-reply or run scorer inline.
	return &pb.ScoreResponse{
		IsAnomalous:     false,
		ConfidenceScore: 0.1,
	}, nil
}

// Serve starts the gRPC listener on addr and blocks until the server stops.
func Serve(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	srv := grpc.NewServer()
	pb.RegisterAnomalyScorerServer(srv, &AnomalyScorerServer{})
	log.Printf("gRPC AnomalyScorer listening on %s", addr)
	return srv.Serve(lis)
}
