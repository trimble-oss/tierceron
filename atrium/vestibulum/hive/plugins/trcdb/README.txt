# Introduction 
You have found the installation folder for trcdb hive plugin templates and secrets.

# Trcdb default table initialization

```
trcinit -env=dev -token=$VAULT_TOKEN  -addr=$VAULT_ADDR -restricted=SpiralDatabase

```

# Create initial flow
INSERT IGNORE INTO TierceronFlow(flowName) VALUES ("DataFlowStatistics");
update TierceronFlow set state=1 where flowName='DataFlowStatistics'


# Adding a Table
This is more complicated than I'd like at the moment.  Add templates 
to atrium/buildopts/flowopts,testopts/flowopts,testopts.go and buildopts/buildoptsfunc.go 
Wire in the flow handler code in flowopts.ProcessFlowController (TODO/Chewbacca)

# Examples:
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
