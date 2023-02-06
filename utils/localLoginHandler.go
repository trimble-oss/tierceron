package utils

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	helperkv "github.com/trimble-oss/tierceron/vaulthelper/kv"

	//pb "github.com/trimble-oss/tierceron/webapi/rpc/apinator"

	tm "golang.org/x/crypto/ssh/terminal"
)

// LoginToLocal prompts the user to enter credentials from the terminal and resolves granular local environment
func LoginToLocal() (string, error) {
	var username, environment string
	var err error
	httpsClient, err := helperkv.CreateHTTPClient(false, coreopts.GetVaultHost(), "nonprod", false)
	if err != nil {
		return "", err
	}

	//client := pb.NewEnterpriseServiceBrokerProtobufClient(coreopts.GetVaultHost(), httpsClient)
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

		// resp, err := client.APILogin(context.Background(), &pb.LoginReq{
		// 	Environment: environment,
		// 	Username:    username,
		// 	Password:    string(password),
		// })
		for i := 0; i < len(password); i++ {
			password[i] = 0
		}
		httpsClient.CloseIdleConnections()

		if err != nil {
			return "", err
		}

		if false /* resp.Success */ {
			break
		} else {
			fmt.Printf("Could not login for user %s in %s\n", username, environment)
		}
	}
	return "local/" + environment + "/" + username, nil
}
