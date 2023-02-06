package validator

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"time"

	//mysql and mssql go libraries
	_ "github.com/denisenkom/go-mssqldb"
	_ "github.com/go-sql-driver/mysql"

	eUtils "github.com/trimble-oss/tierceron/utils"
)

//need mssql for spectrum

// Heartbeat validates the database connection
func Heartbeat(config *eUtils.DriverConfig, url string, username string, password string) (bool, error) {
	//extract driver, server, port and dbname with regex
	driver, server, port, dbname := ParseURL(config, url)
	var err error
	var conn *sql.DB
	if driver == "mysql" {
		if len(port) == 0 {
			conn, err = sql.Open(driver, (username + ":" + password + "@tcp(" + server + ")/" + dbname + "?tls=skip-verify"))
		} else {
			conn, err = sql.Open(driver, (username + ":" + password + "@tcp(" + server + ":" + port + ")/" + dbname + "?tls=skip-verify"))
		}
	} else if driver == "sqlserver" {
		if len(port) == 0 {
			port = "1433"
		}
		conn, err = sql.Open(driver, ("server=" + server + ";user id=" + username + ";password=" + password + ";port=" + port + ";database=" + dbname + ";encrypt=true;TrustServerCertificate=true"))
	}
	if err != nil {
		return false, err
	}
	if conn != nil {
		defer conn.Close()
	}

	// Open doesn't open a connection. Validate DSN data:
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err = conn.PingContext(ctx); err != nil {
		if conn != nil {
			defer conn.Close()
		}
		return false, err
	}
	return true, nil
}
func ParseURL(config *eUtils.DriverConfig, url string) (string, string, string, string) {
	//only works with jdbc:mysql or jdbc:sqlserver.
	regex := regexp.MustCompile(`(?i)(?:jdbc:(mysql|sqlserver|mariadb))://([\w\-\.]+)(?::(\d{0,5}))?(?:/|.*;DatabaseName=)(\w+).*`)
	m := regex.FindStringSubmatch(url)
	if m == nil {
		eUtils.LogErrorObject(config, errors.New("incorrect URL format"), false)
	}
	return m[1], m[2], m[3], m[4]
}
