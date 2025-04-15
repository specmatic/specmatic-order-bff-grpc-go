package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"specmatic-order-bff-grpc-go/internal/com/store/order/bff/config"
	"specmatic-order-bff-grpc-go/internal/com/store/order/bff/models"
	bff_pb "specmatic-order-bff-grpc-go/pkg/api/io/specmatic/examples/store/grpc"

	"github.com/segmentio/kafka-go"
)

func SendProductMessages(products []*bff_pb.Product) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	// Create a new Kafka writer with more configuration options
	w := kafka.NewWriter(kafka.WriterConfig{
		Brokers:      []string{cfg.KafkaService.Host + ":" + cfg.KafkaService.Port},
		Topic:        cfg.KafkaService.Topic,
		Balancer:     &kafka.LeastBytes{},
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
		Async:        false, // Set to true for better performance, but less reliability
	})
	defer w.Close()

	if len(products) > 0 {
		firstProduct := products[0]
		if err := sendSingleProduct(w, *firstProduct); err != nil {
			log.Printf("Error sending product (ID: %d): %v", firstProduct.Id, err)
			return err
		}
	}

	return nil
}

func sendSingleProduct(w *kafka.Writer, product bff_pb.Product) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	productMessage := models.ProductMessage{
		ID:        int(product.Id),
		Name:      product.Name,
		Inventory: int(product.Inventory),
	}

	messageValue, err := json.Marshal(productMessage)
	if err != nil {
		return fmt.Errorf("error marshaling product message: %w", err)
	}

	err = w.WriteMessages(ctx, kafka.Message{
		Key:   []byte(strconv.Itoa(int(product.Id))),
		Value: messageValue,
	})
	if err != nil {
		return fmt.Errorf("error writing message to Kafka: %w", err)
	}

	log.Printf("Successfully sent product message for ID: %d", product.Id)
	return nil
}
