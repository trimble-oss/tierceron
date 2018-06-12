package kv

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net/http"
)

//CreateHTTPClient reads from several .pem files to get the necessary keys and certs to configure the http client and returns the client.
func CreateHTTPClient(certPath string) (client *http.Client, err error) {

	servCertPEM, err := ioutil.ReadFile(certPath)
	if err != nil {
		return nil, err
	}

	// // create a pool of trusted certs
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(servCertPEM)

	// create another test server and use the certificate
	// configure a client to use trust those certificates
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: certPool},
		},
	}
	return httpClient, nil
}
