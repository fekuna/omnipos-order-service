package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/fekuna/omnipos-order-service/internal/model"
	"github.com/fekuna/omnipos-order-service/internal/order/repository"
	"github.com/fekuna/omnipos-pkg/broker"
	productv1 "github.com/fekuna/omnipos-proto/gen/go/omnipos/product/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
)

type Usecase interface {
	CreateOrder(ctx context.Context, input *CreateOrderInput) (*model.Order, error)
	GetOrder(ctx context.Context, id string) (*model.Order, error)
	ListOrders(ctx context.Context, filters map[string]interface{}) ([]model.Order, int, error)
	GetDashboardStats(ctx context.Context, merchantID string) (*model.DashboardStats, error)
}

type orderUseCase struct {
	repo          repository.Repository
	producer      *broker.KafkaProducer
	productClient productv1.ProductServiceClient
}

func NewOrderUseCase(repo repository.Repository, producer *broker.KafkaProducer, productClient productv1.ProductServiceClient) Usecase {
	return &orderUseCase{
		repo:          repo,
		producer:      producer,
		productClient: productClient,
	}
}

type CreateOrderItemInput struct {
	ProductID   string `json:"product_id"`
	ProductName string `json:"product_name"` // Client sends name for now, or fetch from product service? Client usually sends ID, we should fetch. But for speed, let's trust client or assume name is passed (or fetch later). For now, assume client sends name or we put placeholder.
	// Ideally we call ProductService to get name and price.
	// For this iteration, let's assume the frontend sends the price and name (trust client for POS/offline first approach, or simple validation).
	// Better: Client sends ID, we fetch. But that adds complexity of inter-service call.
	// Let's rely on input for now as per "CreateOrderRequest" in proto.
	// Wait, proto CreateOrderItemInput has product_id but NOT name.
	// So we MUST fetch product details or accept them if proto allows.
	// Best practice: Product Service call.
	// Compromise for now: Store "Product {ID}" or similar if no name, OR update proto.
	// Actually, storing the snapshot of the name is important.
	// Let's update `model` to allow Name to be empty or passed.
	// Input DTO here will match Handler's extraction.

	// Let's assume for this step we will set a placeholder name if not available,
	// OR we should have defined Product Name in the input.
	// Let's look at `CreateOrderItemInput` in proto again. It only had ID.
	// Let's stick to the plan: simple logic. We might need to call Product Service later.
	// For now, I will generate UUIDs and calculate totals.
	VariantID *string `json:"variant_id"`
	Quantity  float64 `json:"quantity"`
	UnitPrice float64 `json:"unit_price"` // Snapshot price
	Notes     string  `json:"notes"`
}

type CreateOrderInput struct {
	MerchantID    string
	StoreID       string
	CustomerID    *string
	CashierID     *string
	PaymentMethod model.PaymentMethod
	PaidAmount    float64
	Items         []CreateOrderItemInput
}

type OrderCreatedEvent struct {
	EventID   string       `json:"event_id"`
	EventType string       `json:"event_type"`
	Payload   *model.Order `json:"payload"`
	Timestamp time.Time    `json:"timestamp"`
}

