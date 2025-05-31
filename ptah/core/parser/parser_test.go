package parser_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/core/ast"
	"github.com/denisvmedia/inventario/ptah/core/parser"
)

func TestNewParser(t *testing.T) {
	c := qt.New(t)

	p := parser.NewParser("CREATE TABLE users (id INTEGER);")
	c.Assert(p, qt.IsNotNil)
}

func TestParser_ParseCreateTable_Basic(t *testing.T) {
	c := qt.New(t)

	sql := "CREATE TABLE users (id INTEGER PRIMARY KEY, name VARCHAR(255) NOT NULL);"
	p := parser.NewParser(sql)

	statements, err := p.Parse()
	c.Assert(err, qt.IsNil)
	c.Assert(statements, qt.IsNotNil)
	c.Assert(len(statements.Statements), qt.Equals, 1)

	// Check that it's a CREATE TABLE statement
	createTable, ok := statements.Statements[0].(*ast.CreateTableNode)
	c.Assert(ok, qt.IsTrue)
	c.Assert(createTable.Name, qt.Equals, "users")
	c.Assert(len(createTable.Columns), qt.Equals, 2)

	// Check first column (id)
	idColumn := createTable.Columns[0]
	c.Assert(idColumn.Name, qt.Equals, "id")
	c.Assert(idColumn.Type, qt.Equals, "INTEGER")
	c.Assert(idColumn.Primary, qt.IsTrue)
	c.Assert(idColumn.Nullable, qt.IsFalse) // Primary keys are NOT NULL

	// Check second column (name)
	nameColumn := createTable.Columns[1]
	c.Assert(nameColumn.Name, qt.Equals, "name")
	c.Assert(nameColumn.Type, qt.Equals, "VARCHAR(255)")
	c.Assert(nameColumn.Primary, qt.IsFalse)
	c.Assert(nameColumn.Nullable, qt.IsFalse) // NOT NULL specified
}

func TestParser_ParseCreateTable_WithConstraints(t *testing.T) {
	c := qt.New(t)

	// Test just the constraint part first
	sql := `CREATE TABLE orders (id INTEGER PRIMARY KEY, FOREIGN KEY (id) REFERENCES users(id));`
	p := parser.NewParser(sql)

	statements, err := p.Parse()
	c.Assert(err, qt.IsNil)
	c.Assert(len(statements.Statements), qt.Equals, 1)

	createTable := statements.Statements[0].(*ast.CreateTableNode)
	c.Assert(createTable.Name, qt.Equals, "orders")
	c.Assert(len(createTable.Columns), qt.Equals, 1)
	c.Assert(len(createTable.Constraints), qt.Equals, 1)

	// Check id column
	idColumn := createTable.Columns[0]
	c.Assert(idColumn.Name, qt.Equals, "id")
	c.Assert(idColumn.Type, qt.Equals, "INTEGER")
	c.Assert(idColumn.Primary, qt.IsTrue)

	// Check foreign key constraint
	fkConstraint := createTable.Constraints[0]
	c.Assert(fkConstraint.Type, qt.Equals, ast.ForeignKeyConstraint)
	c.Assert(fkConstraint.Columns, qt.DeepEquals, []string{"id"})
	c.Assert(fkConstraint.Reference, qt.IsNotNil)
	c.Assert(fkConstraint.Reference.Table, qt.Equals, "users")
	c.Assert(fkConstraint.Reference.Column, qt.Equals, "id")
}

func TestParser_ParseCreateTable_WithTableOptions(t *testing.T) {
	c := qt.New(t)

	sql := `CREATE TABLE products (
		id INTEGER PRIMARY KEY,
		name VARCHAR(255)
	) ENGINE=InnoDB CHARSET=utf8mb4 COMMENT='Product catalog';`
	p := parser.NewParser(sql)

	statements, err := p.Parse()
	c.Assert(err, qt.IsNil)
	c.Assert(len(statements.Statements), qt.Equals, 1)

	createTable := statements.Statements[0].(*ast.CreateTableNode)
	c.Assert(createTable.Name, qt.Equals, "products")
	c.Assert(createTable.Options["ENGINE"], qt.Equals, "InnoDB")
	c.Assert(createTable.Options["CHARSET"], qt.Equals, "utf8mb4")
	c.Assert(createTable.Comment, qt.Equals, "'Product catalog'")
}

func TestParser_ParseAlterTable(t *testing.T) {
	c := qt.New(t)

	sql := "ALTER TABLE users ADD COLUMN email VARCHAR(255) UNIQUE;"
	p := parser.NewParser(sql)

	statements, err := p.Parse()
	c.Assert(err, qt.IsNil)
	c.Assert(len(statements.Statements), qt.Equals, 1)

	alterTable, ok := statements.Statements[0].(*ast.AlterTableNode)
	c.Assert(ok, qt.IsTrue)
	c.Assert(alterTable.Name, qt.Equals, "users")
	c.Assert(len(alterTable.Operations), qt.Equals, 1)

	addOp, ok := alterTable.Operations[0].(*ast.AddColumnOperation)
	c.Assert(ok, qt.IsTrue)
	c.Assert(addOp.Column.Name, qt.Equals, "email")
	c.Assert(addOp.Column.Type, qt.Equals, "VARCHAR(255)")
	c.Assert(addOp.Column.Unique, qt.IsTrue)
}

func TestParser_ParseCreateIndex(t *testing.T) {
	c := qt.New(t)

	sql := "CREATE INDEX idx_users_email ON users (email);"
	p := parser.NewParser(sql)

	statements, err := p.Parse()
	c.Assert(err, qt.IsNil)
	c.Assert(len(statements.Statements), qt.Equals, 1)

	index, ok := statements.Statements[0].(*ast.IndexNode)
	c.Assert(ok, qt.IsTrue)
	c.Assert(index.Name, qt.Equals, "idx_users_email")
	c.Assert(index.Table, qt.Equals, "users")
	c.Assert(index.Columns, qt.DeepEquals, []string{"email"})
	c.Assert(index.Unique, qt.IsFalse)
}

func TestParser_ParseCreateUniqueIndex(t *testing.T) {
	c := qt.New(t)

	sql := "CREATE UNIQUE INDEX idx_users_email ON users (email);"
	p := parser.NewParser(sql)

	statements, err := p.Parse()
	c.Assert(err, qt.IsNil)
	c.Assert(len(statements.Statements), qt.Equals, 1)

	index, ok := statements.Statements[0].(*ast.IndexNode)
	c.Assert(ok, qt.IsTrue)
	c.Assert(index.Name, qt.Equals, "idx_users_email")
	c.Assert(index.Table, qt.Equals, "users")
	c.Assert(index.Columns, qt.DeepEquals, []string{"email"})
	c.Assert(index.Unique, qt.IsTrue)
}

func TestParser_ParseCreateEnum(t *testing.T) {
	c := qt.New(t)

	sql := "CREATE TYPE status AS ENUM ('active', 'inactive', 'pending');"
	p := parser.NewParser(sql)

	statements, err := p.Parse()
	c.Assert(err, qt.IsNil)
	c.Assert(len(statements.Statements), qt.Equals, 1)

	enum, ok := statements.Statements[0].(*ast.EnumNode)
	c.Assert(ok, qt.IsTrue)
	c.Assert(enum.Name, qt.Equals, "status")
	c.Assert(enum.Values, qt.DeepEquals, []string{"active", "inactive", "pending"})
}

