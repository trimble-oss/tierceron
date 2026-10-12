#!/bin/bash

# This script will rotate certs on a standard debian linux distro.
cd ~/tierceron/installation/trclocal/tls
./certs_gen.sh
cd ..
install -m 0644 trc_seeds/certs/servicecert.crt /usr/local/vault/certs/serv_cert.pem
install -m 0600 trc_seeds/certs/servicekey.key /usr/local/vault/certs/serv_key.pem
cp trc_seeds/certs/servicecert.crt /etc/ssl/certs/tierceron.test.pem
chmod 644 /etc/ssl/certs/tierceron.test.pem