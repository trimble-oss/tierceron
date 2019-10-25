package kv

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"time"
)

//CreateHTTPClient reads from several .pem files to get the necessary keys and certs to configure the http client and returns the client.
func CreateHTTPClient(env string) (client *http.Client, err error) {
	// // create a pool of trusted certs
	certPath := "../../certs/cert_files/dcidevpublic.pem"
	if env == "prod" || env == "staging" {
		certPath = "../../certs/cert_files/dcipublic.pem"
	}

	cert, err := Asset(certPath)
	//servCertPEM, err := ioutil.ReadFile(certPath)
	//servCertPEM := []byte(cert)
	if err != nil {
		return nil, err
	}
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(cert)

	// create another test server and use the certificate
	// configure a client to use trust those certificates
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: certPool},
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
				DualStack: true,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			DisableKeepAlives:     false,
			MaxConnsPerHost:       10,
		},
	}
	return httpClient, nil
}