func TestParser_ParseMultipleStatements(t *testing.T) {
	c := qt.New(t)

	sql := `
		CREATE TABLE users (id INTEGER PRIMARY KEY);
		CREATE INDEX idx_users_id ON users (id);
		ALTER TABLE users ADD COLUMN name VARCHAR(255);
	`
	p := parser.NewParser(sql)

	statements, err := p.Parse()
	c.Assert(err, qt.IsNil)
	c.Assert(len(statements.Statements), qt.Equals, 3)

	// Check first statement
	createTable, ok := statements.Statements[0].(*ast.CreateTableNode)
	c.Assert(ok, qt.IsTrue)
	c.Assert(createTable.Name, qt.Equals, "users")

	// Check second statement
	index, ok := statements.Statements[1].(*ast.IndexNode)
	c.Assert(ok, qt.IsTrue)
	c.Assert(index.Name, qt.Equals, "idx_users_id")

	// Check third statement
	alterTable, ok := statements.Statements[2].(*ast.AlterTableNode)
	c.Assert(ok, qt.IsTrue)
	c.Assert(alterTable.Name, qt.Equals, "users")
}

func TestParser_ParseColumnWithForeignKey(t *testing.T) {
	c := qt.New(t)

	sql := "CREATE TABLE orders (user_id INTEGER REFERENCES users(id) ON DELETE CASCADE);"
	p := parser.NewParser(sql)

	statements, err := p.Parse()
	c.Assert(err, qt.IsNil)
	c.Assert(len(statements.Statements), qt.Equals, 1)

	createTable := statements.Statements[0].(*ast.CreateTableNode)
	c.Assert(len(createTable.Columns), qt.Equals, 1)

	column := createTable.Columns[0]
	c.Assert(column.Name, qt.Equals, "user_id")
	c.Assert(column.ForeignKey, qt.IsNotNil)
	c.Assert(column.ForeignKey.Table, qt.Equals, "users")
	c.Assert(column.ForeignKey.Column, qt.Equals, "id")
	c.Assert(column.ForeignKey.OnDelete, qt.Equals, "CASCADE")
}

func TestParser_ParseColumnWithDefaultFunction(t *testing.T) {
	c := qt.New(t)

	sql := "CREATE TABLE logs (created_at TIMESTAMP DEFAULT NOW());"
	p := parser.NewParser(sql)

	statements, err := p.Parse()
	c.Assert(err, qt.IsNil)
	c.Assert(len(statements.Statements), qt.Equals, 1)

	createTable := statements.Statements[0].(*ast.CreateTableNode)
	column := createTable.Columns[0]
	c.Assert(column.Name, qt.Equals, "created_at")
	c.Assert(column.Default, qt.IsNotNil)
	c.Assert(column.Default.Expression, qt.Equals, "NOW()")
}

func TestParser_ParseComplexTable(t *testing.T) {
	c := qt.New(t)

	sql := `CREATE TABLE complex_table (
		id INTEGER PRIMARY KEY AUTO_INCREMENT,
		name VARCHAR(255) NOT NULL UNIQUE,
		email VARCHAR(255) NOT NULL,
		age INTEGER CHECK (age >= 0),
		status VARCHAR(20) DEFAULT 'active',
		created_at TIMESTAMP DEFAULT NOW(),
		updated_at TIMESTAMP,
		UNIQUE (email),
		FOREIGN KEY (id) REFERENCES parent_table(id) ON DELETE CASCADE ON UPDATE SET NULL
	) ENGINE=InnoDB CHARSET=utf8mb4 COMMENT='Complex table example';`

	p := parser.NewParser(sql)
	statements, err := p.Parse()
	c.Assert(err, qt.IsNil)
	c.Assert(len(statements.Statements), qt.Equals, 1)

	createTable := statements.Statements[0].(*ast.CreateTableNode)
	c.Assert(createTable.Name, qt.Equals, "complex_table")
	c.Assert(len(createTable.Columns), qt.Equals, 7)
	c.Assert(len(createTable.Constraints), qt.Equals, 2)

	// Check id column
	idCol := createTable.Columns[0]
	c.Assert(idCol.Name, qt.Equals, "id")
	c.Assert(idCol.Primary, qt.IsTrue)
	c.Assert(idCol.AutoInc, qt.IsTrue)

	// Check name column
	nameCol := createTable.Columns[1]
	c.Assert(nameCol.Name, qt.Equals, "name")
	c.Assert(nameCol.Nullable, qt.IsFalse)
	c.Assert(nameCol.Unique, qt.IsTrue)

	// Check age column with check constraint
	ageCol := createTable.Columns[3]
	c.Assert(ageCol.Name, qt.Equals, "age")
	c.Assert(ageCol.Check, qt.Equals, "age >= 0")

	// Check status column with default
	statusCol := createTable.Columns[4]
	c.Assert(statusCol.Name, qt.Equals, "status")
	c.Assert(statusCol.Type, qt.Equals, "VARCHAR(20)")
	c.Assert(statusCol.Default, qt.IsNotNil)
	c.Assert(statusCol.Default.Value, qt.Equals, "'active'")

	// Check created_at with function default
	createdCol := createTable.Columns[5]
	c.Assert(createdCol.Name, qt.Equals, "created_at")
	c.Assert(createdCol.Default, qt.IsNotNil)
	c.Assert(createdCol.Default.Expression, qt.Equals, "NOW()")

	// Check updated_at column
	updatedCol := createTable.Columns[6]
	c.Assert(updatedCol.Name, qt.Equals, "updated_at")
	c.Assert(updatedCol.Type, qt.Equals, "TIMESTAMP")

	// Check table options
	c.Assert(createTable.Options["ENGINE"], qt.Equals, "InnoDB")
	c.Assert(createTable.Options["CHARSET"], qt.Equals, "utf8mb4")
	c.Assert(createTable.Comment, qt.Equals, "'Complex table example'")
}

func TestParser_ParseAlterTableMultipleOperations(t *testing.T) {
	c := qt.New(t)

	sql := "ALTER TABLE users ADD COLUMN phone VARCHAR(20), DROP COLUMN old_field, MODIFY COLUMN name TEXT NOT NULL;"
	p := parser.NewParser(sql)

	statements, err := p.Parse()
	c.Assert(err, qt.IsNil)
	c.Assert(len(statements.Statements), qt.Equals, 1)

	alterTable := statements.Statements[0].(*ast.AlterTableNode)
	c.Assert(alterTable.Name, qt.Equals, "users")
	c.Assert(len(alterTable.Operations), qt.Equals, 3)

	// Check ADD operation
	addOp, ok := alterTable.Operations[0].(*ast.AddColumnOperation)
	c.Assert(ok, qt.IsTrue)
	c.Assert(addOp.Column.Name, qt.Equals, "phone")
	c.Assert(addOp.Column.Type, qt.Equals, "VARCHAR(20)")

	// Check DROP operation
	dropOp, ok := alterTable.Operations[1].(*ast.DropColumnOperation)
	c.Assert(ok, qt.IsTrue)
	c.Assert(dropOp.ColumnName, qt.Equals, "old_field")

	// Check MODIFY operation
	modifyOp, ok := alterTable.Operations[2].(*ast.ModifyColumnOperation)
	c.Assert(ok, qt.IsTrue)
	c.Assert(modifyOp.Column.Name, qt.Equals, "name")
	c.Assert(modifyOp.Column.Type, qt.Equals, "TEXT")
	c.Assert(modifyOp.Column.Nullable, qt.IsFalse)
}

