package util

import (
	"database/sql"

	"tierceron/validator"

	"github.com/xo/dburl"

	eUtils "tierceron/utils"
)

// OpenDirectConnection opens connection to a database using various sql urls used by Spectrum.
func OpenDirectConnection(config *eUtils.DriverConfig, url string, username string, password string) (*sql.DB, error) {
	driver, server, port, dbname := validator.ParseURL(config, url)

	var conn *sql.DB
	var err error

	if driver == "mysql" || driver == "mariadb" {
		if len(port) == 0 {
			// protocol+transport://user:pass@host/dbname?option1=a&option2=b
			conn, err = dburl.Open(driver + "://" + username + ":" + password + "@" + server + "/" + dbname + "?tls=skip-verify&parseTime=true")
		} else {
			conn, err = dburl.Open(driver + "://" + username + ":" + password + "@" + server + ":" + port + "/" + dbname + "?tls=skip-verify&parseTime=true")
		}
	} else if driver == "sqlserver" {
		if len(port) == 0 {
			port = "1433"
		}
		conn, err = dburl.Open(driver + "://" + username + ":" + password + "@" + server + ":" + port + "/" + dbname + "?tls=skip-verify")
	}
	if err != nil {
		defer conn.Close()
		return nil, err
	}

	// Open doesn't open a connection. Validate DSN data:
	err = conn.Ping()
	if err != nil {
		defer conn.Close()
		return nil, err
	}
	return conn, nil
}
