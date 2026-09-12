-- +goose Up
CREATE TABLE users (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    login         TEXT        NOT NULL UNIQUE,
    password_hash TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE balances (
    user_id   BIGINT        PRIMARY KEY REFERENCES users (id),
    current   NUMERIC(14, 2) NOT NULL DEFAULT 0 CHECK (current >= 0),
    withdrawn NUMERIC(14, 2) NOT NULL DEFAULT 0 CHECK (withdrawn >= 0)
);

CREATE TABLE orders (
    number      TEXT        PRIMARY KEY,
    user_id     BIGINT      NOT NULL REFERENCES users (id),
    status      TEXT        NOT NULL DEFAULT 'NEW',
    accrual     NUMERIC(14, 2),
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX orders_user_uploaded_idx ON orders (user_id, uploaded_at DESC);
CREATE INDEX orders_pending_idx ON orders (uploaded_at) WHERE status IN ('NEW', 'PROCESSING');

CREATE TABLE withdrawals (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id      BIGINT        NOT NULL REFERENCES users (id),
    order_number TEXT          NOT NULL,
    sum          NUMERIC(14, 2) NOT NULL CHECK (sum > 0),
    processed_at TIMESTAMPTZ    NOT NULL DEFAULT now()
);

CREATE INDEX withdrawals_user_processed_idx ON withdrawals (user_id, processed_at DESC);

-- +goose Down
DROP TABLE withdrawals;
DROP TABLE orders;
DROP TABLE balances;
DROP TABLE users;