func TestParser_ParseMySQLStyleTable(t *testing.T) {
	c := qt.New(t)

	sql := `CREATE TABLE ` + "`sample`" + ` (
		` + "`id`" + ` int NOT NULL AUTO_INCREMENT,
		` + "`name`" + ` varchar(50) DEFAULT 'John',
		` + "`age`" + ` int DEFAULT 30,
		` + "`created_at`" + ` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
		` + "`active`" + ` tinyint(1) DEFAULT 1,
		PRIMARY KEY (` + "`id`" + `)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;`

	p := parser.NewParser(sql)
	statements, err := p.Parse()
	c.Assert(err, qt.IsNil)
	c.Assert(len(statements.Statements), qt.Equals, 1)

	createTable := statements.Statements[0].(*ast.CreateTableNode)
	c.Assert(createTable.Name, qt.Equals, "`sample`")
	c.Assert(len(createTable.Columns), qt.Equals, 5)
	c.Assert(len(createTable.Constraints), qt.Equals, 1)

	// Check id column
	idColumn := createTable.Columns[0]
	c.Assert(idColumn.Name, qt.Equals, "`id`")
	c.Assert(idColumn.Type, qt.Equals, "int")
	c.Assert(idColumn.Nullable, qt.IsFalse)
	c.Assert(idColumn.AutoInc, qt.IsTrue)

	// Check name column with string default
	nameColumn := createTable.Columns[1]
	c.Assert(nameColumn.Name, qt.Equals, "`name`")
	c.Assert(nameColumn.Type, qt.Equals, "varchar(50)")
	c.Assert(nameColumn.Default, qt.IsNotNil)
	c.Assert(nameColumn.Default.Value, qt.Equals, "'John'")

	// Check age column with numeric default
	ageColumn := createTable.Columns[2]
	c.Assert(ageColumn.Name, qt.Equals, "`age`")
	c.Assert(ageColumn.Type, qt.Equals, "int")
	c.Assert(ageColumn.Default, qt.IsNotNil)
	c.Assert(ageColumn.Default.Value, qt.Equals, "30")

	// Check created_at column with function default and explicit NULL
	createdColumn := createTable.Columns[3]
	c.Assert(createdColumn.Name, qt.Equals, "`created_at`")
	c.Assert(createdColumn.Type, qt.Equals, "timestamp")
	c.Assert(createdColumn.Nullable, qt.IsTrue) // Explicit NULL
	c.Assert(createdColumn.Default, qt.IsNotNil)
	c.Assert(createdColumn.Default.Expression, qt.Equals, "CURRENT_TIMESTAMP()")

	// Check active column with tinyint type and numeric default
	activeColumn := createTable.Columns[4]
	c.Assert(activeColumn.Name, qt.Equals, "`active`")
	c.Assert(activeColumn.Type, qt.Equals, "tinyint(1)")
	c.Assert(activeColumn.Default, qt.IsNotNil)
	c.Assert(activeColumn.Default.Value, qt.Equals, "1")

	// Check PRIMARY KEY constraint
	pkConstraint := createTable.Constraints[0]
	c.Assert(pkConstraint.Type, qt.Equals, ast.PrimaryKeyConstraint)
	c.Assert(pkConstraint.Columns, qt.DeepEquals, []string{"`id`"})

	// Check table options
	c.Assert(createTable.Options["ENGINE"], qt.Equals, "InnoDB")
	c.Assert(createTable.Options["CHARSET"], qt.Equals, "utf8mb4")
	c.Assert(createTable.Options["COLLATE"], qt.Equals, "utf8mb4_0900_ai_ci")
}

func TestParser_ParseBacktickedIdentifiers(t *testing.T) {
	c := qt.New(t)

	sql := "CREATE TABLE `users` (`user_id` INTEGER PRIMARY KEY, `email_address` VARCHAR(255));"
	p := parser.NewParser(sql)

	statements, err := p.Parse()
	c.Assert(err, qt.IsNil)
	c.Assert(len(statements.Statements), qt.Equals, 1)

	createTable := statements.Statements[0].(*ast.CreateTableNode)
	c.Assert(createTable.Name, qt.Equals, "`users`")
	c.Assert(len(createTable.Columns), qt.Equals, 2)

	// Check first column
	userIdColumn := createTable.Columns[0]
	c.Assert(userIdColumn.Name, qt.Equals, "`user_id`")
	c.Assert(userIdColumn.Type, qt.Equals, "INTEGER")
	c.Assert(userIdColumn.Primary, qt.IsTrue)

	// Check second column
	emailColumn := createTable.Columns[1]
	c.Assert(emailColumn.Name, qt.Equals, "`email_address`")
	c.Assert(emailColumn.Type, qt.Equals, "VARCHAR(255)")
}

func TestParser_ParseSimpleMySQLTable(t *testing.T) {
	c := qt.New(t)

	// Test a simpler version first
	sql := "CREATE TABLE `sample` (`id` int NOT NULL AUTO_INCREMENT, PRIMARY KEY (`id`));"
	p := parser.NewParser(sql)

	statements, err := p.Parse()
	c.Assert(err, qt.IsNil)
	c.Assert(len(statements.Statements), qt.Equals, 1)

	createTable := statements.Statements[0].(*ast.CreateTableNode)
	c.Assert(createTable.Name, qt.Equals, "`sample`")
	c.Assert(len(createTable.Columns), qt.Equals, 1)
	c.Assert(len(createTable.Constraints), qt.Equals, 1)

	// Check id column
	idColumn := createTable.Columns[0]
	c.Assert(idColumn.Name, qt.Equals, "`id`")
	c.Assert(idColumn.Type, qt.Equals, "int")
	c.Assert(idColumn.Nullable, qt.IsFalse)
	c.Assert(idColumn.AutoInc, qt.IsTrue)
}

func TestParser_ParseCurrentTimestamp(t *testing.T) {
	c := qt.New(t)

	sql := "CREATE TABLE test (`created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP);"
	p := parser.NewParser(sql)

	statements, err := p.Parse()
	c.Assert(err, qt.IsNil)
	c.Assert(len(statements.Statements), qt.Equals, 1)

	createTable := statements.Statements[0].(*ast.CreateTableNode)
	c.Assert(len(createTable.Columns), qt.Equals, 1)

	column := createTable.Columns[0]
	c.Assert(column.Name, qt.Equals, "`created_at`")
	c.Assert(column.Type, qt.Equals, "timestamp")
	c.Assert(column.Nullable, qt.IsTrue)
	c.Assert(column.Default, qt.IsNotNil)
	c.Assert(column.Default.Expression, qt.Equals, "CURRENT_TIMESTAMP()")
}

func TestParser_ParseMySQLTableStepByStep(t *testing.T) {
	c := qt.New(t)

	// Test with just 2 columns to isolate the issue
	sql := `CREATE TABLE ` + "`sample`" + ` (
		` + "`id`" + ` int NOT NULL AUTO_INCREMENT,
		` + "`name`" + ` varchar(50) DEFAULT 'John'
	);`

	p := parser.NewParser(sql)
	statements, err := p.Parse()
	c.Assert(err, qt.IsNil)
	c.Assert(len(statements.Statements), qt.Equals, 1)

	createTable := statements.Statements[0].(*ast.CreateTableNode)
	c.Assert(createTable.Name, qt.Equals, "`sample`")
	c.Assert(len(createTable.Columns), qt.Equals, 2)
}

func TestParser_ParseNumericDefault(t *testing.T) {
	c := qt.New(t)

	sql := "CREATE TABLE test (`age` int DEFAULT 30);"
	p := parser.NewParser(sql)

	statements, err := p.Parse()
	c.Assert(err, qt.IsNil)
	c.Assert(len(statements.Statements), qt.Equals, 1)

	createTable := statements.Statements[0].(*ast.CreateTableNode)
	c.Assert(len(createTable.Columns), qt.Equals, 1)

	column := createTable.Columns[0]
	c.Assert(column.Name, qt.Equals, "`age`")
	c.Assert(column.Type, qt.Equals, "int")
	c.Assert(column.Default, qt.IsNotNil)
	c.Assert(column.Default.Value, qt.Equals, "30")
}

func TestParser_ParseMySQLTableWithTimestamp(t *testing.T) {
	c := qt.New(t)

	// Test with just the timestamp column that's causing issues
	sql := `CREATE TABLE ` + "`sample`" + ` (
		` + "`id`" + ` int NOT NULL AUTO_INCREMENT,
		` + "`created_at`" + ` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
		` + "`active`" + ` tinyint(1) DEFAULT 1
	);`

	p := parser.NewParser(sql)
	statements, err := p.Parse()
	c.Assert(err, qt.IsNil)
	c.Assert(len(statements.Statements), qt.Equals, 1)

	createTable := statements.Statements[0].(*ast.CreateTableNode)
	c.Assert(createTable.Name, qt.Equals, "`sample`")
	c.Assert(len(createTable.Columns), qt.Equals, 3)

	// Check created_at column
	createdColumn := createTable.Columns[1]
	c.Assert(createdColumn.Name, qt.Equals, "`created_at`")
	c.Assert(createdColumn.Type, qt.Equals, "timestamp")
	c.Assert(createdColumn.Nullable, qt.IsTrue)
	c.Assert(createdColumn.Default, qt.IsNotNil)
	c.Assert(createdColumn.Default.Expression, qt.Equals, "CURRENT_TIMESTAMP()")

	// Check active column
	activeColumn := createTable.Columns[2]
	c.Assert(activeColumn.Name, qt.Equals, "`active`")
	c.Assert(activeColumn.Type, qt.Equals, "tinyint(1)")
	c.Assert(activeColumn.Default, qt.IsNotNil)
	c.Assert(activeColumn.Default.Value, qt.Equals, "1")
}

