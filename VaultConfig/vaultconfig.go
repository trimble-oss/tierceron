package main

import (
	"flag"
	"fmt"
	"strings"

	"bitbucket.org/dexterchaney/whoville/VaultConfig/utils"
	eUtils "bitbucket.org/dexterchaney/whoville/utils"
	"bitbucket.org/dexterchaney/whoville/vault-helper/kv"
	sys "bitbucket.org/dexterchaney/whoville/vault-helper/system"
)

func main() {
	tokenPtr := flag.String("token", "", "Vault access token")
	addrPtr := flag.String("addr", "http://127.0.0.1:8200", "API endpoint for the vault")
	startDirPtr := flag.String("templateDir", "vault_templates/ServiceTechAPI", "Template directory")
	endDirPtr := flag.String("endDir", "config_files/ServiceTechAPI", "Directory to put configured templates into")
	certPathPtr := flag.String("certPath", "certs/cert_files/serv_cert.pem", "Path to the server certificate")
	env := flag.String("env", "dev", "Environment to configure")
	secretMode := flag.Bool("secretMode", true, "Only override secret values in templates?")
	servicesWanted := flag.String("servicesWanted", "", "Services to pull template values for, in the form 'service1,service2' (defaults to all services)")
	secretIDPtr := flag.String("secretID", "", "Public app role ID")
	appRoleIDPtr := flag.String("appRoleID", "", "Secret app role ID")
	tokenNamePtr := flag.String("tokenName", "", "Token name used by this tdinator to access the vault")

	flag.Parse()

	if len(*tokenNamePtr) > 0 {
		if len(*appRoleIDPtr) == 0 || len(*secretIDPtr) == 0 {
			eUtils.CheckError(fmt.Errorf("Need both public and secret app role to retrieve token from vault"), true)
		}
		v, err := sys.NewVault(*addrPtr, *certPathPtr)
		eUtils.CheckError(err, true)

		master, err := v.AppRoleLogin(*appRoleIDPtr, *secretIDPtr)
		eUtils.CheckError(err, true)

		mod, err := kv.NewModifier(master, *addrPtr, *certPathPtr)
		eUtils.CheckError(err, true)
		mod.Env = "bamboo"

		*tokenPtr, err = mod.ReadValue("super-secrets/tokens", *tokenNamePtr)
		eUtils.CheckError(err, true)
	}

	services := []string{}
	if *servicesWanted != "" {
		services = strings.Split(*servicesWanted, ",")
	}

	for _, service := range services {
		service = strings.TrimSpace(service)
	}
	utils.ConfigFromVault(*tokenPtr, *addrPtr, *certPathPtr, *env, *secretMode, services, *startDirPtr, *endDirPtr)
}
