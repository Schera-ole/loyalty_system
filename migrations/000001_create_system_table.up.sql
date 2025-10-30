CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE users (
    id UUID DEFAULT uuid_generate_v4() PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TYPE order_status AS ENUM (
    'NEW',
    'PROCESSING',
    'INVALID',
    'PROCESSED'
);

CREATE TABLE orders (
    id UUID DEFAULT uuid_generate_v4() PRIMARY KEY,
    order_number VARCHAR(255) UNIQUE NOT NULL,
    user_id UUID NOT NULL REFERENCES users(id),
    status order_status NOT NULL,
    accrual INTEGER,
    uploaded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE user_balance (
    id UUID DEFAULT uuid_generate_v4() PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    balance NUMERIC(10,2) DEFAULT 0 CHECK (balance >= 0),
    total_spent NUMERIC(10,2) DEFAULT 0 CHECK (total_spent >= 0),
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TYPE loyalty_transaction_type AS ENUM (
    'earn',
    'spend'
);

CREATE TABLE loyalty_transactions (
    id UUID DEFAULT uuid_generate_v4() PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    order_id UUID REFERENCES orders(id), -- Can be null for withdrawals
    order_number VARCHAR(255), -- Store order number for withdrawals
    points NUMERIC(10,2) NOT NULL,
    transaction_type loyalty_transaction_type NOT NULL,
    processed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_orders_user_id ON orders(user_id);
CREATE INDEX idx_user_balance_user_id ON user_balance(user_id);
CREATE INDEX idx_loyalty_transactions_user_id ON loyalty_transactions(user_id);
CREATE INDEX idx_loyalty_transactions_order_id ON loyalty_transactions(order_id);