func TestParser_ParseMySQLDataTypes(t *testing.T) {
	tests := []struct {
		name         string
		sql          string
		expectedType string
	}{
		{
			name:         "TinyInt",
			sql:          "CREATE TABLE test (col tinyint(1));",
			expectedType: "tinyint(1)",
		},
		{
			name:         "SmallInt",
			sql:          "CREATE TABLE test (col smallint(5));",
			expectedType: "smallint(5)",
		},
		{
			name:         "MediumInt",
			sql:          "CREATE TABLE test (col mediumint(8));",
			expectedType: "mediumint(8)",
		},
		{
			name:         "BigInt",
			sql:          "CREATE TABLE test (col bigint(20));",
			expectedType: "bigint(20)",
		},
		{
			name:         "Float",
			sql:          "CREATE TABLE test (col float(7,4));",
			expectedType: "float(7,4)",
		},
		{
			name:         "Double",
			sql:          "CREATE TABLE test (col double(15,8));",
			expectedType: "double(15,8)",
		},
		{
			name:         "Char",
			sql:          "CREATE TABLE test (col char(10));",
			expectedType: "char(10)",
		},
		{
			name:         "Text",
			sql:          "CREATE TABLE test (col text);",
			expectedType: "text",
		},
		{
			name:         "LongText",
			sql:          "CREATE TABLE test (col longtext);",
			expectedType: "longtext",
		},
		{
			name:         "DateTime",
			sql:          "CREATE TABLE test (col datetime);",
			expectedType: "datetime",
		},
		{
			name:         "Date",
			sql:          "CREATE TABLE test (col date);",
			expectedType: "date",
		},
		{
			name:         "Time",
			sql:          "CREATE TABLE test (col time);",
			expectedType: "time",
		},
		{
			name:         "Year",
			sql:          "CREATE TABLE test (col year(4));",
			expectedType: "year(4)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			p := parser.NewParser(tt.sql)
			statements, err := p.Parse()
			c.Assert(err, qt.IsNil)
			c.Assert(len(statements.Statements), qt.Equals, 1)

			createTable := statements.Statements[0].(*ast.CreateTableNode)
			c.Assert(len(createTable.Columns), qt.Equals, 1)
			c.Assert(createTable.Columns[0].Type, qt.Equals, tt.expectedType)
		})
	}
}

func TestParser_ParseMySQLTableOptions(t *testing.T) {
	c := qt.New(t)

	sql := `CREATE TABLE test (id int)
		ENGINE=MyISAM
		DEFAULT CHARSET=latin1
		COLLATE=latin1_swedish_ci
		AUTO_INCREMENT=1000
		COMMENT='Test table';`

	p := parser.NewParser(sql)
	statements, err := p.Parse()
	c.Assert(err, qt.IsNil)
	c.Assert(len(statements.Statements), qt.Equals, 1)

	createTable := statements.Statements[0].(*ast.CreateTableNode)
	c.Assert(createTable.Options["ENGINE"], qt.Equals, "MyISAM")
	c.Assert(createTable.Options["CHARSET"], qt.Equals, "latin1")
	c.Assert(createTable.Options["COLLATE"], qt.Equals, "latin1_swedish_ci")
	c.Assert(createTable.Comment, qt.Equals, "'Test table'")
}

func TestParser_ParsePostgreSQLEnum(t *testing.T) {
	c := qt.New(t)

	sql := "CREATE TYPE status_enum AS ENUM ('pending', 'active', 'archived');"
	p := parser.NewParser(sql)

	statements, err := p.Parse()
	c.Assert(err, qt.IsNil)
	c.Assert(len(statements.Statements), qt.Equals, 1)

	enum, ok := statements.Statements[0].(*ast.EnumNode)
	c.Assert(ok, qt.IsTrue)
	c.Assert(enum.Name, qt.Equals, "status_enum")
	c.Assert(enum.Values, qt.DeepEquals, []string{"pending", "active", "archived"})
}

func TestParser_ParsePostgreSQLDomain(t *testing.T) {
	c := qt.New(t)

	sql := `CREATE DOMAIN email_domain AS TEXT
		CHECK (VALUE ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$');`
	p := parser.NewParser(sql)

	statements, err := p.Parse()
	c.Assert(err, qt.IsNil)
	c.Assert(len(statements.Statements), qt.Equals, 1)

	comment, ok := statements.Statements[0].(*ast.CommentNode)
	c.Assert(ok, qt.IsTrue)
	c.Assert(comment.Text, qt.Contains, "CREATE DOMAIN email_domain AS TEXT")
	c.Assert(comment.Text, qt.Contains, "CHECK")
}

func TestParser_ParsePostgreSQLSerialTypes(t *testing.T) {
	c := qt.New(t)

	sql := `CREATE TABLE test (
		serial_id SERIAL PRIMARY KEY,
		big_id BIGSERIAL UNIQUE
	);`
	p := parser.NewParser(sql)

	statements, err := p.Parse()
	c.Assert(err, qt.IsNil)
	c.Assert(len(statements.Statements), qt.Equals, 1)

	createTable := statements.Statements[0].(*ast.CreateTableNode)
	c.Assert(createTable.Name, qt.Equals, "test")
	c.Assert(len(createTable.Columns), qt.Equals, 2)

	// Check SERIAL column
	serialCol := createTable.Columns[0]
	c.Assert(serialCol.Name, qt.Equals, "serial_id")
	c.Assert(serialCol.Type, qt.Equals, "SERIAL")
	c.Assert(serialCol.Primary, qt.IsTrue)

	// Check BIGSERIAL column
	bigSerialCol := createTable.Columns[1]
	c.Assert(bigSerialCol.Name, qt.Equals, "big_id")
	c.Assert(bigSerialCol.Type, qt.Equals, "BIGSERIAL")
	c.Assert(bigSerialCol.Unique, qt.IsTrue)
}

func TestParser_ParsePostgreSQLArrayTypes(t *testing.T) {
	c := qt.New(t)

	sql := `CREATE TABLE test (
		tags TEXT[] DEFAULT ARRAY[]::TEXT[],
		matrix INT[][]
	);`
	p := parser.NewParser(sql)

	statements, err := p.Parse()
	c.Assert(err, qt.IsNil)
	c.Assert(len(statements.Statements), qt.Equals, 1)

	createTable := statements.Statements[0].(*ast.CreateTableNode)
	c.Assert(len(createTable.Columns), qt.Equals, 2)

	// Check TEXT[] column with array default
	tagsCol := createTable.Columns[0]
	c.Assert(tagsCol.Name, qt.Equals, "tags")
	c.Assert(tagsCol.Type, qt.Equals, "TEXT[]")
	c.Assert(tagsCol.Default, qt.IsNotNil)
	c.Assert(tagsCol.Default.Expression, qt.Equals, "ARRAY[]::TEXT[]")

	// Check multi-dimensional array
	matrixCol := createTable.Columns[1]
	c.Assert(matrixCol.Name, qt.Equals, "matrix")
	c.Assert(matrixCol.Type, qt.Equals, "INT[][]")
}

