package dbutil

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"strings"
	"testing"

	mssql "github.com/denisenkom/go-mssqldb"
	"github.com/go-sql-driver/mysql"
)

func TestConnectionErrorsNeverRenderCredentials(test *testing.T) {
	for _, testCase := range []struct {
		name  string
		cause error
		want  string
	}{
		{
			name: "login",
			cause: fmt.Errorf("mssql: login error: %w", mssql.Error{
				Number: 18456, Message: "Login failed for user 'database-account-example'.",
			}),
			want: "database login error (SQL Server error 18456)",
		},
		{
			name: "other SQL error",
			cause: mssql.Error{
				Number: 50000, Message: "database-account-example password=example-secret",
			},
			want: "database connection failed (SQL Server error 50000)",
		},
		{
			name: "MySQL login",
			cause: &mysql.MySQLError{
				Number: 1045, Message: "Access denied for user 'database-account-example' (password example-secret)",
			},
			want: "database login error (MySQL error 1045)",
		},
		{
			name:  "untyped driver login error",
			cause: errors.New("mssql: login error: Login failed for user 'database-account-example'."),
			want:  "database login error",
		},
		{
			name:  "connection URL",
			cause: errors.New("dial sqlserver://database-account-example:example-secret@example.invalid"),
			want:  "database connection failed",
		},
	} {
		test.Run(testCase.name, func(test *testing.T) {
			err := WrapConnectionError(testCase.cause)
			if err.Error() != testCase.want || !errors.Is(err, testCase.cause) {
				test.Fatal("safe error did not preserve its category and cause")
			}
			var output bytes.Buffer
			logger := log.New(&output, "", 0)
			logger.Printf("Test example failed with: %s", err.Error())
			logger.Printf("connection: %v / %+v", err, fmt.Errorf("test failed: %w", err))
			if strings.Contains(output.String(), "database-account-example") || strings.Contains(output.String(), "example-secret") {
				test.Fatal("connection error leaked credentials")
			}
		})
	}
}

func TestConnectionErrorPreservesSQLRetryClassification(test *testing.T) {
	original := mssql.Error{Number: 18456, Message: "Login failed for user 'database-account-example'."}
	err := WrapConnectionError(original)
	var sqlErr mssql.Error
	if !errors.As(err, &sqlErr) || sqlErr.Number != 18456 || !IsDatabaseLoginFailure(err) {
		test.Fatal("safe error lost the SQL Server retry classification")
	}
	if WrapConnectionError(nil) != nil {
		test.Fatal("wrapping a nil error must preserve nil")
	}
}

func TestDatabaseErrorForLoggingProtectsPooledQueryFailures(test *testing.T) {
	driverErr := mssql.Error{Number: 18456, Message: "Login failed for user 'database-account-example'."}
	for _, err := range []error{driverErr, &driverErr, fmt.Errorf("query failed: %w", driverErr), WrapConnectionError(driverErr)} {
		if message := DatabaseErrorForLogging(err); message != "database login error (SQL Server error 18456)" {
			test.Fatalf("driver error lost its safe error code: %q", message)
		}
	}
	mysqlErr := &mysql.MySQLError{Number: 1045, Message: "Access denied for user 'database-account-example'"}
	if DatabaseErrorForLogging(mysqlErr) != "database login error (MySQL error 1045)" {
		test.Fatal("MySQL login message was not replaced with a safe category")
	}
}

func TestUntypedLoginFailuresDoNotExposeUsernames(test *testing.T) {
	for _, message := range []string{
		"mssql: login error: Login failed for user 'database-account-example'.",
		"LOGIN FAILED FOR USER 'database-account-example'",
		"Access denied for user 'database-account-example'@'example.invalid'",
		"authentication failed for database-account-example",
	} {
		if DatabaseErrorForLogging(errors.New(message)) != "database login error" {
			test.Fatal("untyped login failure was not replaced with a safe category")
		}
	}
}
