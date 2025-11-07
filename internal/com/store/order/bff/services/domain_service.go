package services

import (
	"context"
	"fmt"
	bff_pb "specmatic-order-bff-grpc-go/pkg/api/order_bff"
	domain_pb "specmatic-order-bff-grpc-go/pkg/api/order_api"

	"google.golang.org/grpc"
)

type DomainAPIService struct {
	orderServiceClient   domain_pb.OrderServiceClient
	productServiceClient domain_pb.ProductServiceClient
}

func NewDomainAPIService(orderConn, productConn *grpc.ClientConn) *DomainAPIService {
	return &DomainAPIService{
		orderServiceClient:   domain_pb.NewOrderServiceClient(orderConn),
		productServiceClient: domain_pb.NewProductServiceClient(productConn),
	}
}

func (s *DomainAPIService) CreateOrder(ctx context.Context, newOrder *bff_pb.NewOrder) (*bff_pb.OrderId, error) {
	domainOrder := &domain_pb.NewOrder{
		ProductId: newOrder.ProductId,
		Count:     newOrder.Count,
		Status:    domain_pb.OrderStatus_PENDING,
	}
	orderId, err := s.orderServiceClient.AddOrder(ctx, domainOrder)
	if err != nil {
		return nil, err
	}
	return &bff_pb.OrderId{Id: orderId.Id}, nil
}

func (s *DomainAPIService) FindProducts(ctx context.Context, req *bff_pb.FindAvailableProductsRequest) (*bff_pb.ProductListResponse, error) {
	domainReq := &domain_pb.ProductSearchRequest{
		Type: domain_pb.ProductType(req.Type),
	}
	domainResp, err := s.productServiceClient.SearchProducts(ctx, domainReq)
	if err != nil {
		fmt.Println("error is : ", err)
		return nil, err
	}

	products := make([]*bff_pb.Product, len(domainResp.Products))
	for i, p := range domainResp.Products {
		products[i] = &bff_pb.Product{
			Id:        p.Id,
			Name:      p.Name,
			Type:      bff_pb.ProductType(p.Type),
			Inventory: p.Inventory,
		}
	}

	// Send Kafka messages
	err = SendProductMessages(products)
	if err != nil {
		return nil, fmt.Errorf("error sending Kafka messages: %w", err)
	}

	return &bff_pb.ProductListResponse{Products: products}, nil
}

func (s *DomainAPIService) CreateProduct(ctx context.Context, newProduct *bff_pb.NewProduct) (*bff_pb.ProductId, error) {
	domainProduct := &domain_pb.NewProduct{
		Name:      newProduct.Name,
		Type:      domain_pb.ProductType(newProduct.Type),
		Inventory: newProduct.Inventory,
	}
	productId, err := s.productServiceClient.AddProduct(ctx, domainProduct)
	if err != nil {
		return nil, err
	}

	return &bff_pb.ProductId{Id: productId.Id}, nil
}
