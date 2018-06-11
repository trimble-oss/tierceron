#!/bin/bash -e
#this script isn't being called on startup... why?
# Install packages
sudo apt-get update -y
sudo apt-get install -y curl unzip

# Download Vault into some temporary directory
curl -L "https://releases.hashicorp.com/vault/0.10.1/vault_0.10.1_linux_amd64.zip" > /tmp/vault.zip

# Unzip it
#/usr/src/app
cd /tmp
sudo -- sh -c "echo '127.0.0.1 $(hostname)' >> /etc/hosts"
sudo unzip vault.zip
sudo mkdir -p /usr/src/app
sudo mv vault /usr/src/app/vault
sudo chmod 0755 /usr/src/app/vault
sudo chown root:root /usr/src/app/vault
#make directory etc/opt/vault
sudo mkdir -p /etc/opt/vault/certs/
#copy everything from /tmp
sudo mv /tmp/serv_*.pem /etc/opt/vault/certs/
#curl http://169.254.169.254/latest/meta-data/local-ipv4
privateip=$(hostname -I | cut -d' ' -f1); sed -i "s/127.0.0.1/$privateip/g" /tmp/vault_properties.hcl
#get pem files locally 
sudo mv /tmp/vault_properties.hcl /etc/opt/vault/vault_properties.hcl
sudo chown root:root /etc/opt/vault/vault_properties.hcl

# Setup the init script
cat <<EOF >/tmp/upstart
description "Vault server"

start on runlevel [2345]
stop on runlevel [!2345]

respawn

script
  if [ -f "/etc/service/vault" ]; then
    . /etc/service/vault
  fi

  # Make sure to use all our CPUs, because Vault can block a scheduler thread
  export GOMAXPROCS=`nproc`

  exec /usr/src/app/vault server \
    -config="/etc/opt/vault/vault_properties.hcl" \
    >>/var/log/vault.log 2>&1
end script
EOF
sudo mv /tmp/upstart /etc/init/vault.conf

# Start Vault
sudo start vault
