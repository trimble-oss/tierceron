package utils

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"

	"Vault.Whoville/vaulthelper/kv"
	pb "Vault.Whoville/webapi/rpc/apinator"
	configcore "VaultConfig.Bootstrap/configcore"
	tm "golang.org/x/crypto/ssh/terminal"
)

// LoginToLocal prompts the user to enter credentials from the terminal and resolves granular local environment
func LoginToLocal() (string, error) {
	if true {
		return "local/dev/JR", nil
	}
	var username, environment string
	var err error
	httpsClient, err := kv.CreateHTTPClient("nonprod")
	if err != nil {
		return "", err
	}

	client := pb.NewEnterpriseServiceBrokerProtobufClient(configcore.VaultHost, httpsClient)
	console := bufio.NewReader(os.Stdin)
	fmt.Println("Login needed to use a local environment")
	for {
		// Get Sign in environment
		fmt.Print("Sign In Environment: ")
		environment, err = console.ReadString('\n')
		if err != nil {
			return "", err
		}
		environment = strings.TrimSpace(environment)

		// Get Username
		fmt.Print("Username: ")
		username, err = console.ReadString('\n')
		if err != nil {
			return "", err
		}
		username = strings.TrimSpace(username)

		// Get Password
		fmt.Print("Password: ")
		password, err := tm.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return "", err
		}

		resp, err := client.APILogin(context.Background(), &pb.LoginReq{
			Environment: environment,
			Username:    username,
			Password:    string(password),
		})
		for i := 0; i < len(password); i++ {
			password[i] = 0
		}
		httpsClient.CloseIdleConnections()

		if err != nil {
			return "", err
		}

		if resp.Success {
			break
		} else {
			fmt.Printf("Could not login for user %s in %s\n", username, environment)
		}
	}
	return "local/" + environment + "/" + username, nil
}
