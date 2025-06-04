# Introduction 
You have found the installation folder for trcdb hive plugin templates and secrets.

# Trcdb default table initialization

```
trcinit -env=dev -token=$VAULT_TOKEN  -addr=$VAULT_ADDR -restricted=SpiralDatabase

```

# Create initial flow
INSERT IGNORE INTO TierceronFlow(flowName) VALUES ("ArgosSocii");
update TierceronFlow set state=1 where flowName='ArgosSocii'


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
