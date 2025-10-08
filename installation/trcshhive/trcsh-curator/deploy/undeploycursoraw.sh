#!/bin/bash

vault plugin deregister trcsh-cursor-aw
vault secrets disable trcsh-cursor-aw
vault secrets list | grep trcsh-cursor-aw

