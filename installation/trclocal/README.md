# Introduction 
The installation folder for trclocal.  If you want to install a local vault, start here.

# Prerequisites
You *must* have all trc cmd line utilities installed as explained in GETTING_STARTED.md

# Build initial cloud infrastructure
Select installation directory.  This example will use /usr/local/vault

sudo mkdir /usr/local/vault
sudo mkdir /usr/local/vault/certs
sudo mkdir /usr/local/vault/plugins
sudo mkdir /usr/local/vault/vault_data

Download current version of vault: vault 1.3.6 (downloadable here: https://releases.hashicorp.com/vault/1.3.6/)
Unzip it and copy the vault executable to /usr/local/vault
sudo cp vault /usr/local/vault/vault

# Generating empty seed files
mkdir trc_seeds
trcx -env=dev -novault

# Edit seed files and provide certificates.
At this point you want to edit all seed variables in preparation for publish.

Fill in seed variables in super-secrets section of trc_seeds/dev/dev_seed.yml

# Create cert placeholder files
trcx -env=dev -certs -novault
After running trcx -certs, a certs folder will appear under trc_seeds with placeholder empty certificate files.
You'll want to replace these placeholder files with the real thing under ./trc_seeds/certs.
sudo cp trc_seeds/certs/* /usr/local/vault/certs/

# Generate vault properties configuration
trcconfig -env=dev -novault
sudo cp resources/vault_properties.hcl /usr/local/vault/
sudo cp trc_seeds/certs/* /usr/local/vault/certs/

# Start vault as a service.
sudo service vault start

Continue with the trcvault step to initialize vault and set up some tokens for utilization.

# Optional: later, after initializing trcvault, you can perform this step: Publish terraform seed data to vault.
trcinit -env=dev -token=$TRC_ROOT_TOKEN -addr=https://<vaulthost:vaultport> -indexed=TrcVault
trcinit -env=dev -certs -token=$TRC_ROOT_TOKEN -addr=https://<vaulthost:vaultport>

