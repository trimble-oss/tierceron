package dbutil

import (
	"context"
	"crypto/tls"
	"database/sql"
	"errors"
	"net"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcdb/opts/prod"
	"github.com/trimble-oss/tierceron/pkg/capauth"
	certutil "github.com/trimble-oss/tierceron/pkg/core/util/cert"
	trctls "github.com/trimble-oss/tierceron/pkg/tls"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
	"github.com/trimble-oss/tierceron/pkg/validator"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"

	"github.com/xo/dburl"
)

// OpenDirectConnection opens connection to a database using various sql urls used by Spectrum.
func OpenDirectConnection(driverConfig *config.DriverConfig,
	goMod *helperkv.Modifier,
	url string,
	username string,
	passwordFunc func() (string, error),
) (*sql.DB, error) {
	driver, server, port, dbname, certName, err := validator.ParseURL(driverConfig.CoreConfig, url)
	if err != nil {
		return nil, err
	}

	var conn *sql.DB
	var tlsConfig *tls.Config

	if goMod != nil {
		var clientCertBytes []byte
		var clientCertPath string
		if driver == "mysql" || driver == "mariadb" {
			clientCertPath = "Common/serviceclientcert.pem.mf.tmpl"
		} else if driver == "sqlserver" {
			clientCertPath = "Common/servicecert.crt.mf.tmpl"
		} else {
			return nil, errors.New("unsupported driver for TLS")
		}
		clientCertBytes, err = certutil.LoadCertComponent(driverConfig,
			goMod,
			clientCertPath)
		if err != nil {
			return nil, err
		}
		tlsConfig, err = trctls.GetTlsConfigFromCertBytes(clientCertBytes)
	} else {
		tlsConfig, err = trctls.GetTlsConfig(certName)
	}
	if err != nil {
		return nil, err
	}
	tlsConfig.ServerName = server

	tlsErr := mysql.RegisterTLSConfig("tiercerontls", tlsConfig)
	if tlsErr != nil {
		return nil, tlsErr
	}

	if driver == "sqlserver" {
		if prod.IsProd() {
			// Domain validation required in production environments.
			if err = capauth.ValidateVhostDomain(server); err != nil {
				return nil, err
			}
		} else if net.ParseIP(server) == nil {
			err = capauth.ValidateVhostDomain(server)
			if err != nil {
				return nil, err
			}
		}
	} else {
		if net.ParseIP(server) == nil {
			err = capauth.ValidateVhostInverse(server, "", true, false)
			if err != nil {
				return nil, err
			}
		}
	}

	password, passErr := passwordFunc()
	if passErr != nil {
		return nil, passErr
	}
	switch driver {
	case "mysql", "mariadb":
		if len(port) == 0 {
			// protocol+transport://user:pass@host/dbname?option1=a&option2=b
			conn, err = dburl.Open(driver + "://" + username + ":" + password + "@" + server + "/" + dbname + "?tls=tiercerontls&parseTime=true")
		} else {
			conn, err = dburl.Open(driver + "://" + username + ":" + password + "@" + server + ":" + port + "/" + dbname + "?tls=tiercerontls&parseTime=true")
		}
	case "sqlserver":
		if len(port) == 0 {
			port = "1433"
		}
		if net.ParseIP(server) == nil {
			conn, err = dburl.Open(driver + "://" + username + ":" + password + "@" + server + ":" + port + "/" + dbname + "?tls=tiercerontls")
		} else {
			conn, err = dburl.Open(driver + "://" + username + ":" + password + "@" + server + ":" + port + "/" + dbname + "?tls=skip-verify")
		}
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
