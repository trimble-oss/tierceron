#!/bin/bash -e
#this script isn't being called on startup... why?
# Install packages
sudo apt-get update -y
sudo apt-get install -y curl unzip

# Download Vault into some temporary directory
curl -L "https://releases.hashicorp.com/vault/1.3.6/vault_1.3.6_linux_amd64.zip" > /tmp/vault.zip
# Unzip it
#/usr/src/app
#download aws cli
curl "https://s3.amazonaws.com/aws-cli/awscli-bundle.zip" -o "awscli-bundle.zip"
sudo unzip awscli-bundle.zip
sudo ./awscli-bundle/install -i /usr/local/aws -b /usr/local/bin/aws
#sudo aws configure --profile default

cd /tmp
sudo -- sh -c "echo '127.0.0.1 $(hostname)' >> /etc/hosts"
sudo unzip vault.zip
sudo mkdir -p /usr/src/app
sudo mv vault /usr/src/app/vault
sudo chmod 0755 /usr/src/app/vault
sudo chown root:root /usr/src/app/vault
sudo mkdir -p /etc/opt/vault/data/
#make directory etc/opt/vault
sudo mkdir -p /etc/opt/vault/certs/
#copy everything from /tmp
sudo mv /tmp/serv_*.pem /etc/opt/vault/certs/
#curl http://169.254.169.254/latest/meta-data/local-ipv4
privateip=$(hostname -I | cut -d' ' -f1); sed -i "s/127.0.0.1/$privateip/g" /tmp/vault_properties.hcl
#get pem files locally 
sudo mv /tmp/vault_properties.hcl /etc/opt/vault/vault_properties.hcl
sudo chown root:root /etc/opt/vault/vault_properties.hcl
#put API files up
sudo mkdir -p /etc/opt/trcAPI
#add build files
sudo mv /tmp/public /etc/opt/trcAPI
#make server log file
sudo touch /etc/opt/trcAPI/server.log
sudo chmod 0777 /etc/opt/trcAPI/server.log
sudo chown root:root /etc/opt/trcAPI/server.log
#add apiRouter executable
sudo unzip /tmp/apirouter.zip
sudo mv /tmp/apiRouter /etc/opt/trcAPI/apiRouter
sudo chmod 0755 /etc/opt/trcAPI/apiRouter
sudo chown root:root /etc/opt/trcAPI/apiRouter
#add policy files
sudo mv /tmp/policy_files /etc/opt/trcAPI
#add token files
sudo mv /tmp/token_files /etc/opt/trcAPI
#add template files
sudo mv /tmp/template_files /etc/opt/trcAPI
sudo mv /tmp/getArtifacts.sh /etc/opt/trcAPI
sudo chmod 0777 /etc/opt/trcAPI/getArtifacts.sh

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