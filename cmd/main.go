package main

import (
	"log"
	"net"

	"specmatic-order-bff-grpc-go/internal/com/store/order/bff/config"
	"specmatic-order-bff-grpc-go/internal/com/store/order/bff/handlers"
	"specmatic-order-bff-grpc-go/internal/com/store/order/bff/services"
	"specmatic-order-bff-grpc-go/internal/com/store/order/bff/utils"

	bff_pb "specmatic-order-bff-grpc-go/pkg/api/io/specmatic/examples/store/grpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	backendServerAddress := cfg.Backend.Host + ":" + cfg.Backend.Port

	orderServiceAddress := backendServerAddress
	productServiceAddress := backendServerAddress

	// Connect to domain services
	orderConn, err := utils.ConnectToService(orderServiceAddress)
	if err != nil {
		log.Fatal(err)
	}
	defer orderConn.Close()

	productConn, err := utils.ConnectToService(productServiceAddress)
	if err != nil {
		log.Fatal(err)
	}
	defer productConn.Close()

	// Setup BFF gRPC server
	domainAPIService := services.NewDomainAPIService(orderConn, productConn)

	grpcServer := grpc.NewServer()

	bffHandler := handlers.NewBffHandler(domainAPIService)
	bff_pb.RegisterOrderServiceServer(grpcServer, bffHandler)

	reflection.Register(grpcServer)

	lis, err := net.Listen("tcp", ":"+cfg.BFFServer.Port)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Starting gRPC server on %s", cfg.BFFServer.Port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal(err)
	}
}
