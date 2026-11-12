package dbutil

import (
	"errors"
	"fmt"
	"strings"

	mssql "github.com/denisenkom/go-mssqldb"
	"github.com/go-sql-driver/mysql"
	"github.com/trimble-oss/tierceron-core/v2/core"
)

type connectionError struct {
	cause error
}

func WrapConnectionError(err error) error {
	if err == nil {
		return nil
	}
	return &connectionError{cause: err}
}

func (err *connectionError) Error() string {
	if number, ok := sqlServerErrorNumber(err.cause); ok {
		switch number {
		case 18456, 18452, 18470, 18487, 18488, 4060:
			return fmt.Sprintf("database login error (SQL Server error %d)", number)
		default:
			return fmt.Sprintf("database connection failed (SQL Server error %d)", number)
		}
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err.cause, &mysqlErr) {
		switch mysqlErr.Number {
		case 1044, 1045, 1698:
			return fmt.Sprintf("database login error (MySQL error %d)", mysqlErr.Number)
		default:
			return fmt.Sprintf("database connection failed (MySQL error %d)", mysqlErr.Number)
		}
	}
	if IsDatabaseLoginFailure(err.cause) {
		return "database login error"
	}
	return "database connection failed"
}

func (err *connectionError) Unwrap() error {
	return err.cause
}

func DatabaseErrorForLogging(err error) string {
	var safe *connectionError
	if errors.As(err, &safe) {
		return safe.Error()
	}
	if _, ok := sqlServerErrorNumber(err); ok {
		return (&connectionError{cause: err}).Error()
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return (&connectionError{cause: err}).Error()
	}
	if IsDatabaseLoginFailure(err) {
		return "database login error"
	}
	return core.SanitizeForLogging(err.Error())
}

func sqlServerErrorNumber(err error) (int32, bool) {
	var value mssql.Error
	if errors.As(err, &value) {
		return value.Number, true
	}
	var pointer *mssql.Error
	if errors.As(err, &pointer) && pointer != nil {
		return pointer.Number, true
	}
	return 0, false
}

func IsDatabaseLoginFailure(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "login error") ||
		strings.Contains(message, "login failed") ||
		strings.Contains(message, "access denied for user") ||
		strings.Contains(message, "authentication failed")
}
