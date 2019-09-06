package kv

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
)

//CreateHTTPClient reads from several .pem files to get the necessary keys and certs to configure the http client and returns the client.
func CreateHTTPClient() (client *http.Client, err error) {
	cert, err := Asset("../../certs/cert_files/serv_cert.pem")
	//servCertPEM, err := ioutil.ReadFile(certPath)
	//servCertPEM := []byte(cert)
	if err != nil {
		return nil, err
	}
	// // create a pool of trusted certs
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(cert)

	// create another test server and use the certificate
	// configure a client to use trust those certificates
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: certPool},
		},
	}
	return httpClient, nil
}
