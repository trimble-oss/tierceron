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

//CreateHTTPClient reads from several .pem files to get the necessary keys and certs to configure the http client and returns the client.
func CreateHTTPClient(insecure bool, address string, env string, scan bool) (client *http.Client, err error) {
	// // create a pool of trusted certs
	certPath := "../../certs/cert_files/dcidevpublic.pem"
	if strings.HasPrefix(env, "prod") || strings.HasPrefix(env, "staging") {
		certPath = "../../certs/cert_files/dcipublic.pem"
	}

	cert, err := Asset(certPath)
	//servCertPEM, err := ioutil.ReadFile(certPath)
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
		u, err := url.Parse(address)
		if err != nil {
			return nil, err
		}
		host, _, _ := net.SplitHostPort(u.Host)
		ips, err := net.LookupIP(host)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			if ip.String() == "127.0.0.1" {
				tlsConfig = &tls.Config{RootCAs: certPool, InsecureSkipVerify: true}
				break
			}
		}
	}

	dialTimeout := 30 * time.Second
	tlsHandshakeTimeout := 10 * time.Second

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
