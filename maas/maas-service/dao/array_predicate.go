package dao

import (
	"gorm.io/gorm"
)

const sqliteDialect = "sqlite"

// ArrayContains builds a predicate matching rows whose array column holds value.
// On the master the column is a real array; on the SQLite cache it is the array
// literal as text, so there the value is matched between two commas.
//
// Elements PostgreSQL renders quoted - those holding a comma, a quote or a brace -
// are not matched on the cache.
func ArrayContains(cnn *gorm.DB, column string, value string) (string, []any) {
	dialector := cnn.Dialector
	if dialector.Name() == sqliteDialect {
		return "instr(',' || trim(" + column + ", '{}') || ',', ?) > 0", []any{"," + value + ","}
	}
	return "?=ANY(" + column + ")", []any{value}
}
