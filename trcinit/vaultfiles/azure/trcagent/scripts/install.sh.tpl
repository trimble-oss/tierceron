#!/bin/bash -e

# Install packages
sudo apt-get update -y
sudo apt-get install -y curl unzip
sudo apt-get install openjdk-11-jre-headless
sudo apt install docker.io
sudo apt install maven

# Download Vault into some temporary directory
curl -L "https://releases.hashicorp.com/vault/1.3.6/vault_1.3.6_linux_amd64.zip" > /tmp/vault.zip

cd /tmp
sudo -- sh -c "echo '127.0.0.1 $(hostname)' >> /etc/hosts"
sudo unzip vault.zip
sudo mkdir -p /usr/src/app
sudo mv vault /usr/src/app/vault
sudo chmod 0700 /usr/src/app/vault
sudo chown root:root /usr/src/app/vault
sudo mkdir -p /etc/opt/vault/data/
#make directory etc/opt/vault
sudo mkdir -p /etc/opt/vault/certs/
#copy everything from /tmp
sudo mv /tmp/serv_*.pem /etc/opt/vault/certs/
sudo mv /tmp/Digi*.crt.pem /etc/opt/vault/certs/
privateip=$(hostname -I | cut -d' ' -f1); sed -i "s/127.0.0.1/$privateip/g" /tmp/vault_properties.hcl
#get pem files locally 
sudo mv /tmp/vault_properties.hcl /etc/opt/vault/vault_properties.hcl
sudo chown root:root /etc/opt/vault/vault_properties.hcl

sudo mkuser azuredeploy
sudo mkdir /home/azuredeploy/bin
sudo chmod 1750 /home/azuredeploy/bin
sudo chown root:azuredeploy /home/azuredeploy/bin
# Agent is presently installed manually.  Probably best to keep it that way for now.

# Set up IP Table
# Add a rule to allow ssh connections
sudo iptables -A INPUT -p tcp --dport ${SSH_PORT} -s ${SCRIPT_CIDR_BLOCK} -j ACCEPT
# Block all other ip addresses
sudo iptables -A INPUT -p tcp -s 0.0.0.0/0 --dport ${SSH_PORT} -j DROP

# Add a rule to allow service connections
sudo iptables -A INPUT -p tcp --dport ${HOSTPORT} -s ${SCRIPT_CIDR_BLOCK} -j ACCEPT
# TODO: Uncomment when on azure fully?
#sudo iptables -A INPUT -p tcp --dport ${HOSTPORT} -s ${ONSITE_CIDR_BLOCK} -j ACCEPT
sudo iptables -A INPUT -p tcp --dport ${CONTROLLERA_PORT} -s ${SCRIPT_CIDR_BLOCK} -j ACCEPT
sudo iptables -A INPUT -p tcp --dport ${CONTROLLERB_PORT} -s ${SCRIPT_CIDR_BLOCK} -j ACCEPT
sudo iptables -A INPUT -p tcp --dport ${TRCDBA_PORT} -s ${SCRIPT_CIDR_BLOCK} -j ACCEPT
sudo iptables -A INPUT -p tcp --dport ${TRCDBB_PORT} -s ${SCRIPT_CIDR_BLOCK} -j ACCEPT
sudo iptables -A INPUT -p tcp --dport ${HOSTPORT} -s 127.0.0.1 -j ACCEPT
sudo iptables -A INPUT -p tcp --dport ${HOSTPORT} -s ${VAULTIP} -j ACCEPT

# Block all other ip addresses
sudo iptables -A INPUT -p tcp -s 0.0.0.0/0 --dport ${HOSTPORT} -j DROP
sudo iptables -A INPUT -p tcp -s 0.0.0.0/0 --dport ${CONTROLLERA_PORT} -j DROP
sudo iptables -A INPUT -p tcp -s 0.0.0.0/0 --dport ${CONTROLLERB_PORT} -j DROP
sudo iptables -A INPUT -p tcp -s 0.0.0.0/0 --dport ${TRCDBA_PORT} -j DROP
sudo iptables -A INPUT -p tcp -s 0.0.0.0/0 --dport ${TRCDBB_PORT} -j DROP

# To add other ip addresses after this process:
# iptables -I INPUT 2 -p tcp -s <ip_address> --dport <PORT> -j ACCEPT



# Setup the init script

# Using heredoc '<<'' in terraform doesn't
# allow for terraform variable substitution.
# it's neccessary to insert '<<' as a variable
# to add the host and host port to the script.
# ${write_service} serves this purpose.
cat ${write_service} EOF >/tmp/upstart
[Unit]
Description=Vault Service
After=systemd-user-sessions.service
[Service]

Type=simple
Environment="VAULT_API_ADDR=https://${HOST}:${HOSTPORT}"
Environment="GOMAXPROCS=$(nproc)"
ExecStart=/usr/src/app/vault server -config /etc/opt/vault/vault_properties.hcl
LimitMEMLOCK=infinity

#end script
EOF
sudo mv /tmp/upstart /lib/systemd/system/vault.service

# Start Vault
#sudo service vault start
