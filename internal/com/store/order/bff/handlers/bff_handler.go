package handlers

import (
	"context"
	"specmatic-order-bff-grpc-go/internal/com/store/order/bff/services"
	bff_pb "specmatic-order-bff-grpc-go/pkg/api/order_bff"

	"specmatic-order-bff-grpc-go/internal/com/store/order/bff/utils"
)

type BffHandler struct {
	bff_pb.UnimplementedOrderServiceServer
	domainAPIService *services.DomainAPIService
}

func NewBffHandler(domainAPIService *services.DomainAPIService) *BffHandler {
	return &BffHandler{
		domainAPIService: domainAPIService,
	}
}

func (h *BffHandler) FindAvailableProducts(ctx context.Context, req *bff_pb.FindAvailableProductsRequest) (*bff_pb.ProductListResponse, error) {
	if err := utils.ValidateReq(req); err != nil {
		return nil, err
	}
	return h.domainAPIService.FindProducts(ctx, req)
}

func (h *BffHandler) CreateProduct(ctx context.Context, req *bff_pb.NewProduct) (*bff_pb.ProductId, error) {
	if err := utils.ValidateReq(req); err != nil {
		return nil, err
	}
	return h.domainAPIService.CreateProduct(ctx, req)
}

func (h *BffHandler) CreateOrder(ctx context.Context, req *bff_pb.NewOrder) (*bff_pb.OrderId, error) {
	if err := utils.ValidateReq(req); err != nil {
		return nil, err
	}
	return h.domainAPIService.CreateOrder(ctx, req)
}
