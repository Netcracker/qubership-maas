package dao

import (
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
)

var DatabaseIsReadonlyError = errors.New("database is in read-only mode")
var DatabaseIsNotActiveError = errors.New("database is not in 'active' mode")

func (d BaseDaoImpl) translateDRModeError(err error) error {
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) {
		switch pgError.Code {
		case "25006": // READ ONLY SQL TRANSACTION
			if !d.drMode.IsActive() {
				return fmt.Errorf("sql operations failed on database due of DR mode set to: %s: error: %s: %w", d.drMode.String(), err.Error(), DatabaseIsNotActiveError)
			}
			return fmt.Errorf("sql operation failed (DR mode: %s): %s: %w", d.drMode.String(), err.Error(), DatabaseIsReadonlyError)
		default:
			return err
		}
	}

	return err
}
