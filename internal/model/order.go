package model

import (
	"time"
)

type OrderStatus string
type PaymentMethod string

const (
	OrderStatusPending   OrderStatus = "PENDING"
	OrderStatusPaid      OrderStatus = "PAID"
	OrderStatusCancelled OrderStatus = "CANCELLED"
	OrderStatusRefunded  OrderStatus = "REFUNDED"

	PaymentMethodCash   PaymentMethod = "CASH"
	PaymentMethodQRIS   PaymentMethod = "QRIS"
	PaymentMethodDebit  PaymentMethod = "DEBIT"
	PaymentMethodCredit PaymentMethod = "CREDIT"
)

type Order struct {
	ID             string        `db:"id" json:"id"`
	MerchantID     string        `db:"merchant_id" json:"merchant_id"`
	StoreID        string        `db:"store_id" json:"store_id"`
	CustomerID     *string       `db:"customer_id" json:"customer_id"` // Nullable
	CashierID      *string       `db:"cashier_id" json:"cashier_id"`   // Nullable
	OrderNumber    string        `db:"order_number" json:"order_number"`
	Status         OrderStatus   `db:"status" json:"status"`
	PaymentMethod  PaymentMethod `db:"payment_method" json:"payment_method"`
	Subtotal       float64       `db:"subtotal" json:"subtotal"`
	TaxAmount      float64       `db:"tax_amount" json:"tax_amount"`
	DiscountAmount float64       `db:"discount_amount" json:"discount_amount"`
	TotalAmount    float64       `db:"total_amount" json:"total_amount"`
	PaidAmount     float64       `db:"paid_amount" json:"paid_amount"`
	ChangeAmount   float64       `db:"change_amount" json:"change_amount"`
	CreatedAt      time.Time     `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time     `db:"updated_at" json:"updated_at"`

	Items []OrderItem `db:"-" json:"items"` // Populated manually
}

type OrderItem struct {
	ID          string    `db:"id" json:"id"`
	OrderID     string    `db:"order_id" json:"order_id"`
	ProductID   string    `db:"product_id" json:"product_id"`
	ProductName string    `db:"product_name" json:"product_name"`
	VariantID   *string   `db:"variant_id" json:"variant_id"`     // Nullable
	VariantName *string   `db:"variant_name" json:"variant_name"` // Nullable
	Quantity    float64   `db:"quantity" json:"quantity"`
	UnitPrice   float64   `db:"unit_price" json:"unit_price"`
	TotalPrice  float64   `db:"total_price" json:"total_price"`
	Notes       *string   `db:"notes" json:"notes"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
}

type DailySales struct {
	Date  string  `db:"date"`
	Total float64 `db:"total"`
}

type TopProduct struct {
	ProductName string  `db:"product_name"`
	SalesCount  int     `db:"sales_count"`
	Revenue     float64 `db:"revenue"`
}

type DashboardStats struct {
	TotalRevenue       float64
	TotalOrders        int
	TotalItemsSold     int
	SalesChart         []DailySales
	TopProducts        []TopProduct
	RecentTransactions []Order
}
