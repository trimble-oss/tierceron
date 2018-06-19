package main

import (
	"flag"
	"strings"

	"bitbucket.org/dexterchaney/whoville/VaultConfig/utils"
)

var environments = [...]string{
	"secret",
	"local",
	"dev",
	"QA"}

func main() {
	tokenPtr := flag.String("token", "", "Vault access token")
	addrPtr := flag.String("addr", "http://127.0.0.1:8200", "API endpoint for the vault")
	startDirPtr := flag.String("templateDir", "vault_templates/ST", "Template directory")
	endDirPtr := flag.String("endDir", "config_files/ST", "Directory to put configured templates into")
	certPathPtr := flag.String("certPath", "certs/cert_files/serv_cert.pem", "Path to the server certificate")
	env := flag.String("env", environments[0], "Environment to configure")
	secretMode := flag.Bool("secretMode", true, "Only override secret values in templates?")
	servicesWanted := flag.String("servicesWanted", "", "Services to pull template values for, in the form 'service1,service2' (defaults to all services)")
	flag.Parse()

	services := []string{}
	if *servicesWanted != "" {
		services = strings.Split(*servicesWanted, ",")
	}

	for _, service := range services {
		service = strings.TrimSpace(service)
	}
	utils.ConfigFromVault(*tokenPtr, *addrPtr, *certPathPtr, *env, *secretMode, services, *startDirPtr, *endDirPtr)
}
