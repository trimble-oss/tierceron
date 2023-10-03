package kv

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func IsUrlIp(address string) (bool, error) {
	if strings.HasPrefix(address, "https://127.0.0.1") {
		return true, nil
	}
	u, err := url.Parse(address)
	if err != nil {
		return false, err
	}
	host, _, _ := net.SplitHostPort(u.Host)
	ipHost := net.ParseIP(host)
	if ipHost.To4() != nil {
		return true, nil
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return false, err
	}
	for _, ip := range ips {
		if ip.String() == "127.0.0.1" {
			return true, nil
		}
	}
	return false, nil
}

// CreateHTTPClient reads from several .pem files to get the necessary keys and certs to configure the http client and returns the client.
func CreateHTTPClient(insecure bool, address string, env string, scan bool) (client *http.Client, err error) {
	return CreateHTTPClientAllowNonLocal(insecure, address, env, scan, false)
}

// CreateHTTPClient reads from several .pem files to get the necessary keys and certs to configure the http client and returns the client.
func CreateHTTPClientAllowNonLocal(insecure bool, address string, env string, scan bool, allowNonLocal bool) (client *http.Client, err error) {
	// // create a pool of trusted certs
	certPath := "../../certs/cert_files/dcidevpublic.pem"
	if strings.HasPrefix(env, "prod") || strings.HasPrefix(env, "staging") {
		certPath = "../../certs/cert_files/dcipublic.pem"
	}

	cert, err := Asset(certPath)
	//servCertPEM, err := os.ReadFile(certPath)
	//servCertPEM := []byte(cert)
	if err != nil {
		return nil, err
	}
	certPool, _ := x509.SystemCertPool()
	if certPool == nil {
		certPool = x509.NewCertPool()
	}

	certPool.AppendCertsFromPEM(cert)

	var tlsConfig = &tls.Config{RootCAs: certPool}
	if insecure {
		if isLocal, lookupErr := IsUrlIp(address); isLocal {
			if lookupErr != nil {
				return nil, lookupErr
			}
			tlsConfig = &tls.Config{RootCAs: certPool, InsecureSkipVerify: true}
		} else {
			if lookupErr != nil {
				return nil, lookupErr
			}
			if allowNonLocal {
				tlsConfig = &tls.Config{RootCAs: certPool, InsecureSkipVerify: true}
			}
		}
	}

	dialTimeout := 30 * time.Second
	tlsHandshakeTimeout := 30 * time.Second

	if scan {
		dialTimeout = 50 * time.Millisecond
		tlsHandshakeTimeout = 50 * time.Millisecond
	}

	// create another test server and use the certificate
	// configure a client to use trust those certificates
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
			DialContext: (&net.Dialer{
				Timeout:   dialTimeout,
				KeepAlive: 30 * time.Second,
				DualStack: true,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   tlsHandshakeTimeout,
			ExpectContinueTimeout: 1 * time.Second,
			DisableKeepAlives:     false,
			MaxConnsPerHost:       10,
		},
	}
	return httpClient, nil
}
