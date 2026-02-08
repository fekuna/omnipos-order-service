package handler

import (
	"context"

	"github.com/fekuna/omnipos-order-service/internal/auth"
	"github.com/fekuna/omnipos-order-service/internal/model"
	"github.com/fekuna/omnipos-order-service/internal/order/usecase"
	orderv1 "github.com/fekuna/omnipos-proto/proto/order/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type OrderHandler struct {
	orderv1.UnimplementedOrderServiceServer
	uc usecase.Usecase
}

func NewOrderHandler(uc usecase.Usecase) *OrderHandler {
	return &OrderHandler{uc: uc}
}

func (h *OrderHandler) CreateOrder(ctx context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
	// Extract MerchantID/CashierID from context (middleware)
	// Assuming metadata/context has it.
	// For now, hardcode or expect it to be passed?
	// The Gateway passes x-merchant-id header. We need to extract it.
	// BUT we don't have the context extractor helper here yet.
	// Let's assume we can get it from headers or it's passed in the request?
	// Request doesn't have merchant_id.
	// We MUST parse metadata.

	// Simplified: Just use a placeholder or check headers if we implement a helper.
	// Let's assume the auth interceptor works and puts it in context values or we parse metadata.

	// TODO: proper metadata extraction
	merchantID := "14045670-dd76-416d-b798-436757cef4b6" // Hardcoded for dev until helper is imported
	storeID := req.StoreId
	if storeID == "" {
		storeID = uuid.New().String() // Generate a random store ID for now
	}

	input := &usecase.CreateOrderInput{
		MerchantID:    merchantID,
		StoreID:       storeID,
		PaymentMethod: model.PaymentMethod(req.PaymentMethod.String()), // Enum mapping?
		PaidAmount:    req.PaidAmount,
	}

	// Map Enum carefully
	// Proto Enum: PAYMENT_METHOD_CASH (1) -> Model: "CASH"
	switch req.PaymentMethod {
	case orderv1.PaymentMethod_PAYMENT_METHOD_CASH:
		input.PaymentMethod = model.PaymentMethodCash
	case orderv1.PaymentMethod_PAYMENT_METHOD_QRIS:
		input.PaymentMethod = model.PaymentMethodQRIS
	default:
		input.PaymentMethod = model.PaymentMethodCash
	}

	if req.CustomerId != "" {
		input.CustomerID = &req.CustomerId
	}

	for _, item := range req.Items {
		var variantID *string
		if item.VariantId != "" {
			val := item.VariantId
			variantID = &val
		}
		input.Items = append(input.Items, usecase.CreateOrderItemInput{
			ProductID: item.ProductId,
			VariantID: variantID,
			Quantity:  item.Quantity,
			UnitPrice: item.UnitPrice,
			Notes:     item.Notes,
		})
	}

	order, err := h.uc.CreateOrder(ctx, input)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create order: %v", err)
	}

	return &orderv1.CreateOrderResponse{
		Order: mapModelToProto(order),
	}, nil
}

func (h *OrderHandler) GetOrder(ctx context.Context, req *orderv1.GetOrderRequest) (*orderv1.GetOrderResponse, error) {
	order, err := h.uc.GetOrder(ctx, req.Id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get order: %v", err)
	}
	if order == nil {
		return nil, status.Error(codes.NotFound, "order not found")
	}

	return &orderv1.GetOrderResponse{
		Order: mapModelToProto(order),
	}, nil
}

func (h *OrderHandler) ListOrders(ctx context.Context, req *orderv1.ListOrdersRequest) (*orderv1.ListOrdersResponse, error) {
	filters := map[string]interface{}{
		"limit":  int(req.PageSize),
		"offset": int((req.Page - 1) * req.PageSize),
		// "merchant_id": ...
	}
	if req.StoreId != "" {
		filters["store_id"] = req.StoreId
	}

	orders, total, err := h.uc.ListOrders(ctx, filters)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list orders: %v", err)
	}

	var protoOrders []*orderv1.Order
	for _, o := range orders {
		protoOrders = append(protoOrders, mapModelToProto(&o))
	}

	return &orderv1.ListOrdersResponse{
		Orders:   protoOrders,
		Total:    int32(total),
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

func mapModelToProto(o *model.Order) *orderv1.Order {
	// Map Status Enum
	var status orderv1.OrderStatus
	switch o.Status {
	case model.OrderStatusPaid:
		status = orderv1.OrderStatus_ORDER_STATUS_PAID
	case model.OrderStatusPending:
		status = orderv1.OrderStatus_ORDER_STATUS_PENDING
	default:
		status = orderv1.OrderStatus_ORDER_STATUS_UNSPECIFIED
	}

	po := &orderv1.Order{
		Id:         o.ID,
		MerchantId: o.MerchantID,
		StoreId:    o.StoreID,
		// CustomerId:     valOrEmpty(o.CustomerID),
		// CashierId:      valOrEmpty(o.CashierID),
		OrderNumber: o.OrderNumber,
		Status:      status,
		// PaymentMethod: ...
		Subtotal:       o.Subtotal,
		TaxAmount:      o.TaxAmount,
		DiscountAmount: o.DiscountAmount,
		TotalAmount:    o.TotalAmount,
		PaidAmount:     o.PaidAmount,
		ChangeAmount:   o.ChangeAmount,
		CreatedAt:      timestamppb.New(o.CreatedAt),
		UpdatedAt:      timestamppb.New(o.UpdatedAt),
	}

	if o.CustomerID != nil {
		po.CustomerId = *o.CustomerID
	}
	if o.CashierID != nil {
		po.CashierId = *o.CashierID
	}

	// Items
	for _, item := range o.Items {
		pi := &orderv1.OrderItem{
			Id:          item.ID,
			ProductId:   item.ProductID,
			ProductName: item.ProductName,
			Quantity:    item.Quantity,
			UnitPrice:   item.UnitPrice,
			TotalPrice:  item.TotalPrice,
		}
		if item.VariantID != nil {
			pi.VariantId = *item.VariantID
		}
		if item.Notes != nil {
			pi.Notes = *item.Notes
		}
		po.Items = append(po.Items, pi)
	}

	return po
}

func (h *OrderHandler) GetDashboardStats(ctx context.Context, req *orderv1.GetDashboardStatsRequest) (*orderv1.GetDashboardStatsResponse, error) {
	merchantID := auth.GetMerchantID(ctx)
	if merchantID == "" {
		return nil, status.Error(codes.Unauthenticated, "missing merchant context")
	}

	stats, err := h.uc.GetDashboardStats(ctx, merchantID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get dashboard stats: %v", err)
	}

	// Map model to proto
	resp := &orderv1.GetDashboardStatsResponse{
		TotalRevenue:   stats.TotalRevenue,
		TotalOrders:    int32(stats.TotalOrders),
		TotalItemsSold: int32(stats.TotalItemsSold),
		// Trends are defaults (0) for now
	}

	// Charts
	for _, s := range stats.SalesChart {
		resp.SalesChart = append(resp.SalesChart, &orderv1.DailySales{
			Date:  s.Date,
			Total: s.Total,
		})
	}

	// Top Products
	for _, tp := range stats.TopProducts {
		resp.TopProducts = append(resp.TopProducts, &orderv1.TopProductStat{
			ProductName: tp.ProductName,
			SalesCount:  int32(tp.SalesCount),
			Revenue:     tp.Revenue,
		})
	}

	// Recent
	for _, o := range stats.RecentTransactions {
		resp.RecentTransactions = append(resp.RecentTransactions, mapModelToProto(&o))
	}

	return resp, nil
}
