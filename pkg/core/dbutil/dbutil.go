package dbutil

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"errors"
	"net"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/trimble-oss/tierceron-core/v2/buildopts/kernelopts"
	"github.com/trimble-oss/tierceron-core/v2/core/coreconfig/cache"
	"github.com/trimble-oss/tierceron-core/v2/prod"
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

	if goMod != nil && (kernelopts.BuildOptions.IsKernel() || !prod.IsProd()) {
		var clientCertBytes *[]byte
		var clientCertPath string
		switch driver {
		case "mysql", "mariadb":
			if prod.IsProd() {
				// TODO: If prod combines to a common domain, we can get rid of this.
				clientCertPath = "Common/db_cert.pem.mf.tmpl"
			} else {
				clientCertPath = "Common/serviceclientcert.pem.mf.tmpl"
			}
		case "sqlserver":
			clientCertPath = "Common/servicecert.crt.mf.tmpl"
		default:
			return nil, errors.New("unsupported driver for TLS")
		}
		goMod.Reset()

		coreCopy := *driverConfig.CoreConfig
		driverConfigCopy := *driverConfig
		driverConfigCopy.CoreConfig = &coreCopy
		clientCertBytes, err = certutil.AddToCache(clientCertPath,
			&driverConfigCopy,
			goMod)
		if err != nil || clientCertBytes == nil {
			if err == nil {
				err = errors.New("clientCertBytes is nil")
			}
			return nil, err
		}
		driverConfig.CoreConfig.CertCache = driverConfigCopy.CoreConfig.CertCache
		rootCertPool := x509.NewCertPool()
		if len(*clientCertBytes) == 0 {
			return nil, errors.New("client certificate bytes are empty")
		}
		if ok := rootCertPool.AppendCertsFromPEM(*clientCertBytes); !ok {
			return nil, errors.New("couldn't append certs to root")
		}

		tlsConfig = &tls.Config{
			RootCAs:                     rootCertPool,
			MinVersion:                  tls.VersionTLS12,
			DynamicRecordSizingDisabled: true,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			},
		}
	} else {
		cacheKey := certName
		if cacheKey == "" {
			cacheKey = trctls.ServCert
		}
		var certBytes []byte
		if driverConfig.CoreConfig.CertCache != nil {
			if cv, ok := driverConfig.CoreConfig.CertCache.Get(cacheKey); ok && cv != nil && cv.CertBytes != nil {
				certBytes = *cv.CertBytes
			}
		}
		if certBytes == nil {
			certBytes, err = trctls.ReadServerCert(certName)
			if err == nil && driverConfig.CoreConfig.CertCache != nil {
				driverConfig.CoreConfig.CertCache.Set(cacheKey, &cache.CertValue{CertBytes: &certBytes})
			}
		}
		if err == nil {
			rootCertPool := x509.NewCertPool()
			if len(certBytes) == 0 {
				return nil, errors.New("client certificate bytes are empty")
			}
			if ok := rootCertPool.AppendCertsFromPEM(certBytes); !ok {
				return nil, errors.New("couldn't append certs to root")
			}

			tlsConfig = &tls.Config{
				RootCAs:                     rootCertPool,
				MinVersion:                  tls.VersionTLS12,
				DynamicRecordSizingDisabled: true,
				CipherSuites: []uint16{
					tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				},
			}
		}
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
