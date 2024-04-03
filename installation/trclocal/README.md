# Introduction 
The installation folder for trclocal.  If you want to install a local vault, start here.

# Prerequisites
You *must* have all trc cmd line utilities installed as explained in GETTING_STARTED.md

# Build initial cloud infrastructure
Select installation directory.  This example will use /usr/local/vault

```
sudo mkdir /usr/local/vault
sudo mkdir /usr/local/vault/certs
sudo mkdir /usr/local/vault/plugins
sudo mkdir /usr/local/vault/vault_data
```

Download current version of vault: vault 1.3.6 (downloadable here: https://releases.hashicorp.com/vault/1.3.6/)

Unzip it and copy the vault executable to /usr/local/vault

```
curl -L "https://releases.hashicorp.com/vault/1.3.6/vault_1.3.6_linux_amd64.zip" > /tmp/vault.zip
cd /tmp
sudo unzip vault.zip
sudo mkdir -p /usr/local/vault
sudo mv vault /usr/local/vault/vault
sudo chmod 0700 /usr/local/vault/vault
sudo chown root:root /usr/local/vault/vault
sudo setcap cap_ipc_lock=+ep /usr/local/vault/vault
```

# Generating empty seed files
```
mkdir trc_seeds
trcx -env=dev -novault
```

# Edit seed files and provide certificates.
At this point you want to edit all seed variables in preparation for publish.

Fill in seed variables in super-secrets section of trc_seeds/dev/dev_seed.yml

# Create cert placeholder files
```
trcx -env=dev -certs -novault
```

After running trcx -certs, a certs folder will appear under trc_seeds with placeholder empty certificate files.
You'll want to replace these placeholder files with the real thing under ./trc_seeds/certs.
```
sudo cp trc_seeds/certs/* /usr/local/vault/certs/
```

# Generate vault properties configuration
```
trcconfig -env=dev -novault
sudo cp resources/vault_properties.hcl /usr/local/vault/
sudo cp trc_seeds/certs/* /usr/local/vault/certs/
```

# Start vault as a service.
```
sudo service vault start
```

Continue with the trcvault step to initialize vault and set up some tokens for utilization.

# Rebooting vault (requires unseal)
You'll need to run the following command once for each unseal key you set up...

```
VAULT_ADDR=https://<vaulthost:vaultport> /usr/local/vault/vault operator unseal
```

Note, for local development installs where you may be using a self signed certificate, you can use the --tls-skip-verify

# Confirm vault running
You can enter https://<vaulthost:vaultport>/v1/sys/health in your browser to confirm vault is running.

# Make some tokens to operate on vault (other than root token)
```
trcinit -rotateTokens -namespace=base -addr=https://<vaulthost:vaultport> -token=<root token>
```

# Optional: later, after initializing trcvault, you can perform this step: Publish terraform seed data to vault.
```
trcpub -env=dev -token=$VAULT_PUB_TOKEN -addr=https://<vaulthost:vaultport>
```

```
trcinit -env=dev -token=$TRC_ROOT_TOKEN -addr=https://<vaulthost:vaultport>
```

```
trcinit -env=dev -token=$TRC_ROOT_TOKEN -addr=https://<vaulthost:vaultport> -certs
```

# Test your configs are in vault.
```
trcconfig -env=dev -token=$VAULT_CONFIG_TOKEN -addr=https://<vaulthost:vaultport> -insecure 
```

# Clean up locally stored secrets (Recommended but not required):
```
rm -r trc_seeds/dev
rm -r trc_seeds/certs
rm -r resources
rm -r scripts
rm *.log
```

# Initialze simple secrets to vault
```
cd trchelloworld
mkdir trc_seeds
trcx -env=dev -novault
```

# Change some secrets 
```
vim trc_seeds/dev/dev_seed.yml
```

```
trcinit -env=dev -token=$VAULT_TOKEN -addr=$VAULT_ADDR
```

# Clean up...
```
rm -r trc_seeds/dev
```
