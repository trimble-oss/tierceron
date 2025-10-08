# Introduction 
You have found the installation folder for trcdb hive plugin templates and secrets.

# Trcdb default table initialization

```
trcinit -env=dev -token=$VAULT_TOKEN  -addr=$VAULT_ADDR -restricted=SpiralDatabase

```

# Create initial flow.  Do this for each environment initialize default argossocii flow
Spin up a local flow controller using the following:
```
trcctl edit -env=dev -token=$VAULT_TOKEN -addr=$VAULT_ADDR
```

The editor will start up the flow controlle but not allow editing of plugin data until You
complete the next step.  Connect using a mariadb compatible sql client, and execute the following 
statements to get the default ArgosSocii flow running.  (If you set up the controller correctly 
earlier, you should find access credentials in vault under 
super-secrets/<env>/Restricted/VaultDatabase/config.  You'll need a full access vault token to read 
from this area)


```
INSERT IGNORE INTO TierceronFlow(flowName) VALUES ("ArgosSocii");
update TierceronFlow set state=1 where flowName='ArgosSocii'
```


# Examples data for ArgosSocii
Proiectum:
    Fabrica Navis
        Servitium:
            Constructio
            Navigatio Technica
            Auxilium Medicum
            Cultura
    Questus Aureae Velleris
        Servitium:
            Navigatio
            Praesidium
            Informatio
            Communicationis