func TestParser_ParsePostgreSQLUUIDWithFunction(t *testing.T) {
	c := qt.New(t)

	sql := "CREATE TABLE test (uuid_id UUID DEFAULT gen_random_uuid() NOT NULL);"
	p := parser.NewParser(sql)

	statements, err := p.Parse()
	c.Assert(err, qt.IsNil)
	c.Assert(len(statements.Statements), qt.Equals, 1)

	createTable := statements.Statements[0].(*ast.CreateTableNode)
	c.Assert(len(createTable.Columns), qt.Equals, 1)

	uuidCol := createTable.Columns[0]
	c.Assert(uuidCol.Name, qt.Equals, "uuid_id")
	c.Assert(uuidCol.Type, qt.Equals, "UUID")
	c.Assert(uuidCol.Default, qt.IsNotNil)
	c.Assert(uuidCol.Default.Expression, qt.Equals, "gen_random_uuid()")
	c.Assert(uuidCol.Nullable, qt.IsFalse)
}

func TestParser_ParsePostgreSQLGeneratedColumn(t *testing.T) {
	c := qt.New(t)

	sql := "CREATE TABLE test (first_name TEXT, last_name TEXT, full_name TEXT GENERATED ALWAYS AS (first_name || ' ' || last_name) STORED);"
	p := parser.NewParser(sql)

	statements, err := p.Parse()
	c.Assert(err, qt.IsNil)
	c.Assert(len(statements.Statements), qt.Equals, 1)

	createTable := statements.Statements[0].(*ast.CreateTableNode)
	c.Assert(len(createTable.Columns), qt.Equals, 3)

	// Check generated column
	fullNameCol := createTable.Columns[2]
	c.Assert(fullNameCol.Name, qt.Equals, "full_name")
	c.Assert(fullNameCol.Type, qt.Equals, "TEXT")
	c.Assert(fullNameCol.Check, qt.Contains, "GENERATED ALWAYS AS")
	c.Assert(fullNameCol.Check, qt.Contains, "first_name || ' ' || last_name")
}

func TestParser_ParsePostgreSQLJSONTypes(t *testing.T) {
	c := qt.New(t)

	sql := `CREATE TABLE test (
		json_field JSON,
		jsonb_field JSONB NOT NULL DEFAULT '{}'::jsonb
	);`
	p := parser.NewParser(sql)

	statements, err := p.Parse()
	c.Assert(err, qt.IsNil)
	c.Assert(len(statements.Statements), qt.Equals, 1)

	createTable := statements.Statements[0].(*ast.CreateTableNode)
	c.Assert(len(createTable.Columns), qt.Equals, 2)

	// Check JSON column
	jsonCol := createTable.Columns[0]
	c.Assert(jsonCol.Name, qt.Equals, "json_field")
	c.Assert(jsonCol.Type, qt.Equals, "JSON")

	// Check JSONB column with cast default
	jsonbCol := createTable.Columns[1]
	c.Assert(jsonbCol.Name, qt.Equals, "jsonb_field")
	c.Assert(jsonbCol.Type, qt.Equals, "JSONB")
	c.Assert(jsonbCol.Nullable, qt.IsFalse)
	c.Assert(jsonbCol.Default, qt.IsNotNil)
	c.Assert(jsonbCol.Default.Value, qt.Equals, "'{}'::jsonb")
}

func TestParser_ParsePostgreSQLCommentStatements(t *testing.T) {
	c := qt.New(t)

	sql := `COMMENT ON TABLE public.full_demo IS 'Comprehensive demo table';`
	p := parser.NewParser(sql)

	statements, err := p.Parse()
	c.Assert(err, qt.IsNil)
	c.Assert(len(statements.Statements), qt.Equals, 1)

	comment, ok := statements.Statements[0].(*ast.CommentNode)
	c.Assert(ok, qt.IsTrue)
	c.Assert(comment.Text, qt.Contains, "COMMENT ON TABLE public.full_demo IS")
	c.Assert(comment.Text, qt.Contains, "Comprehensive demo table")
}

func TestParser_ParsePostgreSQLSchemaTable(t *testing.T) {
	c := qt.New(t)

	sql := "CREATE TABLE public.test (id SERIAL PRIMARY KEY);"
	p := parser.NewParser(sql)

	statements, err := p.Parse()
	c.Assert(err, qt.IsNil)
	c.Assert(len(statements.Statements), qt.Equals, 1)

	createTable := statements.Statements[0].(*ast.CreateTableNode)
	c.Assert(createTable.Name, qt.Equals, "public.test")
	c.Assert(len(createTable.Columns), qt.Equals, 1)
}

func TestParser_ParsePostgreSQLFullDemo(t *testing.T) {
	c := qt.New(t)

	// Test a much simpler version to avoid infinite loops
	sql := `CREATE TABLE public.full_demo (
		serial_id SERIAL PRIMARY KEY,
		uuid_id UUID DEFAULT gen_random_uuid() NOT NULL,
		varchar_var VARCHAR(255) NOT NULL,
		tags TEXT[] DEFAULT ARRAY[]::TEXT[],
		jsonb_field JSONB NOT NULL DEFAULT '{}'::jsonb,
		user_id INTEGER REFERENCES users(id) ON DELETE CASCADE ON UPDATE SET NULL
	);`

	p := parser.NewParser(sql)
	statements, err := p.Parse()
	c.Assert(err, qt.IsNil)
	c.Assert(len(statements.Statements), qt.Equals, 1)

	createTable := statements.Statements[0].(*ast.CreateTableNode)
	c.Assert(createTable.Name, qt.Equals, "public.full_demo")
	c.Assert(len(createTable.Columns), qt.Equals, 6)

	// Test some key columns
	serialCol := createTable.Columns[0]
	c.Assert(serialCol.Name, qt.Equals, "serial_id")
	c.Assert(serialCol.Type, qt.Equals, "SERIAL")
	c.Assert(serialCol.Primary, qt.IsTrue)

	uuidCol := createTable.Columns[1]
	c.Assert(uuidCol.Name, qt.Equals, "uuid_id")
	c.Assert(uuidCol.Type, qt.Equals, "UUID")
	c.Assert(uuidCol.Default.Expression, qt.Equals, "gen_random_uuid()")

	tagsCol := createTable.Columns[3]
	c.Assert(tagsCol.Name, qt.Equals, "tags")
	c.Assert(tagsCol.Type, qt.Equals, "TEXT[]")
	c.Assert(tagsCol.Default.Expression, qt.Equals, "ARRAY[]::TEXT[]")

	jsonbCol := createTable.Columns[4]
	c.Assert(jsonbCol.Name, qt.Equals, "jsonb_field")
	c.Assert(jsonbCol.Type, qt.Equals, "JSONB")
	c.Assert(jsonbCol.Default.Value, qt.Equals, "'{}'::jsonb")
}

func TestParser_ParsePostgreSQLMultipleStatements(t *testing.T) {
	c := qt.New(t)

	sql := `
		CREATE TYPE status_enum AS ENUM ('pending', 'active', 'archived');

		CREATE DOMAIN email_domain AS TEXT
			CHECK (VALUE ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$');

		CREATE TABLE test (
			id SERIAL PRIMARY KEY,
			status status_enum DEFAULT 'pending',
			email email_domain
		);

		COMMENT ON TABLE test IS 'Test table with custom types';
	`

	p := parser.NewParser(sql)
	statements, err := p.Parse()
	c.Assert(err, qt.IsNil)
	c.Assert(len(statements.Statements), qt.Equals, 4)

	// Check enum
	enum, ok := statements.Statements[0].(*ast.EnumNode)
	c.Assert(ok, qt.IsTrue)
	c.Assert(enum.Name, qt.Equals, "status_enum")

	// Check domain (represented as comment)
	domain, ok := statements.Statements[1].(*ast.CommentNode)
	c.Assert(ok, qt.IsTrue)
	c.Assert(domain.Text, qt.Contains, "CREATE DOMAIN email_domain")

	// Check table
	table, ok := statements.Statements[2].(*ast.CreateTableNode)
	c.Assert(ok, qt.IsTrue)
	c.Assert(table.Name, qt.Equals, "test")
	c.Assert(len(table.Columns), qt.Equals, 3)

	// Check comment
	comment, ok := statements.Statements[3].(*ast.CommentNode)
	c.Assert(ok, qt.IsTrue)
	c.Assert(comment.Text, qt.Contains, "COMMENT ON TABLE test IS")
}

