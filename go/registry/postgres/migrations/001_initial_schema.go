package migrations

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// InitialSchemaMigration returns the initial schema migration
func InitialSchemaMigration() *Migration {
	return &Migration{
		Version:     1,
		Description: "Initial schema",
		Up: func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `
				CREATE TABLE IF NOT EXISTS locations (
					id TEXT PRIMARY KEY,
					name TEXT NOT NULL,
					address TEXT NOT NULL
				);

				CREATE TABLE IF NOT EXISTS areas (
					id TEXT PRIMARY KEY,
					name TEXT NOT NULL,
					location_id TEXT NOT NULL REFERENCES locations(id) ON DELETE CASCADE
				);

				CREATE TABLE IF NOT EXISTS commodities (
					id TEXT PRIMARY KEY,
					name TEXT NOT NULL,
					short_name TEXT,
					type TEXT NOT NULL,
					area_id TEXT NOT NULL REFERENCES areas(id) ON DELETE CASCADE,
					count INTEGER NOT NULL DEFAULT 1,
					original_price DECIMAL(15,2),
					original_price_currency TEXT,
					converted_original_price DECIMAL(15,2),
					current_price DECIMAL(15,2),
					serial_number TEXT,
					extra_serial_numbers JSONB,
					part_numbers JSONB,
					tags JSONB,
					status TEXT NOT NULL,
					purchase_date TEXT,
					registered_date TEXT,
					last_modified_date TEXT,
					urls JSONB,
					comments TEXT,
					draft BOOLEAN NOT NULL DEFAULT FALSE
				);

				CREATE TABLE IF NOT EXISTS images (
					id TEXT PRIMARY KEY,
					commodity_id TEXT NOT NULL REFERENCES commodities(id) ON DELETE CASCADE,
					path TEXT NOT NULL,
					original_path TEXT NOT NULL,
					ext TEXT NOT NULL,
					mime_type TEXT NOT NULL
				);

				CREATE TABLE IF NOT EXISTS invoices (
					id TEXT PRIMARY KEY,
					commodity_id TEXT NOT NULL REFERENCES commodities(id) ON DELETE CASCADE,
					path TEXT NOT NULL,
					original_path TEXT NOT NULL,
					ext TEXT NOT NULL,
					mime_type TEXT NOT NULL
				);

				CREATE TABLE IF NOT EXISTS manuals (
					id TEXT PRIMARY KEY,
					commodity_id TEXT NOT NULL REFERENCES commodities(id) ON DELETE CASCADE,
					path TEXT NOT NULL,
					original_path TEXT NOT NULL,
					ext TEXT NOT NULL,
					mime_type TEXT NOT NULL
				);

				CREATE TABLE IF NOT EXISTS settings (
					name TEXT PRIMARY KEY,
					value JSONB NOT NULL
				);
			`)
			return err
		},
		Down: func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `
				DROP TABLE IF EXISTS settings;
				DROP TABLE IF EXISTS manuals;
				DROP TABLE IF EXISTS invoices;
				DROP TABLE IF EXISTS images;
				DROP TABLE IF EXISTS commodities;
				DROP TABLE IF EXISTS areas;
				DROP TABLE IF EXISTS locations;
			`)
			return err
		},
	}
}
