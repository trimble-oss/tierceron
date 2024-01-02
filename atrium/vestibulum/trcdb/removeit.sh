#!/bin/bash

cd ../../../Vault.Hashicorp

vault secrets disable vaultdb/

vault plugin deregister trc-vault-plugin
rm vault/data/core/plugin-catalog/secret/_trc-vault-plugin
rm plugins/*
