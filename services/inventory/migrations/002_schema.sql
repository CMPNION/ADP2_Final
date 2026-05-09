-- full schema for inventory service

CREATE TABLE IF NOT EXISTS warehouses (
  id UUID PRIMARY KEY,
  name TEXT NOT NULL,
  location TEXT,
  is_active BOOLEAN DEFAULT TRUE,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT now()
);

CREATE TABLE IF NOT EXISTS product_stocks (
  id UUID PRIMARY KEY,
  sku TEXT NOT NULL,
  warehouse_id UUID NOT NULL REFERENCES warehouses(id),
  total_qty BIGINT NOT NULL DEFAULT 0,
  reserved_qty BIGINT NOT NULL DEFAULT 0,
  safety_stock_level BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT now(),
  updated_at TIMESTAMP WITH TIME ZONE DEFAULT now(),
  UNIQUE (sku, warehouse_id)
);

CREATE TABLE IF NOT EXISTS stock_reservations (
  id UUID PRIMARY KEY,
  order_id TEXT NOT NULL,
  sku TEXT NOT NULL,
  warehouse_id UUID NOT NULL,
  quantity BIGINT NOT NULL,
  status TEXT NOT NULL,
  expires_at TIMESTAMP WITH TIME ZONE,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT now()
);

CREATE TABLE IF NOT EXISTS stock_movements (
  id UUID PRIMARY KEY,
  sku TEXT NOT NULL,
  warehouse_id UUID NOT NULL,
  type TEXT NOT NULL,
  quantity BIGINT NOT NULL,
  reference_id TEXT,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_product_stocks_sku ON product_stocks(sku);
CREATE INDEX IF NOT EXISTS idx_product_stocks_available ON product_stocks((total_qty - reserved_qty));
CREATE INDEX IF NOT EXISTS idx_stock_reservations_order ON stock_reservations(order_id);
CREATE INDEX IF NOT EXISTS idx_stock_movements_sku ON stock_movements(sku);
