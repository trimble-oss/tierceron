package dbutil

import (
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"errors"
	"net/url"
	"regexp"

	etlcore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcninja/core"

	_ "github.com/denisenkom/go-mssqldb"
	"github.com/go-sql-driver/mysql"
	_ "github.com/go-sql-driver/mysql"
	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	"github.com/xo/dburl"
)

func ParseURL(url string) (string, string, string, string, string, error) {
	// only works with jdbc:mysql or jdbc:sqlserver.
	regex := regexp.MustCompile(`(?i)(?:jdbc:(mysql|sqlserver|mariadb))://([\w\-\.]+)(?::(\d{0,5}))?(?:/|.*;DatabaseName=)(\w+)(.*certName=([\w\.]+)|.*).*`)
	m := regex.FindStringSubmatch(url)
	if m == nil {
		err := errors.New("incorrect URL format")
		etlcore.LogError(err.Error())
		// eUtils.LogErrorObject(config, err, false)
		return "", "", "", "", "", err
	}
	certName := ""
	if len(m) >= 7 {
		certName = m[6]
	}
	return m[1], m[2], m[3], m[4], certName, nil
}

// OpenDirectConnection opens connection to a database using various sql urls
func OpenDirectConnection(configContext *tccore.ConfigContext) (*sql.DB, error) {
	if configContext.Config == nil {
		return nil, errors.New("missing required config")
	}

	var sqlUrl, sqlUsername, sqlPassword string
	var ok bool
	if sqlUrl, ok = (*configContext.Config)["sqlUrl"].(string); !ok {
		return nil, errors.New("missing required sqlUrl")
	}
	if sqlUsername, ok = (*configContext.Config)["sqlUsername"].(string); !ok {
		return nil, errors.New("missing required sqlUsername")
	}
	if sqlPassword, ok = (*configContext.Config)["sqlPassword"].(string); !ok {
		return nil, errors.New("missing required sqlPassword")
	}
	sqlPassword = url.QueryEscape(sqlPassword)
	driver, server, port, dbname, _, err := ParseURL(sqlUrl)
	if err != nil {
		return nil, err
	}

	var conn *sql.DB
	var tlsConfig *tls.Config
	var clientCertBytes []byte
	if certBytes, ok := (*configContext.Config)[tccore.TRCSHHIVEK_CERT].([]byte); ok {
		clientCertBytes = certBytes
	} else {
		return nil, errors.New("missing required certificate for ninja direct connection")
	}

	rootCertPool := x509.NewCertPool()
	if ok := rootCertPool.AppendCertsFromPEM(clientCertBytes); !ok {
		return nil, errors.New("couldn't append certs to root")
	}

	tlsConfig = &tls.Config{
		RootCAs: rootCertPool,
	}
	tlsConfig.ServerName = server

	tlsErr := mysql.RegisterTLSConfig("tiercerontls", tlsConfig)
	if tlsErr != nil {
		return nil, tlsErr
	}

	if driver == "mysql" {
		if len(port) == 0 {
			// protocol+transport://user:pass@host/dbname?option1=a&option2=b
			conn, err = dburl.Open(driver + "://" + sqlUsername + ":" + sqlPassword + "@" + server + "/" + dbname + "?tls=skip-verify")
		} else {
			conn, err = dburl.Open(driver + "://" + sqlUsername + ":" + sqlPassword + "@" + server + ":" + port + "/" + dbname + "?tls=skip-verify")
		}
	} else if driver == "sqlserver" {
		if len(port) == 0 {
			port = "1433"
		}
		conn, err = dburl.Open(driver + "://" + sqlUsername + ":" + sqlPassword + "@" + server + ":" + port + "/" + dbname + "?tls=skip-verify")
	} else if driver == "mariadb" {
		if len(port) == 0 {
			// protocol+transport://user:pass@host/dbname?option1=a&option2=b
			conn, err = dburl.Open(driver + "://" + sqlUsername + ":" + sqlPassword + "@" + server + "/" + dbname + "?tls=skip-verify&parseTime=true")
		} else {
			conn, err = dburl.Open(driver + "://" + sqlUsername + ":" + sqlPassword + "@" + server + ":" + port + "/" + dbname + "?tls=skip-verify&parseTime=true")
		}
	}

	if err != nil {
		if conn != nil {
			defer conn.Close()
		}
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
