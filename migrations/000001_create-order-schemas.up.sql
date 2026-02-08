CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id UUID NOT NULL,
    store_id UUID,
    terminal_id UUID,
    order_number VARCHAR(50) NOT NULL, -- human-readable number
    customer_id UUID, -- NULL for walk-in customers
    user_id UUID NOT NULL, -- cashier/staff who created the order
    status VARCHAR(30) NOT NULL DEFAULT 'draft', -- draft, pending, completed, cancelled, refunded
    order_type VARCHAR(30) DEFAULT 'pos', -- pos, online, delivery
    subtotal DECIMAL(15,2) NOT NULL DEFAULT 0,
    tax_amount DECIMAL(15,2) DEFAULT 0,
    discount_amount DECIMAL(15,2) DEFAULT 0,
    total_amount DECIMAL(15,2) NOT NULL DEFAULT 0,
    paid_amount DECIMAL(15,2) DEFAULT 0,
    change_amount DECIMAL(15,2) DEFAULT 0,
    notes TEXT,
    completed_at TIMESTAMPTZ,
    cancelled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(merchant_id, order_number)
);
CREATE INDEX idx_orders_merchant_id ON orders(merchant_id);
CREATE INDEX idx_orders_store_id ON orders(store_id);
CREATE INDEX idx_orders_customer_id ON orders(customer_id);
CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_orders_created_at ON orders(created_at DESC);
CREATE INDEX idx_orders_number ON orders(order_number);

CREATE TABLE order_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id UUID NOT NULL,
    variant_id UUID,
    sku VARCHAR(100) NOT NULL,
    product_name VARCHAR(200) NOT NULL, -- denormalized for history
    quantity DECIMAL(15,3) NOT NULL,
    unit_price DECIMAL(15,2) NOT NULL,
    discount_amount DECIMAL(15,2) DEFAULT 0,
    tax_rate DECIMAL(5,2) DEFAULT 0,
    tax_amount DECIMAL(15,2) DEFAULT 0,
    subtotal DECIMAL(15,2) NOT NULL,
    total DECIMAL(15,2) NOT NULL,
    notes TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_order_items_order_id ON order_items(order_id);
CREATE INDEX idx_order_items_product_id ON order_items(product_id);

CREATE TABLE order_payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    payment_method VARCHAR(50) NOT NULL, -- cash, card, qr, wallet, etc.
    amount DECIMAL(15,2) NOT NULL,
    reference_number VARCHAR(100), -- card transaction ID, QR ref, etc.
    status VARCHAR(30) DEFAULT 'completed', -- pending, completed, failed, refunded
    metadata JSONB, -- additional payment gateway data
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_order_payments_order_id ON order_payments(order_id);
CREATE INDEX idx_order_payments_method ON order_payments(payment_method);
CREATE INDEX idx_order_payments_status ON order_payments(status);

CREATE TABLE refunds (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id UUID NOT NULL,
    original_order_id UUID NOT NULL REFERENCES orders(id),
    refund_order_id UUID, -- new order created for the refund
    refund_number VARCHAR(50) NOT NULL,
    reason VARCHAR(200),
    refund_amount DECIMAL(15,2) NOT NULL,
    refund_method VARCHAR(50) NOT NULL, -- same as payment, cash, store_credit
    status VARCHAR(30) DEFAULT 'pending', -- pending, approved, completed, rejected
    processed_by UUID, -- user_id
    processed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(merchant_id, refund_number)
);
CREATE INDEX idx_refunds_merchant_id ON refunds(merchant_id);
CREATE INDEX idx_refunds_original_order_id ON refunds(original_order_id);
CREATE INDEX idx_refunds_status ON refunds(status);
