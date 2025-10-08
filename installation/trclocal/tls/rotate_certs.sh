#!/bin/bash

# This script will rotate certs on a standard debian linux distro.
cd ~/tierceron/installation/trclocal/tls
./certs_gen.sh
cd ..
cp trc_seeds/certs/cert.pem /usr/local/vault/certs/serv_cert.pem
cp trc_seeds/certs/key.pem /usr/local/vault/certs/serv_key.pem 
cp trc_seeds/certs/cert.pem  /etc/ssl/certs/tierceron.test.pem
chmod 644 /etc/ssl/certs/tierceron.test.pem