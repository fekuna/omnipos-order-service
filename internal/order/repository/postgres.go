package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/fekuna/omnipos-order-service/internal/model"
	"github.com/jmoiron/sqlx"
)

type Repository interface {
	CreateOrder(ctx context.Context, order *model.Order) error
	FindByID(ctx context.Context, id string) (*model.Order, error)
	FindAll(ctx context.Context, filters map[string]interface{}) ([]model.Order, int, error)
	GetDashboardStats(ctx context.Context, merchantID string) (*model.DashboardStats, error)
}

type postgresRepository struct {
	db *sqlx.DB
}

func NewPostgresRepository(db *sqlx.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) CreateOrder(ctx context.Context, order *model.Order) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Insert Order
	query := `
		INSERT INTO orders (
			id, merchant_id, store_id, customer_id, cashier_id,
			order_number, status, payment_method,
			subtotal, tax_amount, discount_amount, total_amount,
			paid_amount, change_amount, created_at, updated_at
		) VALUES (
			:id, :merchant_id, :store_id, :customer_id, :cashier_id,
			:order_number, :status, :payment_method,
			:subtotal, :tax_amount, :discount_amount, :total_amount,
			:paid_amount, :change_amount, :created_at, :updated_at
		)
	`
	_, err = tx.NamedExecContext(ctx, query, order)
	if err != nil {
		return fmt.Errorf("failed to insert order: %w", err)
	}

	// Insert Order Items
	if len(order.Items) > 0 {
		itemQuery := `
			INSERT INTO order_items (
				id, order_id, product_id, product_name,
				variant_id, variant_name, quantity,
				unit_price, total_price, notes, created_at
			) VALUES (
				:id, :order_id, :product_id, :product_name,
				:variant_id, :variant_name, :quantity,
				:unit_price, :total_price, :notes, :created_at
			)
		`
		_, err = tx.NamedExecContext(ctx, itemQuery, order.Items)
		if err != nil {
			return fmt.Errorf("failed to insert order items: %w", err)
		}
	}

	return tx.Commit()
}

func (r *postgresRepository) FindByID(ctx context.Context, id string) (*model.Order, error) {
	var order model.Order
	query := `SELECT * FROM orders WHERE id = $1`
	err := r.db.GetContext(ctx, &order, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	// Fetch items
	var items []model.OrderItem
	itemQuery := `SELECT * FROM order_items WHERE order_id = $1`
	err = r.db.SelectContext(ctx, &items, itemQuery, id)
	if err != nil {
		return nil, err
	}
	order.Items = items

	return &order, nil
}

func (r *postgresRepository) FindAll(ctx context.Context, filters map[string]interface{}) ([]model.Order, int, error) {
	query := `SELECT * FROM orders WHERE 1=1`
	countQuery := `SELECT COUNT(*) FROM orders WHERE 1=1`
	var args []interface{}
	argIdx := 1

	if merchantID, ok := filters["merchant_id"]; ok && merchantID != "" {
		query += fmt.Sprintf(" AND merchant_id = $%d", argIdx)
		countQuery += fmt.Sprintf(" AND merchant_id = $%d", argIdx)
		args = append(args, merchantID)
		argIdx++
	}

	if storeID, ok := filters["store_id"]; ok && storeID != "" {
		query += fmt.Sprintf(" AND store_id = $%d", argIdx)
		countQuery += fmt.Sprintf(" AND store_id = $%d", argIdx)
		args = append(args, storeID)
		argIdx++
	}

	// Sort
	query += " ORDER BY created_at DESC"

	// Pagination
	limit := 20
	offset := 0
	if l, ok := filters["limit"].(int); ok && l > 0 {
		limit = l
	}
	if o, ok := filters["offset"].(int); ok && o >= 0 {
		offset = o
	}

	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, offset)

	var orders []model.Order
	err := r.db.SelectContext(ctx, &orders, query, args...)
	if err != nil {
		return nil, 0, err
	}

	// Count
	var total int
	// Use arguments without limit/offset for count
	countArgs := args[:argIdx-1]
	err = r.db.GetContext(ctx, &total, countQuery, countArgs...)
	if err != nil {
		return nil, 0, err
	}

	// Ideally populate items for each order, but for list view maybe not needed for performance?
	// Let's populate for now or keep it light. The types suggest items are part of Order.
	// Loop and fetch items? Or join?
	// Join is better but complex mapping with sqlx. Simple loop for now or skip items in list.
	// Skip items in list for performance.

	return orders, total, nil
}

func (r *postgresRepository) GetDashboardStats(ctx context.Context, merchantID string) (*model.DashboardStats, error) {
	stats := &model.DashboardStats{}

	// 1. Overview Stats
	type Overview struct {
		TotalRevenue float64 `db:"total_revenue"`
		TotalOrders  int     `db:"total_orders"`
	}
	overviewQuery := `
		SELECT 
			COALESCE(SUM(total_amount), 0) as total_revenue,
			COUNT(id) as total_orders
		FROM orders 
		WHERE merchant_id = $1 AND status = 'ORDER_STATUS_PAID'
	`
	// Note: 'ORDER_STATUS_PAID' string must match DB enum or string value.
	// In E2E verification it returned 10000 revenue for PAID status, so check string carefully.
	// proto enum usually maps to string if not using int in DB.
	// Let's assume 'ORDER_STATUS_PAID' is correct based on other queries or ensure it matches.

	var ov Overview
	err := r.db.GetContext(ctx, &ov, overviewQuery, merchantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get overview stats: %w", err)
	}
	stats.TotalRevenue = ov.TotalRevenue
	stats.TotalOrders = ov.TotalOrders

	// 2. Sales Chart (Last 7 days)
	chartQuery := `
		SELECT 
			TO_CHAR(created_at, 'YYYY-MM-DD') as date,
			COALESCE(SUM(total_amount), 0) as total
		FROM orders
		WHERE merchant_id = $1 AND status = 'ORDER_STATUS_PAID' 
			AND created_at >= NOW() - INTERVAL '7 days'
		GROUP BY 1
		ORDER BY 1 ASC
	`
	var chart []model.DailySales
	err = r.db.SelectContext(ctx, &chart, chartQuery, merchantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get sales chart: %w", err)
	}
	stats.SalesChart = chart

	// 3. Top Products
	topQuery := `
		SELECT
			oi.product_name,
			COUNT(oi.id) as sales_count,
			COALESCE(SUM(oi.total_price), 0) as revenue
		FROM order_items oi
		JOIN orders o ON oi.order_id = o.id
		WHERE o.merchant_id = $1 AND o.status = 'ORDER_STATUS_PAID'
		GROUP BY oi.product_name
		ORDER BY sales_count DESC
		LIMIT 5
	`
	var top []model.TopProduct
	err = r.db.SelectContext(ctx, &top, topQuery, merchantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get top products: %w", err)
	}
	stats.TopProducts = top

	// 4. Recent Transactions
	recentQuery := `SELECT * FROM orders WHERE merchant_id = $1 ORDER BY created_at DESC LIMIT 5`
	var recent []model.Order
	err = r.db.SelectContext(ctx, &recent, recentQuery, merchantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent transactions: %w", err)
	}
	stats.RecentTransactions = recent

	return stats, nil
}