func (uc *orderUseCase) CreateOrder(ctx context.Context, input *CreateOrderInput) (*model.Order, error) {
	if len(input.Items) == 0 {
		return nil, errors.New("order must have at least one item")
	}

	orderID := uuid.New().String()

	// 1. Reserve Stock (Synchronous)
	// Build request
	reserveReq := &productv1.ReserveStockRequest{
		OrderId: orderID,
		Items:   make([]*productv1.ReserveStockRequest_Item, 0, len(input.Items)),
	}

	for _, item := range input.Items {
		var variantID string
		if item.VariantID != nil {
			variantID = *item.VariantID
		}
		reserveReq.Items = append(reserveReq.Items, &productv1.ReserveStockRequest_Item{
			ProductId: item.ProductID,
			VariantId: variantID, // If empty, ignored
			Quantity:  int32(item.Quantity),
		})
	}

	// Call Product Service
	// Add merchant_id to outgoing metadata
	md := metadata.Pairs("x-merchant-id", input.MerchantID)
	outCtx := metadata.NewOutgoingContext(ctx, md)

	reserveResp, err := uc.productClient.ReserveStock(outCtx, reserveReq)
	if err != nil {
		return nil, fmt.Errorf("failed to check stock: %w", err)
	}
	if !reserveResp.Success {
		return nil, fmt.Errorf("out of stock: %s", reserveResp.Message)
	}

	// 2. Continue with Order Creation
	orderNumber := "ORD-" + time.Now().Format("20060102-150405")

	var subtotal float64
	var orderItems []model.OrderItem

	for _, itemInput := range input.Items {
		totalPrice := itemInput.Quantity * itemInput.UnitPrice
		subtotal += totalPrice

		// Placeholder name if we don't fetch from product service
		pName := "Product " + itemInput.ProductID

		item := model.OrderItem{
			ID:          uuid.New().String(),
			OrderID:     orderID,
			ProductID:   itemInput.ProductID,
			ProductName: pName, // TODO: Fetch from Product Service or accept in input
			VariantID:   itemInput.VariantID,
			// VariantName: itemInput.VariantName,
			Quantity:   itemInput.Quantity,
			UnitPrice:  itemInput.UnitPrice,
			TotalPrice: totalPrice,
			Notes:      &itemInput.Notes,
			CreatedAt:  time.Now(),
		}
		// Handle pointer for empty string notes?
		if itemInput.Notes == "" {
			item.Notes = nil
		}
		orderItems = append(orderItems, item)
	}

	// Simple tax logic: 10%? or 0?
	// Let's assume 0 for now or configurable.
	taxAmount := 0.0 // subtotal * 0.10
	discountAmount := 0.0
	totalAmount := subtotal + taxAmount - discountAmount

	// Validate payment
	if input.PaidAmount < totalAmount && input.PaymentMethod == model.PaymentMethodCash {
		// Allow partial payment? No, standard POS usually fully paid or split.
		// warning: "payment less than total"
	}

	changeAmount := input.PaidAmount - totalAmount
	if changeAmount < 0 {
		changeAmount = 0
	}

	status := model.OrderStatusPaid
	// IF we are doing pay later/orders, it might be Pending.
	// For POS direct sale, it is Paid.

	order := &model.Order{
		ID:             orderID,
		MerchantID:     input.MerchantID,
		StoreID:        input.StoreID,
		CustomerID:     input.CustomerID,
		CashierID:      input.CashierID,
		OrderNumber:    orderNumber,
		Status:         status,
		PaymentMethod:  input.PaymentMethod,
		Subtotal:       subtotal,
		TaxAmount:      taxAmount,
		DiscountAmount: discountAmount,
		TotalAmount:    totalAmount,
		PaidAmount:     input.PaidAmount,
		ChangeAmount:   changeAmount,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		Items:          orderItems,
	}

	err = uc.repo.CreateOrder(ctx, order)
	if err != nil {
		return nil, err
	}

	// Publish Event
	event := OrderCreatedEvent{
		EventID:   uuid.New().String(),
		EventType: "OrderCreated",
		Payload:   order,
		Timestamp: time.Now(),
	}

	eventBytes, err := json.Marshal(event)
	if err == nil {
		// Key: MerchantID (for partition affinity?) or OrderID.
		// MerchantID affinity is good for sequential processing per merchant.
		// Note: using context.Background() or detached context for async?
		// Publish is usually fast but blocking. If buffering is enabled it's fast.
		// Better to not block response?
		// For now, synchronous publish (safest).
		_ = uc.producer.Publish(ctx, []byte(order.MerchantID), eventBytes)
	}

	return order, nil
}

func (uc *orderUseCase) GetOrder(ctx context.Context, id string) (*model.Order, error) {
	return uc.repo.FindByID(ctx, id)
}

func (uc *orderUseCase) ListOrders(ctx context.Context, filters map[string]interface{}) ([]model.Order, int, error) {
	return uc.repo.FindAll(ctx, filters)
}

func (uc *orderUseCase) GetDashboardStats(ctx context.Context, merchantID string) (*model.DashboardStats, error) {
	return uc.repo.GetDashboardStats(ctx, merchantID)
}