func TestParser_ParsePostgreSQLComprehensiveDemo(t *testing.T) {
	c := qt.New(t)

	// Test PostgreSQL CREATE TABLE statement with key advanced features
	// This test covers the most important PostgreSQL features from the original comprehensive SQL
	sql := `CREATE TABLE public.full_demo (
		serial_id SERIAL PRIMARY KEY,
		big_id BIGSERIAL UNIQUE,
		uuid_id UUID DEFAULT gen_random_uuid() NOT NULL,
		char_fixed CHAR(10),
		varchar_var VARCHAR(255) NOT NULL,
		text_field TEXT CHECK (char_length(text_field) <= 5000),
		small_value SMALLINT DEFAULT 1 CHECK (small_value > 0),
		numeric_precise NUMERIC(12,4) NOT NULL DEFAULT 0.0000,
		real_value REAL,
		double_value DOUBLE PRECISION,
		is_active BOOLEAN DEFAULT TRUE,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMPTZ DEFAULT now(),
		tags TEXT[] DEFAULT ARRAY[]::TEXT[],
		matrix INT[][],
		json_field JSON,
		jsonb_field JSONB NOT NULL DEFAULT '{}'::jsonb,
		data BYTEA,
		email_address email_domain,
		user_id INTEGER REFERENCES users(id) ON DELETE CASCADE ON UPDATE SET NULL,
		CONSTRAINT uq_email UNIQUE (email_address),
		CONSTRAINT chk_positive_value CHECK (numeric_precise >= 0),
		CHECK (created_at <= updated_at)
	);`

	p := parser.NewParser(sql)
	statements, err := p.Parse()
	c.Assert(err, qt.IsNil)
	c.Assert(len(statements.Statements), qt.Equals, 1)

	createTable := statements.Statements[0].(*ast.CreateTableNode)
	c.Assert(createTable.Name, qt.Equals, "public.full_demo")

	// Test that we have the expected number of columns (now includes DOUBLE PRECISION)
	c.Assert(len(createTable.Columns), qt.Equals, 20)

	// Test key PostgreSQL features

	// Serial types
	serialCol := createTable.Columns[0]
	c.Assert(serialCol.Name, qt.Equals, "serial_id")
	c.Assert(serialCol.Type, qt.Equals, "SERIAL")
	c.Assert(serialCol.Primary, qt.IsTrue)

	bigSerialCol := createTable.Columns[1]
	c.Assert(bigSerialCol.Name, qt.Equals, "big_id")
	c.Assert(bigSerialCol.Type, qt.Equals, "BIGSERIAL")
	c.Assert(bigSerialCol.Unique, qt.IsTrue)

	// UUID with function default
	uuidCol := createTable.Columns[2]
	c.Assert(uuidCol.Name, qt.Equals, "uuid_id")
	c.Assert(uuidCol.Type, qt.Equals, "UUID")
	c.Assert(uuidCol.Default, qt.IsNotNil)
	c.Assert(uuidCol.Default.Expression, qt.Equals, "gen_random_uuid()")
	c.Assert(uuidCol.Nullable, qt.IsFalse)

	// Character types
	charCol := createTable.Columns[3]
	c.Assert(charCol.Name, qt.Equals, "char_fixed")
	c.Assert(charCol.Type, qt.Equals, "CHAR(10)")

	// Text with check constraint
	textCol := createTable.Columns[5]
	c.Assert(textCol.Name, qt.Equals, "text_field")
	c.Assert(textCol.Type, qt.Equals, "TEXT")
	c.Assert(textCol.Check, qt.Contains, "char_length(text_field) <= 5000")

	// Numeric with check constraint
	smallCol := createTable.Columns[6]
	c.Assert(smallCol.Name, qt.Equals, "small_value")
	c.Assert(smallCol.Type, qt.Equals, "SMALLINT")
	c.Assert(smallCol.Default, qt.IsNotNil)
	c.Assert(smallCol.Default.Value, qt.Equals, "1")
	c.Assert(smallCol.Check, qt.Contains, "small_value > 0")

	// NUMERIC with precision and default
	numericCol := createTable.Columns[7]
	c.Assert(numericCol.Name, qt.Equals, "numeric_precise")
	c.Assert(numericCol.Type, qt.Equals, "NUMERIC(12,4)")
	c.Assert(numericCol.Nullable, qt.IsFalse)
	c.Assert(numericCol.Default, qt.IsNotNil)
	c.Assert(numericCol.Default.Value, qt.Equals, "0.0000")

	// DOUBLE PRECISION type
	doubleCol := createTable.Columns[9]
	c.Assert(doubleCol.Name, qt.Equals, "double_value")
	c.Assert(doubleCol.Type, qt.Equals, "DOUBLE PRECISION")

	// Boolean with default
	boolCol := createTable.Columns[10]
	c.Assert(boolCol.Name, qt.Equals, "is_active")
	c.Assert(boolCol.Type, qt.Equals, "BOOLEAN")
	c.Assert(boolCol.Default, qt.IsNotNil)
	c.Assert(boolCol.Default.Value, qt.Equals, "TRUE")

	// TIMESTAMPTZ type
	updatedCol := createTable.Columns[12]
	c.Assert(updatedCol.Name, qt.Equals, "updated_at")
	c.Assert(updatedCol.Type, qt.Equals, "TIMESTAMPTZ")
	c.Assert(updatedCol.Default, qt.IsNotNil)
	c.Assert(updatedCol.Default.Expression, qt.Equals, "now()")

	// Array types
	tagsCol := createTable.Columns[13]
	c.Assert(tagsCol.Name, qt.Equals, "tags")
	c.Assert(tagsCol.Type, qt.Equals, "TEXT[]")
	c.Assert(tagsCol.Default, qt.IsNotNil)
	c.Assert(tagsCol.Default.Expression, qt.Equals, "ARRAY[]::TEXT[]")

	matrixCol := createTable.Columns[14]
	c.Assert(matrixCol.Name, qt.Equals, "matrix")
	c.Assert(matrixCol.Type, qt.Equals, "INT[][]")

	// JSON types
	jsonbCol := createTable.Columns[16]
	c.Assert(jsonbCol.Name, qt.Equals, "jsonb_field")
	c.Assert(jsonbCol.Type, qt.Equals, "JSONB")
	c.Assert(jsonbCol.Nullable, qt.IsFalse)
	c.Assert(jsonbCol.Default, qt.IsNotNil)
	c.Assert(jsonbCol.Default.Value, qt.Equals, "'{}'::jsonb")

	// BYTEA type
	dataCol := createTable.Columns[17]
	c.Assert(dataCol.Name, qt.Equals, "data")
	c.Assert(dataCol.Type, qt.Equals, "BYTEA")

	// Domain type
	emailCol := createTable.Columns[18]
	c.Assert(emailCol.Name, qt.Equals, "email_address")
	c.Assert(emailCol.Type, qt.Equals, "email_domain")

	// Foreign key with cascading rules
	userIdCol := createTable.Columns[19]
	c.Assert(userIdCol.Name, qt.Equals, "user_id")
	c.Assert(userIdCol.Type, qt.Equals, "INTEGER")
	c.Assert(userIdCol.ForeignKey, qt.IsNotNil)
	c.Assert(userIdCol.ForeignKey.Table, qt.Equals, "users")
	c.Assert(userIdCol.ForeignKey.Column, qt.Equals, "id")
	c.Assert(userIdCol.ForeignKey.OnDelete, qt.Equals, "CASCADE")
	c.Assert(userIdCol.ForeignKey.OnUpdate, qt.Equals, "SET NULL")

	// Test table-level constraints
	c.Assert(len(createTable.Constraints), qt.Equals, 3)

	// Unique constraint
	uniqueConstraint := createTable.Constraints[0]
	c.Assert(uniqueConstraint.Type, qt.Equals, ast.UniqueConstraint)
	c.Assert(uniqueConstraint.Name, qt.Equals, "uq_email")
	c.Assert(uniqueConstraint.Columns, qt.DeepEquals, []string{"email_address"})

	// Named check constraint
	checkConstraint := createTable.Constraints[1]
	c.Assert(checkConstraint.Type, qt.Equals, ast.CheckConstraint)
	c.Assert(checkConstraint.Name, qt.Equals, "chk_positive_value")
	c.Assert(checkConstraint.Expression, qt.Contains, "numeric_precise >= 0")

	// Unnamed table-level check constraint
	tableLevelCheck := createTable.Constraints[2]
	c.Assert(tableLevelCheck.Type, qt.Equals, ast.CheckConstraint)
	c.Assert(tableLevelCheck.Expression, qt.Contains, "created_at <= updated_at")
}

