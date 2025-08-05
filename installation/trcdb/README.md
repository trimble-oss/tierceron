# Introduction 
You have found the installation folder for trcdb templates and secrets.

# Prerequisites
This assumes the existence of a vault with tokens.  You'll need a root or unrestricted token to initialize data from here on out.  If you're a security purist, you'll already have deleted the root token at this point and will just operate with the unrestricted dev token for the steps below.

# Generating empty seed files
```
trcpub -env=dev -token=$VAULT_TOKEN -addr=https://<vaulthost:vaultport>
```

```
trcx -env=dev -novault
```

```
trcx -env=dev -certs -novault
```

# Edit seed files and provide certificates.
At this point you want to edit all seed variables in preparation for publish.
After running trcx -certs, a certs folder will appear under trc_seeds with placeholder empty certificate files.
You'll want to replace these placeholder files with the real thing.

# Publish initial trcdb default seed data
```
trcinit -env=dev -token=$VAULT_TOKEN -addr=https://<vaulthost:vaultport>
```

```
trcinit -env=dev -certs -token=$VAULT_TOKEN -addr=https://<vaulthost:vaultport>
```

# TrcDb installation
From the root of the tierceron project, run the following commands.

```
cd atrium
make devplugintrcdb
cd ../installation/trcshhive/trcsh-curator/deploy
./deploy.sh (for trc-vault-plugin)
```

# TrcDb refresh Vault Database access restrictions
Access to trcdb is ip controlled via cidr block using the seed k/v cidrblock.

```
trcx -env=dev -token=$VAULT_TOKEN -restricted=VaultDatabase -serviceFilter=config -indexFilter=config
```

```
trcinit -env=dev -token=$VAULT_TOKEN  -addr=$VAULT_ADDR -restricted=VaultDatabase

```

# Spiral DB refresh access restrictions
Access to spiral db is optional since it only provides optional data flow statistics.
```
trcx -env=dev -token=$VAULT_TOKEN -restricted=SpiralDatabase -serviceFilter=config -indexFilter=config
```

```
trcinit -env=dev -token=$VAULT_TOKEN  -addr=$VAULT_ADDR -restricted=SpiralDatabase
```

# Trcdb default table initialization
Generate the default table and flow: DataFlowStatistics

```
trcinit -indexed=TrcDb
INSERT IGNORE INTO TierceronFlow(flowName) VALUES ("DataFlowStatistics");
```

In order to extract data flow statistic data to seed files, run the following:
```
trcx -env=dev -token=$VAULT_TOKEN -addr=$VAULT_ADDR -dynamicPath=PublicIndex/TrcDb/argosId/%s/DataFlowStatistics/DataFlowGroup/%s/dataFlowName/%s/%s
```

# Adding additional table flows
Adding other table and business flows is as simple as creating and
pubbing the templates (table schema), initializing the default values for these templates (via trcx/trcinit), and inserting a row into TierceronFlow.  You'll also need to provide some default table processing code.  You can find an example with ArgosSocii in the default/example trcdb plugin.  Tables can be managed in this way from any directory containing properly named project/service templates where project is the database name and service is the name of the table you want to add.
