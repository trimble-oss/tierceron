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

# Edit seed files and provide certificates
At this point you want to edit all seed variables in preparation for publish.

Fill in seed variables in super-secrets section of trc_seeds/dev/dev_seed.yml, placing TODO for variables you don't care about.

Example secrets follow...
```
    adminUser: TODO -- only needed if you want mysql backing store.
    dbPassword: TODO -- only needed if you want mysql backing store.
    dbcert_name: TODO -- only needed if you want mysql backing store.
    dbname: TODO -- only needed if you want mysql backing store.
    hostport: "1234"
    vault_ip: 127.0.0.1
    vault_root_install: "/usr/local/vault"
```

# Create cert placeholder files
```
trcx -env=dev -certs -novault
```

After running trcx -certs, a certs folder will appear under trc_seeds with placeholder empty certificate files.
You'll want to replace these placeholder files with the real thing under ./trc_seeds/certs.

You can generate certs using the certs_gen.sh script located in [tls/certs_gen.sh](tls/certs_gen.sh).  Be sure to look at san.cnf before running the script to make
any desired changes to your self signed certificates.

```
sudo cp trc_seeds/certs/* /usr/local/vault/certs/
```

# Generate vault properties configuration
```
trcconfig -env=dev -novault
chmod 700 ./scripts/installconfigs.sh
sudo ./scripts/installconfigs.sh
chmod 700 ./scripts/install.sh
sudo ./scripts/install.sh
```

# Start vault as a service
```
sudo service vault start
```

Continue with the trcvault step to initialize vault and set up some tokens for utilization.

# Confirm vault running
You can enter https://<vaulthost:vaultport>/v1/sys/health in your browser to confirm vault is running.

# Make some tokens to operate on vault (other than root token)
```
trcinit -rotateTokens -namespace=base -addr=https://<vaulthost:vaultport> -token=<root token>
```

# Optional: later, after initializing trcvault, you can perform this step: Publish installation setup configuration seed data to vault
```
trcpub -env=dev -token=$VAULT_PUB_TOKEN -addr=https://<vaulthost:vaultport>
```

```
trcinit -env=dev -token=$VAULT_TOKEN -addr=https://<vaulthost:vaultport>
```

```
trcinit -env=dev -token=$VAULT_TOKEN -addr=https://<vaulthost:vaultport> -certs
```

# Test your configs are in vault
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

# TrcHelloWorld
```
cd trchelloworld
```
