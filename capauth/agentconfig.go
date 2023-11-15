package capauth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"strings"
	"time"

	"github.com/trimble-oss/tierceron-hat/cap"
	"github.com/trimble-oss/tierceron-hat/cap/tap"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	eUtils "github.com/trimble-oss/tierceron/utils"
	helperkv "github.com/trimble-oss/tierceron/vaulthelper/kv"
	"google.golang.org/grpc"
)

var TrcCtlComplete string = "trcctlcomplete"

type AgentConfigs struct {
	HandshakeHostPort *string
	FeatherHostPort   *string
	HandshakeCode     *string
	DeployRoleID      *string
	EncryptPass       *string
	EncryptSalt       *string
	Deployments       *string
	Env               *string
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

var gTrcHatSecretsPort string = ""

func randomString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func ValidateVhost(host string) error {
	protocolHost := "https://" + host
	for _, endpoint := range coreopts.GetSupportedEndpoints() {
		if strings.HasPrefix(endpoint, protocolHost) {
			return nil
		}
	}
	return errors.New("Bad host: " + host)
}

func (agentconfig *AgentConfigs) PenseFeatherQuery(pense string) (*string, error) {
	penseCode := randomString(7 + rand.Intn(7))
	penseArray := sha256.Sum256([]byte(penseCode))
	penseSum := hex.EncodeToString(penseArray[:])

	_, featherErr := cap.FeatherWriter(*agentconfig.EncryptPass,
		*agentconfig.EncryptSalt,
		*agentconfig.HandshakeHostPort,
		*agentconfig.HandshakeCode,
		penseSum)
	if featherErr != nil {
		return nil, featherErr
	}

	creds, credErr := GetTransportCredentials()

	if credErr != nil {
		return nil, credErr
	}

	conn, err := grpc.Dial(*agentconfig.FeatherHostPort, grpc.WithTransportCredentials(creds))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := cap.NewCapClient(conn)

	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	r, err := c.Pense(ctx, &cap.PenseRequest{Pense: penseCode, PenseIndex: pense})
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	var penseProtect *string
	rPense := r.GetPense()
	penseProtect = &rPense
	memprotectopts.MemProtect(nil, penseProtect)

	return penseProtect, nil
}

func (agentconfig *AgentConfigs) LoadConfigs(address string, agentToken string, deployments string, env string) (*TrcShConfig, error) {
	var trcshConfig *TrcShConfig

	mod, modErr := helperkv.NewModifier(false, agentToken, address, env, nil, true, nil)
	if modErr != nil {
		fmt.Println("trcsh Failed to bootstrap")
		os.Exit(-1)
	}
	mod.Direct = true
	mod.Env = env

	data, readErr := mod.ReadData("super-secrets/Restricted/TrcshAgent/config")
	if readErr != nil {
		return nil, readErr
	} else {
		trcHatEncryptPass := data["trcHatEncryptPass"].(string)
		memprotectopts.MemProtect(nil, &trcHatEncryptPass)
		trcHatEncryptSalt := data["trcHatEncryptSalt"].(string)
		memprotectopts.MemProtect(nil, &trcHatEncryptSalt)
		trcHatEnv := data["trcHatEnv"].(string)
		memprotectopts.MemProtect(nil, &trcHatEnv)
		trcHatHandshakeCode := data["trcHatHandshakeCode"].(string)
		memprotectopts.MemProtect(nil, &trcHatHandshakeCode)
		trcHatHandshakePort := data["trcHatHandshakePort"].(string)
		memprotectopts.MemProtect(nil, &trcHatHandshakePort)
		trcHatHost := data["trcHatHost"].(string)
		memprotectopts.MemProtect(nil, &trcHatHost)
		trcHatSecretsPort := data["trcHatSecretsPort"].(string)
		memprotectopts.MemProtect(nil, &trcHatSecretsPort)
		trcHandshakeHostPort := trcHatHost + ":" + trcHatHandshakePort
		memprotectopts.MemProtect(nil, &trcHandshakeHostPort)
		trcFeatherHostPort := trcHatHost + ":" + trcHatSecretsPort
		memprotectopts.MemProtect(nil, &trcFeatherHostPort)

		agentconfig.HandshakeHostPort = &trcHandshakeHostPort
		agentconfig.FeatherHostPort = &trcFeatherHostPort
		agentconfig.HandshakeCode = &trcHatHandshakeCode
		agentconfig.EncryptPass = &trcHatEncryptPass
		agentconfig.EncryptSalt = &trcHatEncryptSalt
		agentconfig.Deployments = &deployments
		agentconfig.Env = &trcHatEnv

		trcshConfig = &TrcShConfig{Env: trcHatEnv,
			EnvContext: trcHatEnv,
		}
		trcShConfigRole, penseError := agentconfig.PenseFeatherQuery("configrole")
		if penseError != nil {
			return nil, penseError
		}
		memprotectopts.MemProtect(nil, trcShConfigRole)
		trcshConfig.ConfigRole = trcShConfigRole
	}

	return trcshConfig, nil
}

func PenseQuery(config *eUtils.DriverConfig, pense string) (*string, error) {
	penseCode := randomString(7 + rand.Intn(7))
	penseArray := sha256.Sum256([]byte(penseCode))
	penseSum := hex.EncodeToString(penseArray[:])

	penseEye, capWriteErr := tap.TapWriter(penseSum)

	if trcHtSp, trcHSPOk := penseEye["trcHatSecretsPort"]; trcHSPOk {
		if gTrcHatSecretsPort != trcHtSp {
			gTrcHatSecretsPort = trcHtSp
		}
	}

	if capWriteErr != nil || gTrcHatSecretsPort == "" {
		fmt.Println("Code 54 failure...")
		// 2023-06-30T01:29:21.7020686Z read unix @->/tmp/trccarrier/trcsnap.sock: read: connection reset by peer
		//		os.Exit(-1) // restarting carrier will rebuild necessary resources...
		return new(string), errors.Join(errors.New("Tap writer error"), capWriteErr)
	}

	localIP, err := LocalIp(config.EnvRaw)
	if err != nil {
		return nil, err
	}
	addrs, hostErr := net.LookupAddr(localIP)
	if hostErr != nil {
		return nil, hostErr
	}
	localHost := ""
	if len(addrs) > 0 {
		localHost = strings.TrimRight(addrs[0], ".")
		if validErr := ValidateVhost(localHost); validErr != nil {
			return nil, validErr
		}
	} else {
		return nil, errors.New("Invalid host")
	}

	// TODO: add domain if it's missing because that might actually happen...  Pull the domain from
	// vaddress in config..  that should always be the same...

	creds, err := GetTransportCredentials()
	if err != nil {
		return nil, err
	}

	conn, err := grpc.Dial(fmt.Sprintf("%s:%s", localHost, gTrcHatSecretsPort), grpc.WithTransportCredentials(creds))
	if err != nil {
		return new(string), err
	}
	defer conn.Close()
	c := cap.NewCapClient(conn)

	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	r, penseErr := c.Pense(ctx, &cap.PenseRequest{Pense: penseCode, PenseIndex: pense})
	if penseErr != nil {
		return new(string), errors.Join(errors.New("pense error"), penseErr)
	}
	var penseProtect *string
	rPense := r.GetPense()
	penseProtect = &rPense
	memprotectopts.MemProtect(nil, penseProtect)

	return penseProtect, nil
}
