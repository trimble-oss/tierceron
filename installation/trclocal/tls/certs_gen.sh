#!/bin/bash

# This script generates self signed certificates.  
# They are not to be used in a production environment!
openssl req -new -nodes -newkey rsa:2048 -config san.cnf -reqexts v3_req -keyout cert.key -out cert.csr
openssl x509 -req -in cert.csr -extfile san.cnf -extensions v3_req -signkey cert.key -days 365 -out cert.crt

cp cert.key ../trc_seeds/certs/key.pem
cp cert.crt ../trc_seeds/certs/cert.pem
