package executor

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"

	"github.com/denisvmedia/inventario/ptah/core/goschema"
	"github.com/denisvmedia/inventario/ptah/core/sqlutil"
	"github.com/denisvmedia/inventario/ptah/schema/parser"
)
