package kv

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net/http"
)

//CreateHTTPClient reads from several .pem files to get the necessary keys and certs to configure the http client and returns the client.
func CreateHTTPClient() (client *http.Client, err error) {

	servCertPEM, err := ioutil.ReadFile("certs/cert_files/serv_cert.pem")
	if err != nil {
		return nil, err
	}

	servKeyPEM, err := ioutil.ReadFile("certs/cert_files/serv_key.pem")
	if err != nil {
		return nil, err
	}

	servTLSCert, err := tls.X509KeyPair(servCertPEM, servKeyPEM)
	if err != nil {
		return nil, err
	}
	// create a pool of trusted certs
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(servCertPEM)

	// create another test server and use the certificate
	// configure a client to use trust those certificates
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{Certificates: []tls.Certificate{servTLSCert}, RootCAs: certPool},
			//InsecureSkipVerify: true
		},
	}
	return httpClient, nil
}