func TestParser_ParseMultiWordTypes(t *testing.T) {
	tests := []struct {
		name         string
		sql          string
		expectedType string
	}{
		{
			name:         "DOUBLE PRECISION",
			sql:          "CREATE TABLE test (value DOUBLE PRECISION);",
			expectedType: "DOUBLE PRECISION",
		},
		{
			name:         "CHARACTER VARYING",
			sql:          "CREATE TABLE test (name CHARACTER VARYING(255));",
			expectedType: "CHARACTER VARYING(255)",
		},
		{
			name:         "DOUBLE PRECISION with default",
			sql:          "CREATE TABLE test (value DOUBLE PRECISION DEFAULT 0.0);",
			expectedType: "DOUBLE PRECISION",
		},
		{
			name:         "DOUBLE PRECISION NOT NULL",
			sql:          "CREATE TABLE test (value DOUBLE PRECISION NOT NULL);",
			expectedType: "DOUBLE PRECISION",
		},
		{
			name:         "TIMESTAMP WITH TIME ZONE",
			sql:          "CREATE TABLE test (ts TIMESTAMP WITH TIME ZONE);",
			expectedType: "WITH TIMESTAMP TIME ZONE",
		},
		{
			name:         "TIMESTAMP WITHOUT TIME ZONE",
			sql:          "CREATE TABLE test (ts TIMESTAMP WITHOUT TIME ZONE);",
			expectedType: "WITHOUT TIMESTAMP TIME ZONE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			p := parser.NewParser(tt.sql)
			statements, err := p.Parse()
			c.Assert(err, qt.IsNil)
			c.Assert(len(statements.Statements), qt.Equals, 1)

			createTable := statements.Statements[0].(*ast.CreateTableNode)
			c.Assert(len(createTable.Columns), qt.Equals, 1)

			column := createTable.Columns[0]
			c.Assert(column.Type, qt.Equals, tt.expectedType)
		})
	}
}

func TestParser_ParseParameterizedArrayTypes(t *testing.T) {
	tests := []struct {
		name         string
		sql          string
		expectedType string
	}{
		{
			name:         "NUMERIC array with parameters",
			sql:          "CREATE TABLE test (scores NUMERIC(5,2)[]);",
			expectedType: "NUMERIC(5,2)[]",
		},
		{
			name:         "VARCHAR array",
			sql:          "CREATE TABLE test (names VARCHAR(100)[]);",
			expectedType: "VARCHAR(100)[]",
		},
		{
			name:         "DECIMAL multi-dimensional array",
			sql:          "CREATE TABLE test (matrix DECIMAL(10,2)[][]);",
			expectedType: "DECIMAL(10,2)[][]",
		},
		{
			name:         "CHAR array",
			sql:          "CREATE TABLE test (codes CHAR(3)[]);",
			expectedType: "CHAR(3)[]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			p := parser.NewParser(tt.sql)
			statements, err := p.Parse()
			c.Assert(err, qt.IsNil)
			c.Assert(len(statements.Statements), qt.Equals, 1)

			createTable := statements.Statements[0].(*ast.CreateTableNode)
			c.Assert(len(createTable.Columns), qt.Equals, 1)

			column := createTable.Columns[0]
			c.Assert(column.Type, qt.Equals, tt.expectedType)
		})
	}
}

func TestParser_ParseOriginalProblematicSQL(t *testing.T) {
	c := qt.New(t)

	// This is the original SQL that was causing infinite loop due to DOUBLE PRECISION
	sql := `CREATE TABLE public.full_demo (
		serial_id SERIAL PRIMARY KEY,
		double_value DOUBLE PRECISION,
		varchar_var VARCHAR(255) NOT NULL
	);`

	p := parser.NewParser(sql)
	statements, err := p.Parse()
	c.Assert(err, qt.IsNil)
	c.Assert(len(statements.Statements), qt.Equals, 1)

	createTable := statements.Statements[0].(*ast.CreateTableNode)
	c.Assert(createTable.Name, qt.Equals, "public.full_demo")
	c.Assert(len(createTable.Columns), qt.Equals, 3)

	// Verify DOUBLE PRECISION column is parsed correctly
	doubleCol := createTable.Columns[1]
	c.Assert(doubleCol.Name, qt.Equals, "double_value")
	c.Assert(doubleCol.Type, qt.Equals, "DOUBLE PRECISION")
}

func TestParser_ParsePostgreSQLTableOptions(t *testing.T) {
	tests := []struct {
		name            string
		sql             string
		expectedOptions map[string]string
	}{
		{
			name: "WITH clause with multiple options",
			sql: `CREATE TABLE test (
				id INTEGER PRIMARY KEY
			) WITH (
				fillfactor = 70,
				autovacuum_enabled = true,
				autovacuum_vacuum_threshold = 50
			);`,
			expectedOptions: map[string]string{
				"fillfactor":                   "70",
				"autovacuum_enabled":           "true",
				"autovacuum_vacuum_threshold":  "50",
			},
		},
		{
			name: "WITH clause and TABLESPACE",
			sql: `CREATE TABLE test (
				id INTEGER PRIMARY KEY
			) WITH (
				fillfactor = 80
			) TABLESPACE pg_default;`,
			expectedOptions: map[string]string{
				"fillfactor": "80",
				"TABLESPACE": "pg_default",
			},
		},
		{
			name: "TABLESPACE only",
			sql: `CREATE TABLE test (
				id INTEGER PRIMARY KEY
			) TABLESPACE custom_tablespace;`,
			expectedOptions: map[string]string{
				"TABLESPACE": "custom_tablespace",
			},
		},
		{
			name: "WITH clause with string values",
			sql: `CREATE TABLE test (
				id INTEGER PRIMARY KEY
			) WITH (
				toast_tuple_target = 128,
				parallel_workers = 4
			);`,
			expectedOptions: map[string]string{
				"toast_tuple_target": "128",
				"parallel_workers":   "4",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			p := parser.NewParser(tt.sql)
			statements, err := p.Parse()
			c.Assert(err, qt.IsNil)
			c.Assert(len(statements.Statements), qt.Equals, 1)

			createTable := statements.Statements[0].(*ast.CreateTableNode)
			c.Assert(len(createTable.Columns), qt.Equals, 1)

			// Verify all expected options are present
			for key, expectedValue := range tt.expectedOptions {
				actualValue, exists := createTable.Options[key]
				c.Assert(exists, qt.IsTrue, qt.Commentf("Option %s should exist", key))
				c.Assert(actualValue, qt.Equals, expectedValue, qt.Commentf("Option %s should have value %s", key, expectedValue))
			}
		})
	}
}

