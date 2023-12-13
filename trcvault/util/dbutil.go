package util

import (
	"context"
	"database/sql"
	"time"

	"github.com/trimble-oss/tierceron/validator"

	"github.com/xo/dburl"

	eUtils "github.com/trimble-oss/tierceron/utils"
)

// OpenDirectConnection opens connection to a database using various sql urls used by Spectrum.
func OpenDirectConnection(config *eUtils.DriverConfig, url string, username string, password string) (*sql.DB, error) {
	driver, server, port, dbname, err := validator.ParseURL(config, url)

	if err != nil {
		return nil, err
	}

	var conn *sql.DB

	switch driver {
	case "mysql", "mariadb":
		if len(port) == 0 {
			// protocol+transport://user:pass@host/dbname?option1=a&option2=b
			conn, err = dburl.Open(driver + "://" + username + ":" + password + "@" + server + "/" + dbname + "?tls=skip-verify&parseTime=true")
		} else {
			conn, err = dburl.Open(driver + "://" + username + ":" + password + "@" + server + ":" + port + "/" + dbname + "?tls=skip-verify&parseTime=true")
		}
	case "sqlserver":
		if len(port) == 0 {
			port = "1433"
		}
		conn, err = dburl.Open(driver + "://" + username + ":" + password + "@" + server + ":" + port + "/" + dbname + "?tls=skip-verify")
	}

	if err != nil {
		if conn != nil {
			defer conn.Close()
		}
		return nil, err
	}

	// Open doesn't open a connection. Validate DSN data:
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err = conn.PingContext(ctx); err != nil {
		if conn != nil {
			defer conn.Close()
		}
		return nil, err
	}

	return conn, nil
}
