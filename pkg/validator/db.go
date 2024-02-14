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

	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
)

//need mssql for spectrum

// Heartbeat validates the database connection
func Heartbeat(config *eUtils.DriverConfig, url string, username string, password string) (bool, error) {
	//extract driver, server, port and dbname with regex
	driver, server, port, dbname, _, err := ParseURL(config, url)
	if err != nil {
		return false, err
	}
	var conn *sql.DB
	switch driver {
	case "mysql":
		if len(port) == 0 {
			conn, err = sql.Open(driver, (username + ":" + password + "@tcp(" + server + ")/" + dbname + "?tls=skip-verify"))
		} else {
			conn, err = sql.Open(driver, (username + ":" + password + "@tcp(" + server + ":" + port + ")/" + dbname + "?tls=skip-verify"))
		}
	case "sqlserver":
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
func ParseURL(config *eUtils.DriverConfig, url string) (string, string, string, string, string, error) {
	//only works with jdbc:mysql or jdbc:sqlserver.
	regex := regexp.MustCompile(`(?i)(?:jdbc:(mysql|sqlserver|mariadb))://([\w\-\.]+)(?::(\d{0,5}))?(?:/|.*;DatabaseName=)(\w+)(.*certName=([\w\.]+)|.*).*`)
	m := regex.FindStringSubmatch(url)
	if m == nil {
		err := errors.New("incorrect URL format")
		eUtils.LogErrorObject(config, err, false)
		return "", "", "", "", "", err
	}
	certName := ""
	if len(m) >= 7 {
		certName = m[6]
	}
	return m[1], m[2], m[3], m[4], certName, nil
}
