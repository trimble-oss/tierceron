#!/bin/bash

echo "Enter vault host base url: "
read VAULT_ADDR

echo "Enter root token: "
read VAULT_TOKEN

echo "Enter environment: "
read VAULT_ENV

echo "Enter environment token with write permissions: "
read VAULT_ENV_TOKEN

vault secrets disable vaultdb/
vault plugin deregister trc-vault-plugin



if [ "$VAULT_ENV" = "prod" ] || [ "$VAULT_ENV" = "staging" ]
then
# Just writing to vault will trigger the carrier plugin...
# First we set Copied to false...
# This should also trigger the copy process...
# It should return sha256 of copied plugin on success.
SHA256BUNDLE=$(vault write vaultcarrier/$VAULT_ENV token=$VAULT_ENV_TOKEN plugin=trc-vault-plugin-prod)
# TODO: If this errors, then fail...
for keyval in $(ech $SHA256BUNDLE | grep -E '": [^\{]' | sed -e 's/: /=/' -e "s/\(\,\)$//"); do
    echo "export $keyval"
    eval export $keyval
done

# TODO: -- fall back to jq if the above doesn't work -- or delete this if it does work.
#for s in $(echo $SHA256BUNDLE | jq -r "to_entries|map(\"\(.key)=\(.value|tostring)\")|.[]" ); do
#    echo $s
#    export $s
#done

exit 0
vault plugin register \
          -command=trc-vault-plugin-prod \
          -sha256=$( cat target/trc-vault-plugin-prod.sha256 ) \
          -args=`backendUUID=4` \
          trc-vault-plugin-prod
vault secrets enable \
          -path=vaultdb \
          -plugin-name=trc-vault-plugin-prod \
          -description="Tierceron Vault Plugin Prod" \
          plugin
else
# Just writing to vault will trigger a copy...
# First we set Copied to false...
# This should also trigger the copy process...
# It should return sha256 of copied plugin on success.
SHA256BUNDLE=$(vault write vaultcarrier/$VAULT_ENV token=$VAULT_ENV_TOKEN plugin=trc-vault-plugin)
# TODO: If this errors, then fail...
for keyval in $(ech $SHA256BUNDLE | grep -E '": [^\{]' | sed -e 's/: /=/' -e "s/\(\,\)$//"); do
    echo "export $keyval"
    eval export $keyval
done

# TODO: -- fall back to jq if the above doesn't work -- or delete this if it does work.
#for s in $(echo $SHA256BUNDLE | jq -r "to_entries|map(\"\(.key)=\(.value|tostring)\")|.[]" ); do
#    echo $s
#    export $s
#done


exit 0

# TODO: If this errors, then fail...
vault plugin register \
          -command=trc-vault-plugin \
          -sha256=$( cat target/trc-vault-plugin.sha256 ) \
          -args=`backendUUID=4` \
          trc-vault-plugin
vault secrets enable \
          -path=vaultdb \
          -plugin-name=trc-vault-plugin \
          -description="Tierceron Vault Plugin" \
          plugin
fi

# Note: plugin should update deployed flag for itself.
vault write vaultdb/$VAULT_ENV token=$VAULT_ENV_TOKEN

