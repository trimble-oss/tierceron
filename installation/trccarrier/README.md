# Introduction 
You have found the installation folder for trc carrier.  This is a trusted vault
plugin utilized in the tierceron secure deployment services.  Carrier, working
in tandem with trcsh will interact with a docker registry and either virtual machines or a kubernetes cluster in order to securely deploy services.

# Prerequisites
This assumes the existence of a vault with tokens.  You also must have installed the build dependencies under [GETTING_STARTED.MD](../../GETTING_STARTED.MD#command-line-building-via-makefile). You'll need a root and unrestricted token install the carrier.

# Build carrier deployment scripts
trcconfig
chmod 700 deploy/*.sh
cp deploy/deploycarrier.sh ../../atrium/plugins/deploy/
cp deploy/refreshcarriertoken.sh ../../atrium/plugins/deploy/

# Building the carrier
cd ../../atrium
make certify devplugincarrier
cd plugins/deploy

# Deploy the carrier
sudo cp target/trc-vault-carrier-plugin /usr/local/vault/plugins
sudo setcap cap_ipc_lock=+ep /usr/local/vault/plugins/trc-vault-carrier-plugin

./deploycarrier.sh [Deploy Carrier](atrium/plugin/deploy)
./deploy.sh