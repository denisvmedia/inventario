-- Initial schema migration
-- This is an example migration file

CREATE TABLE IF NOT EXISTS example_table (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
