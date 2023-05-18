#!/bin/bash -e

# Install packages
sudo apt-get update -y
sudo apt-get install -y curl unzip
sudo apt-get install -y coreutils 
sudo apt-get install -y docker.io
sudo apt-get install -y openjdk-11-jre-headless
sudo apt-get install -y maven

curl https://packages.microsoft.com/keys/microsoft.asc | sudo apt-key add -
curl https://packages.microsoft.com/config/ubuntu/20.04/prod.list | sudo tee /etc/apt/sources.list.d/msprod.list

sudo apt-get update
# Because of licensing, this step has to be done manually. 
# sudo apt-get install -y mssql-tools unixodbc-dev

# Upgrade openssl to latest....
# https://www.openssl.org/news/openssl-1.1.1-notes.html
# sudo apt-get install make
# Insall openssl-1.1.1.t
# wget https://www.openssl.org/source/openssl-1.1.1t.tar.gz -O openssl-1.1.1t.tar.gz
# tar -zxvf openssl-1.1.1t.tar.gz
# cd openssl-1.1.1t
# ./config
# make
# sudo make install
# sudo ldconfig
# openssl version
#
# IMPORTANT!!!  If thinks go sideways after install check this directory: /usr/local/ssl/certs
# If it is empty....
# sudo rmdir /usr/local/ssl/certs
# sudo ln -s /etc/ssl/certs /usr/local/ssl/certs

# Download Vault into some temporary directory
curl -L "https://releases.hashicorp.com/vault/1.3.6/vault_1.3.6_linux_amd64.zip" > /tmp/vault.zip

cd /tmp
sudo -- sh -c "echo '127.0.0.1 $(hostname)' >> /etc/hosts"
sudo -- sh -c "echo '127.0.0.1 ${HOST}' >> /etc/hosts"
# TODO: fully qualified hostname....
sudo unzip vault.zip
sudo mkdir -p /usr/src/app
sudo mv vault /usr/src/app/vault
sudo chmod 0700 /usr/src/app/vault
sudo chown root:root /usr/src/app/vault
sudo setcap cap_ipc_lock=+ep /usr/src/app/vault
sudo mkdir -p /etc/opt/vault/data/
#make directory etc/opt/vault
sudo mkdir -p /etc/opt/vault/certs/
#copy everything from /tmp
sudo mv /tmp/serv_*.pem /etc/opt/vault/certs/
sudo mv /tmp/Digi*.crt.pem /etc/opt/vault/certs/
sudo chown -R root:root /etc/opt/vault/certs
sudo chmod 600 /etc/opt/vault/certs/*.pem

privateip=$(hostname -I | cut -d' ' -f1); sed -i "s/127.0.0.1/$privateip/g" /tmp/vault_properties.hcl
#get pem files locally 
sudo mv /tmp/vault_properties.hcl /etc/opt/vault/vault_properties.hcl
sudo chown root:root /etc/opt/vault/vault_properties.hcl
sudo chmod -R 0700 /etc/opt/vault/

# AGENT BLOCK: begin
# When building out TrcDb instances, remove this AGENT BLOCK section from .tpl....
sudo adduser --disabled-password --system --shell /bin/bash --group --home /home/azuredeploy azuredeploy
sudo mkdir -p /home/azuredeploy/bin
sudo mkdir -p /home/azuredeploy/myagent

# MANUAL STEP: IMPORTANT! Ensure azuredeploy is *not* a sudoer...
sudo chmod 1750 /home/azuredeploy/bin
sudo chown root:azuredeploy /home/azuredeploy/bin

curl -L "https://vstsagentpackage.azureedge.net/agent/2.217.2/vsts-agent-linux-x64-2.217.2.tar.gz" > /tmp/vsts-agent-linux-x64-2.217.2.tar.gz
sudo tar -C /home/azuredeploy/myagent -xzvf /tmp/vsts-agent-linux-x64-2.217.2.tar.gz

# Give ownership over to azuredeploy.
sudo chown -R azuredeploy:azuredeploy /home/azuredeploy/myagent

# echo 'export PATH="$PATH:/opt/mssql-tools/bin:/home/azuredeploy/bin"' >> ~/.bashrc
# echo $PATH > ~/myagent/.path

#Give docker permission to azuredeploy. 
sudo usermod -a -G docker azuredeploy
sudo chown root:azuredeploy /usr/bin/docker
sudo chmod 750 /usr/bin/docker

# MANUAL STEP: Agent is presently installed manually.  Probably best to keep it that way for now because of dependency on PAT.
# Get a PAT from https://viewpointvso.visualstudio.com/_usersSettings/tokens with Agent Pools (Read + Manage) permissions.
# 

# SSH and sudo/su ubuntu->root->azuredeploy
# Run following as azuredeploy:
# ./config.sh #Provide PAT from above.
#  When it asks for server url, use: https://dev.azure.com/<organization>
# ./run.sh
# As root, run: ./svc.sh install azuredeploy # important to install under restricted user azuredeploy

# As user azuredeploy, run the following:
# echo 'export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/opt/mssql-tools/bin:/home/azuredeploy/bin' >> ~/.bashrc
# . ~/.bashrc
# echo $PATH > ~/myagent/.path
# After install, run:
# ./svc.sh start as user root...
# If you ever have to re-register agent: 
#  ./svc.sh uninstall
#  ./config.sh remove --auth 'PAT' --token ''

# AGENT BLOCK: end


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


# Manual Mysql Database step...
# Connect with local mysql and Run sql command: `alter table vault modify vault_key varbinary(1024);`
# Update mysql variables to following:
# character_set_server	utf8
# collation_server	utf8_unicode_ci
# max_connections	512

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
