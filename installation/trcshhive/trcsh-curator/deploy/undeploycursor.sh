#!/bin/bash

vault plugin deregister trcsh-cursor-k
vault secrets disable trcsh-cursor-k
vault secrets list | grep trcsh-cursor-k

