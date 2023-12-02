package capauth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strings"
	"time"

	"github.com/trimble-oss/tierceron-hat/cap"
	"github.com/trimble-oss/tierceron-hat/cap/tap"
	captiplib "github.com/trimble-oss/tierceron-hat/captip/captiplib"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	eUtils "github.com/trimble-oss/tierceron/utils"
	helperkv "github.com/trimble-oss/tierceron/vaulthelper/kv"
	"google.golang.org/grpc"
)

type AgentConfigs struct {
	*cap.FeatherContext
	AgentToken      *string
	FeatherHostPort *string
	DeployRoleID    *string
	Deployments     *string
	Env             *string
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
	protocolHost := host
	if !strings.HasPrefix(host, "https://") {
		protocolHost = fmt.Sprintf("https://%s", host)
	}
	for _, endpoint := range coreopts.GetSupportedEndpoints() {
		if strings.HasPrefix(endpoint, protocolHost) {
			return nil
		}
	}
	return errors.New("Bad host: " + host)
}

func (agentconfig *AgentConfigs) PenseFeatherQuery(featherCtx *cap.FeatherContext, pense string) (*string, error) {
	penseCode := randomString(7 + rand.Intn(7))
	penseArray := sha256.Sum256([]byte(penseCode))
	penseSum := hex.EncodeToString(penseArray[:])

	_, featherErr := cap.FeatherWriter(featherCtx, penseSum)
	if featherErr != nil {
		return nil, featherErr
	}

	creds, credErr := GetTransportCredentials()

	if credErr != nil {
		return nil, credErr
	}

	conn, err := grpc.Dial(*agentconfig.FeatherHostPort, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	c := cap.NewCapClient(conn)

	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	r, err := c.Pense(ctx, &cap.PenseRequest{Pense: penseCode, PenseIndex: pense})
	if err != nil {
		return nil, err
	}
	var penseProtect *string
	rPense := r.GetPense()
	penseProtect = &rPense
	memprotectopts.MemProtect(nil, penseProtect)

	return penseProtect, nil
}

func NewAgentConfig(address string,
	agentToken string,
	deployments string,
	env string,
	acceptRemoteFunc func(*cap.FeatherContext, int, string) (bool, error),
	interruptedFunc func(*cap.FeatherContext) error) (*AgentConfigs, *TrcShConfig, error) {
	mod, modErr := helperkv.NewModifier(false, agentToken, address, env, nil, true, nil)
	if modErr != nil {
		fmt.Println("trcsh Failed to bootstrap")
		os.Exit(-1)
	}
	mod.Direct = true
	mod.Env = env

	data, readErr := mod.ReadData("super-secrets/Restricted/TrcshAgent/config")
	if readErr != nil {
		return nil, nil, readErr
	} else {
		trcHatHostLocal := new(string)
		trcHatEncryptPass := data["trcHatEncryptPass"].(string)
		memprotectopts.MemProtect(nil, &trcHatEncryptPass)
		trcHatEncryptSalt := data["trcHatEncryptSalt"].(string)
		memprotectopts.MemProtect(nil, &trcHatEncryptSalt)
		hatHandshakeHostAddr := fmt.Sprintf("%s:%s", data["trcHatHost"].(string), data["trcHatHandshakePort"].(string))
		memprotectopts.MemProtect(nil, &hatHandshakeHostAddr)
		trcHatHandshakeCode := data["trcHatHandshakeCode"].(string)
		memprotectopts.MemProtect(nil, &trcHatHandshakeCode)
		sessionIdentifier := "sessionIdDynamicFill"

		hatFeatherHostAddr := fmt.Sprintf("%s:%s", data["trcHatHost"].(string), data["trcHatSecretsPort"].(string))
		memprotectopts.MemProtect(nil, &hatFeatherHostAddr)
		trcHatEnv := data["trcHatEnv"].(string)

		agentconfig := &AgentConfigs{
			captiplib.FeatherCtlInit(nil,
				trcHatHostLocal,
				&trcHatEncryptPass,
				&trcHatEncryptSalt,
				&hatHandshakeHostAddr,
				&trcHatHandshakeCode,
				&sessionIdentifier, captiplib.AcceptRemote, nil),
			&agentToken,
			&hatFeatherHostAddr,
			new(string),
			&deployments,
			&trcHatEnv,
		}

		trcshConfig := &TrcShConfig{Env: trcHatEnv,
			EnvContext: trcHatEnv,
		}

		trcShConfigRole, penseError := agentconfig.PenseFeatherQuery(agentconfig.FeatherContext, "configrole")
		if penseError != nil {
			return nil, nil, penseError
		}
		memprotectopts.MemProtect(nil, trcShConfigRole)
		trcshConfig.ConfigRole = trcShConfigRole

		return agentconfig, trcshConfig, nil
	}
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
