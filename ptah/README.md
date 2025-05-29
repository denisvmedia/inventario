# Ptah

**Ptah** is a schema management tool for relational databases, inspired by the ancient Egyptian god of creation. In
mythology, Ptah brought the world into existence through thought and speech—shaping order from chaos. This tool follows
a similar philosophy: it turns structured Go code into coherent, executable database schemas, ensuring consistency
between code and data.

The name **Ptah** is also an acronym:

> **P.T.A.H.** — *Parse, Transform, Apply, Harmonize*

- **Parse** – extract schema definitions from annotated Go structs
- **Transform** – generate SQL DDL and schema diffs
- **Apply** – execute up/down migrations with version tracking
- **Harmonize** – synchronize code-defined schema with actual database state

---

## Key Features

`ptah` provides a unified workflow to define, evolve, and apply database schemas based on Go code annotations. Its main
capabilities include:

- 📘 **Go Struct Parsing**  
  Extracts tables, columns, indexes, foreign keys, and constraints from structured comments in Go code.

- 🧱 **Schema Generation (DDL)**  
  Builds platform-specific `CREATE TABLE`, `CREATE INDEX`, and other DDL statements.

- 🔍 **Database Introspection**  
  Reads the current schema directly from Postgres or MySQL for comparison and analysis.

- 🧮 **Schema Diffing**  
  Compares code-based schema with the live database schema using AST representations.

- 🪄 **Migration Generation**  
  Automatically generates `up` and `down` SQL migrations to bring the database in sync.

- 🚀 **Migration Execution**  
  Applies versioned migrations in both directions, tracking state via a migrations table.

- 💥 **Database Cleaning**  
  Drops all user-defined schema objects—useful for testing or re-initialization.

---

## Example Usage

TBD