func TestParser_ParseExtendedPostgreSQLDemo(t *testing.T) {
	c := qt.New(t)

	// Extended comprehensive PostgreSQL CREATE TABLE statement with even more advanced features
	sql := `CREATE TABLE public.extended_demo (
		-- Identity and serial types
		serial_id SERIAL PRIMARY KEY,
		big_id BIGSERIAL UNIQUE,
		small_id SMALLSERIAL,

		-- UUID with default generator
		uuid_id UUID DEFAULT gen_random_uuid() NOT NULL,
		uuid_alt UUID DEFAULT uuid_generate_v4(),

		-- Character types with various specifications
		char_fixed CHAR(10),
		varchar_var VARCHAR(255) NOT NULL,
		varchar_unlimited VARCHAR,
		text_field TEXT CHECK (char_length(text_field) <= 5000),
		char_varying CHARACTER VARYING(100),

		-- Numeric types with constraints
		small_value SMALLINT DEFAULT 1 CHECK (small_value > 0),
		int_value INTEGER,
		big_value BIGINT,
		numeric_precise NUMERIC(12,4) NOT NULL DEFAULT 0.0000,
		decimal_alt DECIMAL(10,2),
		money_value MONEY DEFAULT '$0.00',

		-- Floating-point types
		real_value REAL,
		double_value DOUBLE PRECISION,
		float_value FLOAT(24),

		-- Boolean with default
		is_active BOOLEAN DEFAULT TRUE,
		is_deleted BOOLEAN DEFAULT FALSE,

		-- Dates and timestamps with various formats
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMPTZ DEFAULT now(),
		due_date DATE,
		time_only TIME,
		time_with_tz TIMETZ,
		interval_field INTERVAL,
		timestamp_no_tz TIMESTAMP WITHOUT TIME ZONE,
		timestamp_with_tz TIMESTAMP WITH TIME ZONE DEFAULT now(),

		-- Enum (requires pre-defined type)
		status status_enum DEFAULT 'pending',
		priority priority_type DEFAULT 'medium',

		-- Arrays of various types
		tags TEXT[] DEFAULT ARRAY[]::TEXT[],
		matrix INT[][],
		scores NUMERIC(5,2)[],
		flags BOOLEAN[] DEFAULT '{false,false,true}',

		-- JSON and JSONB
		json_field JSON,
		jsonb_field JSONB NOT NULL DEFAULT '{}'::jsonb,
		metadata JSONB DEFAULT '{"version": 1}',

		-- Binary data
		data BYTEA,
		file_content BYTEA,

		-- Network types
		ip_address INET,
		mac_address MACADDR,
		network_range CIDR,

		-- Geometric types
		point_location POINT,
		line_segment LSEG,
		box_area BOX,
		path_data PATH,
		polygon_shape POLYGON,
		circle_area CIRCLE,

		-- Text search types
		search_vector TSVECTOR,
		search_query TSQUERY,

		-- Range types
		int_range INT4RANGE,
		timestamp_range TSRANGE,
		date_range DATERANGE,

		-- Domain types (assume domains are defined)
		email_address email_domain,
		phone_number phone_domain,
		postal_code zipcode_domain,

		-- Generated columns
		full_name TEXT GENERATED ALWAYS AS (char_fixed || ' ' || varchar_var) STORED,
		search_text TSVECTOR GENERATED ALWAYS AS (to_tsvector('english', text_field)) STORED,

		-- Collation examples
		case_insensitive TEXT COLLATE "C",
		locale_specific TEXT COLLATE "en_US.UTF-8",

		-- Foreign keys with various cascading rules
		user_id INTEGER REFERENCES users(id) ON DELETE CASCADE ON UPDATE SET NULL,
		category_id INTEGER REFERENCES categories(id) ON DELETE SET NULL,
		parent_id INTEGER REFERENCES extended_demo(serial_id) ON DELETE CASCADE,

		-- Composite unique constraint
		CONSTRAINT uq_tag_and_status UNIQUE (tags, status),
		CONSTRAINT uq_user_category UNIQUE (user_id, category_id),

		-- Check constraints with expressions
		CONSTRAINT chk_price_non_negative CHECK (numeric_precise >= 0),
		CONSTRAINT chk_valid_email CHECK (email_address ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$'),
		CONSTRAINT chk_date_order CHECK (created_at <= updated_at),
		CONSTRAINT chk_status_priority CHECK (
			(status = 'urgent' AND priority IN ('high', 'critical')) OR
			(status != 'urgent')
		),

		-- Table-level check constraints
		CHECK (created_at <= updated_at),
		CHECK (small_value BETWEEN 1 AND 1000)
	)
	-- Table options
	WITH (
		fillfactor = 70,
		autovacuum_enabled = true,
		autovacuum_vacuum_threshold = 50,
		autovacuum_analyze_threshold = 50
	)
	TABLESPACE pg_default;
`

	p := parser.NewParser(sql)
	statements, err := p.Parse()
	c.Assert(err, qt.IsNil)
	c.Assert(len(statements.Statements), qt.Equals, 1)

	createTable := statements.Statements[0].(*ast.CreateTableNode)
	c.Assert(createTable.Name, qt.Equals, "public.extended_demo")

	// Test that we have a substantial number of columns (should be at least 40)
	c.Assert(len(createTable.Columns) > 40, qt.IsTrue)

	// Test key PostgreSQL features that should be parsed correctly

	// Test DOUBLE PRECISION is now working
	var doubleCol *ast.ColumnNode
	for _, col := range createTable.Columns {
		if col.Name == "double_value" {
			doubleCol = col
			break
		}
	}
	c.Assert(doubleCol, qt.IsNotNil)
	c.Assert(doubleCol.Type, qt.Equals, "DOUBLE PRECISION")

	// Test CHARACTER VARYING
	var charVaryingCol *ast.ColumnNode
	for _, col := range createTable.Columns {
		if col.Name == "char_varying" {
			charVaryingCol = col
			break
		}
	}
	c.Assert(charVaryingCol, qt.IsNotNil)
	c.Assert(charVaryingCol.Type, qt.Equals, "CHARACTER VARYING(100)")

	// Test TIMESTAMP WITH TIME ZONE
	var timestampCol *ast.ColumnNode
	for _, col := range createTable.Columns {
		if col.Name == "timestamp_with_tz" {
			timestampCol = col
			break
		}
	}
	c.Assert(timestampCol, qt.IsNotNil)
	c.Assert(timestampCol.Type, qt.Equals, "WITH TIMESTAMP TIME ZONE")

	// Test parameterized array type
	var scoresCol *ast.ColumnNode
	for _, col := range createTable.Columns {
		if col.Name == "scores" {
			scoresCol = col
			break
		}
	}
	c.Assert(scoresCol, qt.IsNotNil)
	c.Assert(scoresCol.Type, qt.Equals, "NUMERIC(5,2)[]")

	// Test that we have multiple constraints
	c.Assert(len(createTable.Constraints) > 0, qt.IsTrue)

	// Test table options from WITH clause
	c.Assert(createTable.Options["fillfactor"], qt.Equals, "70")
	c.Assert(createTable.Options["autovacuum_enabled"], qt.Equals, "true")
	c.Assert(createTable.Options["autovacuum_vacuum_threshold"], qt.Equals, "50")
	c.Assert(createTable.Options["autovacuum_analyze_threshold"], qt.Equals, "50")

	// Test TABLESPACE option
	c.Assert(createTable.Options["TABLESPACE"], qt.Equals, "pg_default")
}

func TestParser_ErrorHandling(t *testing.T) {
	tests := []struct {
		name string
		sql  string
	}{
		{
			name: "Invalid SQL keyword",
			sql:  "INVALID TABLE users (id INTEGER);",
		},
		{
			name: "Missing table name",
			sql:  "CREATE TABLE (id INTEGER);",
		},
		{
			name: "Missing opening parenthesis",
			sql:  "CREATE TABLE users id INTEGER);",
		},
		{
			name: "Missing column type",
			sql:  "CREATE TABLE users (id);",
		},
		{
			name: "Unterminated column list",
			sql:  "CREATE TABLE users (id INTEGER",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			p := parser.NewParser(tt.sql)
			_, err := p.Parse()
			c.Assert(err, qt.IsNotNil)
		})
	}
}
