These are special executables require Hashicorp Vault in order to function.

Carrier Plugin
Works in tandem with the secure trcshell to perform secure deployments to Azure.

TrcDb Plugin
Creates a running database with direct access to vault.  Configurations for the
database are provided by templates found under:
trcdb/trc_templates/TrcVault/Database
trcdb/trc_templates/TrcVault/VaultDatabase
trcdb/trc_templates/TrcVault/Identity
trcdb/trc_templates/Common

Additional tables are defined using templates found in trcdb/trc_templates
cd trcdb
mkdir trc_seeds
trcx -env=dev

Edit configurations under trc_seeds/dev/dev_seed.yml

trcinit -env=dev -token=$WRITE_TOKEN -addr=$VAULT_ADDR

# Deployment
Scripts are located in:
plugins/deploy
