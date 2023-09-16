package trcshauth

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/trimble-oss/tierceron-hat/cap"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	eUtils "github.com/trimble-oss/tierceron/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

//go:embed tls/mashup.crt
var MashupCert embed.FS

//go:embed tls/mashup.key
var MashupKey embed.FS

var mashupCertPool *x509.CertPool

func init() {
	rand.Seed(time.Now().UnixNano())
	mashupCertBytes, err := MashupCert.ReadFile("tls/mashup.crt")
	if err != nil {
		fmt.Println("Cert read failure.")
		return
	}

	mashupBlock, _ := pem.Decode([]byte(mashupCertBytes))

	mashupClientCert, parseErr := x509.ParseCertificate(mashupBlock.Bytes)
	if parseErr != nil {
		fmt.Println("Cert parse read failure.")
		return
	}
	mashupCertPool = x509.NewCertPool()
	mashupCertPool.AddCert(mashupClientCert)
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func randomString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

type TrcShConfig struct {
	Env        string
	EnvContext string // Current env context...
	ConfigRole string
	PubRole    string
	KubeConfig string
}

const configDir = "/.tierceron/config.yml"
const envContextPrefix = "envContext: "

func GetSetEnvAddrContext(env string, envContext string, addrPort string) (string, string, string, error) {
	dirname, err := os.UserHomeDir()
	if err != nil {
		return "", "", "", err
	}

	//This will use env by default, if blank it will use context. If context is defined, it will replace context.
	if env == "" {
		file, err := os.ReadFile(dirname + configDir)
		if err != nil {
			fmt.Printf("Could not read the context file due to this %s error \n", err)
			return "", "", "", err
		}
		fileContent := string(file)
		if fileContent == "" {
			return "", "", "", errors.New("Could not read the context file")
		}
		if !strings.Contains(fileContent, envContextPrefix) && envContext != "" {
			var output string
			if !strings.HasSuffix(fileContent, "\n") {
				output = fileContent + "\n" + envContextPrefix + envContext + "\n"
			} else {
				output = fileContent + envContextPrefix + envContext + "\n"
			}

			if err = os.WriteFile(dirname+configDir, []byte(output), 0600); err != nil {
				return "", "", "", err
			}
			fmt.Println("Context flag has been written out.")
			env = envContext
		} else {
			re := regexp.MustCompile(`[-]?\d[\d,]*[\.]?[\d{2}]*`)
			result := re.FindAllString(fileContent[:strings.Index(fileContent, "\n")], -1)
			if len(result) == 1 {
				addrPort = result[0]
			} else {
				return "", "", "", errors.New("Couldn't find port.")
			}
			currentEnvContext := strings.TrimSpace(fileContent[strings.Index(fileContent, envContextPrefix)+len(envContextPrefix):])
			if envContext != "" {
				output := strings.Replace(fileContent, envContextPrefix+currentEnvContext, envContextPrefix+envContext, -1)
				if err = os.WriteFile(dirname+configDir, []byte(output), 0600); err != nil {
					return "", "", "", err
				}
				fmt.Println("Context flag has been written out.")
				env = envContext
			} else if env == "" {
				env = currentEnvContext
				envContext = currentEnvContext
			}
		}
	} else {
		envContext = env
		fmt.Println("Context flag will be ignored as env is defined.")
	}
	return env, envContext, addrPort, nil
}

// Helper function for obtaining auth components.
func TrcshAuth(config *eUtils.DriverConfig) (*TrcShConfig, error) {
	trcshConfig := &TrcShConfig{}
	var err error

	fmt.Println("Auth phase 1")
	if config.EnvRaw == "staging" || config.EnvRaw == "prod" || len(config.TrcShellRaw) > 0 {
		dir, err := os.UserHomeDir()
		if err != nil {
			fmt.Println("No homedir for current user")
			os.Exit(1)
		}
		fileBytes, err := os.ReadFile(dir + "/.kube/config")
		if err != nil {
			fmt.Println("No local kube config found...")
			os.Exit(1)
		}
		trcshConfig.KubeConfig = base64.StdEncoding.EncodeToString(fileBytes)

		if len(config.TrcShellRaw) > 0 {
			return trcshConfig, nil
		}
	} else {
		trcshConfig.KubeConfig, err = PenseQuery("kubeconfig")
	}

	if err != nil {
		return trcshConfig, err
	}
	memprotectopts.MemProtect(nil, &trcshConfig.KubeConfig)

	fmt.Println("Auth phase 2")
	addr, vAddressErr := PenseQuery("vaddress")
	if vAddressErr != nil {
		var addrPort string
		var env, envContext string

		fmt.Println(vAddressErr)
		//Env should come from command line - not context here. but addr port is needed.
		trcshConfig.Env, trcshConfig.EnvContext, addrPort, err = GetSetEnvAddrContext(env, envContext, addrPort)
		if err != nil {
			fmt.Println(err)
			return trcshConfig, err
		}
		addr = "https://127.0.0.1:" + addrPort

		config.Env = env
		config.EnvRaw = env
	}

	config.VaultAddress = addr
	memprotectopts.MemProtect(nil, &config.VaultAddress)

	fmt.Println("Auth phase 3")
	trcshConfig.ConfigRole, err = PenseQuery("configrole")
	if err != nil {
		return trcshConfig, err
	}
	memprotectopts.MemProtect(nil, &trcshConfig.ConfigRole)

	fmt.Println("Auth phase 4")
	trcshConfig.PubRole, err = PenseQuery("pubrole")
	if err != nil {
		return trcshConfig, err
	}
	memprotectopts.MemProtect(nil, &trcshConfig.PubRole)
	fmt.Println("Auth complete.")

	return trcshConfig, err
}

func PenseQuery(pense string) (string, error) {
	penseCode := randomString(7 + rand.Intn(7))
	penseArray := sha256.Sum256([]byte(penseCode))
	penseSum := hex.EncodeToString(penseArray[:])

	capWriteErr := cap.TapWriter(penseSum)
	if capWriteErr != nil {
		fmt.Println("Code 54 failure...")
		// 2023-06-30T01:29:21.7020686Z read unix @->/tmp/trccarrier/trcsnap.sock: read: connection reset by peer
		os.Exit(-1) // restarting carrier will rebuild necessary resources...
		return "", errors.Join(errors.New("Tap writer error"), capWriteErr)
	}

	conn, err := grpc.Dial("127.0.0.1:12384", grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{ServerName: "", RootCAs: mashupCertPool, InsecureSkipVerify: true})))
	if err != nil {
		return "", err
	}
	defer conn.Close()
	c := cap.NewCapClient(conn)

	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	r, penseErr := c.Pense(ctx, &cap.PenseRequest{Pense: penseCode, PenseIndex: pense})
	if penseErr != nil {
		return "", errors.Join(errors.New("Pense error"), penseErr)
	}

	return r.GetPense(), nil
}
