DROP INDEX IF EXISTS idx_loyalty_transactions_user_id;
DROP INDEX IF EXISTS idx_loyalty_transactions_order_id;
DROP INDEX IF EXISTS idx_user_balance_user_id;
DROP INDEX IF EXISTS idx_orders_user_id;

DROP TABLE IF EXISTS loyalty;
DROP TABLE IF EXISTS user_balance;
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS users;

DROP TYPE IF EXISTS loyalty_transaction_type;
DROP TYPE IF EXISTS order_status;