# Bootstrap SQL Migrations

These are custom SQL migration files that must be run under a privileged database user, either manually or using inventario automations.

## Key Characteristics

- **Idempotent**: All statements can be run multiple times safely
- **Privileged Access Required**: Requires elevated database privileges
- **Prerequisite**: Must run before regular Ptah migrations

## File Naming Convention

Evolve bootstrap instructions by incrementing the filename prefix:
- `001_initial.sql`
- `002_next_bootstrap.sql`
- `003_another_bootstrap.sql`
- etc.

## Execution Order

1. **First**: Bootstrap migrations (this directory)
2. **Second**: Regular Ptah migrations

## Usage

Execute these files:
- Manually by a database administrator
- Automatically through inventario automation tools

**Important**: Ensure you have the necessary database privileges before running these migrations.
