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
	captiplib "github.com/trimble-oss/tierceron-hat/captip/captiplib"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcdb/opts/prod"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/buildopts/saltyopts"
	"github.com/trimble-oss/tierceron/pkg/tls"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AgentConfigs struct {
	*cap.FeatherContext
	AgentToken      *string
	FeatherHostPort *string
	DeployRoleID    *string
	Deployments     *string
	Env             *string
	Drone           *bool
}

type TrcshDriverConfig struct {
	DriverConfig eUtils.DriverConfig
	FeatherCtx   *cap.FeatherContext
	FeatherCtlCb func(*cap.FeatherContext, string) error
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

func ValidateVhost(host string, protocol string, skipPort bool, logger ...*log.Logger) error {
	return ValidateVhostInverse(host, protocol, false, skipPort, logger...)
}

func ValidateVhostDomain(host string) error {
	for _, domain := range coreopts.BuildOptions.GetSupportedDomains(prod.IsProd()) {
		if strings.HasSuffix(host, domain) {
			return nil
		}
	}
	return errors.New("Bad host: " + host)
}

func ValidateVhostInverse(host string, protocol string, inverse bool, skipPort bool, logger ...*log.Logger) error {
	if !strings.HasPrefix(host, protocol) || (len(protocol) > 0 && !strings.HasPrefix(protocol, "https")) {
		if len(logger) > 0 {
			logger[0].Printf("missing required protocol: %s", protocol)
		}
		return fmt.Errorf("missing required protocol: %s", protocol)
	}
	var ip string
	hostname := host
	hostname = host[len(protocol):]
	// Remove remaining invalid characters from host
	for {
		if strings.HasPrefix(hostname, ":") {
			hostname = hostname[strings.Index(hostname, ":")+1:]
		} else if strings.HasPrefix(hostname, "/") {
			hostname = hostname[strings.Index(hostname, "/")+1:]
		} else {
			break
		}
	}
	if strings.Contains(hostname, ":") {
		hostname = hostname[:strings.Index(hostname, ":")]
	}
	if strings.Contains(hostname, "/") {
		hostname = hostname[:strings.Index(hostname, "/")]
	}
	ips, err := net.LookupIP(hostname)
	if err != nil {
		if len(ips) == 0 && strings.Contains(hostname, ".test") {
			ip = "127.0.0.1"
		} else {
			fmt.Println("Error looking up host ip address, please confirm current tierceron vault host name and ip.")
			fmt.Println(err)
			if len(logger) > 0 {
				logger[0].Println("Error looking up host ip address, please confirm current tierceron vault host name and ip.")
				logger[0].Println(err)
			}
			return errors.New("Bad host: " + host)
		}
	}
	if len(ips) > 0 {
		ip = ips[0].String()
	}

	for _, endpoint := range coreopts.BuildOptions.GetSupportedEndpoints(prod.IsProd()) {
		if inverse {
			if endpoint[1] == "n/a" || endpoint[1] == ip {
				// format protocol if non-empty
				if len(protocol) > 0 && !strings.HasSuffix(protocol, "://") {
					if strings.Contains(protocol, ":") {
						protocol = protocol[:strings.Index(protocol, ":")]
					}
					protocol = protocol + "://"
				}
				if strings.Contains(fmt.Sprintf("%s%s", protocol, endpoint[0]), host) || (skipPort && strings.Contains(endpoint[0], hostname)) {
					return nil
				}
			} else {
				fmt.Printf("Invalid IP address of supported domain: %s \n", ip)
				fmt.Println("Please confirm current tierceron vault host name and ip.")
				if len(logger) > 0 {
					logger[0].Printf("Invalid IP address of supported domain: %s Please confirm current tierceron vault host name and ip.\n", ip)
				}
				return errors.New("Bad host: " + host)
			}
		} else {
			var protocolHost = host
			if !strings.HasPrefix(host, "https://") {
				protocolHost = fmt.Sprintf("https://%s", host)
			}
			var protocolEndpoint = endpoint[0]
			if !strings.HasPrefix(endpoint[0], "https://") {
				protocolEndpoint = fmt.Sprintf("https://%s", endpoint[0])
			}
			if strings.HasPrefix(protocolEndpoint, protocolHost) || (skipPort && strings.Contains(protocolEndpoint, hostname)) {
				if endpoint[1] == "n/a" || endpoint[1] == ip {
					return nil
				} else {
					fmt.Printf("Invalid IP address of supported domain: %s \n", ip)
					fmt.Println("Please confirm current tierceron vault host name and ip.")
					if len(logger) > 0 {
						logger[0].Printf("Invalid IP address of supported domain: %s \n", ip)
						logger[0].Println("Please confirm current tierceron vault host name and ip.")
					}
					return errors.New("Bad host: " + host)
				}
			}
		}
	}
	return errors.New("Bad host: " + host)
}

func (agentconfig *AgentConfigs) RetryingPenseFeatherQuery(pense string) (*string, error) {
	retry := 0
	for retry < 5 {
		result, err := agentconfig.PenseFeatherQuery(agentconfig.FeatherContext, pense)

		if err != nil || result == nil || *result == "...." {
			time.Sleep(time.Second)
			retry = retry + 1
		} else {
			return result, err
		}
	}
	return nil, errors.New("unavailable secrets")
}

func (agentconfig *AgentConfigs) PenseFeatherQuery(featherCtx *cap.FeatherContext, pense string) (*string, error) {
	penseCode := randomString(12 + rand.Intn(7))
	penseArray := sha256.Sum256([]byte(penseCode))
	penseSum := hex.EncodeToString(penseArray[:])
	penseSum = penseSum + saltyopts.BuildOptions.GetSaltyGuardian()

	creds, credErr := tls.GetTransportCredentials(agentconfig.Drone)

	if credErr != nil {
		return nil, credErr
	}

	err := ValidateVhost(*agentconfig.FeatherHostPort, "", true)
	if err != nil {
		return nil, err
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

	var r *cap.PenseReply
	retry := 0

	for {
		_, err := c.Pense(ctx, &cap.PenseRequest{Pense: "", PenseIndex: ""})
		if err != nil {
			st, ok := status.FromError(err)

			if ok && (retry < 5) && st.Code() == codes.Unavailable {
				retry = retry + 1
				continue
			} else {
				return nil, err
			}
		} else {
			break
		}
	}

	_, featherErr := cap.FeatherWriter(featherCtx, penseSum)
	if featherErr != nil {
		return nil, featherErr
	}

	for {
		r, err = c.Pense(ctx, &cap.PenseRequest{Pense: penseCode, PenseIndex: pense})
		if err != nil {
			st, ok := status.FromError(err)

			if ok && (retry < 5) && st.Code() == codes.Unavailable {
				retry = retry + 1
				continue
			} else {
				return nil, err
			}
		} else {
			break
		}
	}

	var penseProtect *string
	rPense := r.GetPense()
	penseProtect = &rPense
	memprotectopts.MemProtect(nil, penseProtect)

	return penseProtect, nil
}

func NewAgentConfig(address string,
	agentToken string,
	env string,
	acceptRemoteFunc func(*cap.FeatherContext, int, string) (bool, error),
	interruptedFunc func(*cap.FeatherContext) error,
	logger *log.Logger,
	drone ...*bool) (*AgentConfigs, *TrcShConfig, error) {
	if logger != nil {
		logger.Printf(".")
	} else {
		fmt.Printf(".")
	}

	mod, modErr := helperkv.NewModifier(false, agentToken, address, env, nil, true, nil)
	if modErr != nil {
		logger.Println("trcsh Failed to bootstrap")
		os.Exit(-1)
	}
	mod.Direct = true
	envParts := strings.Split(env, "-")
	mod.Env = envParts[0]

	if logger != nil {
		logger.Printf(".")
	} else {
		fmt.Printf(".")
	}
	data, readErr := mod.ReadData("super-secrets/Restricted/TrcshAgent/config")
	defer func(m *helperkv.Modifier, e string) {
		m.Env = e
	}(mod, env)

	if logger != nil {
		logger.Printf(".")
	} else {
		fmt.Printf(".")
	}

	if readErr != nil {
		return nil, nil, readErr
	} else {
		if data["trcHatEncryptPass"] == nil ||
			data["trcHatEncryptSalt"] == nil ||
			data["trcHatHandshakeCode"] == nil ||
			data["trcHatEnv"] == nil ||
			data["trcHatHost"] == nil {
			return nil, nil, errors.New("missing required secrets: possible missing secrets or lack of permissions for provided token")
		}
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
		var trcHatEnv string
		if strings.HasPrefix(env, data["trcHatEnv"].(string)) {
			trcHatEnv = env
		} else {
			trcHatEnv = data["trcHatEnv"].(string)
		}
		if logger != nil {
			logger.Printf(".")
		} else {
			fmt.Printf(".")
		}

		deployments := "bootstrap"
		isDrone := false
		if len(drone) > 0 {
			isDrone = *drone[0]
		}
		agentconfig := &AgentConfigs{
			captiplib.FeatherCtlInit(nil,
				trcHatHostLocal,
				&trcHatEncryptPass,
				&trcHatEncryptSalt,
				&hatHandshakeHostAddr,
				&trcHatHandshakeCode,
				&sessionIdentifier,
				&env,
				acceptRemoteFunc, interruptedFunc),
			&agentToken,
			&hatFeatherHostAddr,
			new(string),
			&deployments,
			&trcHatEnv,
			&isDrone,
		}

		trcshConfig := &TrcShConfig{Env: trcHatEnv,
			EnvContext: trcHatEnv,
		}

		var penseError error
		trcshConfig.ConfigRole, penseError = agentconfig.RetryingPenseFeatherQuery("configrole")
		if penseError != nil {
			return nil, nil, penseError
		}
		if logger != nil {
			logger.Printf(".")
		} else {
			fmt.Printf(".")
		}

		trcshConfig.VaultAddress, penseError = agentconfig.RetryingPenseFeatherQuery("caddress")
		if penseError != nil {
			return nil, nil, penseError
		}
		if logger != nil {
			logger.Printf(".")
		} else {
			fmt.Printf(".")
		}

		return agentconfig, trcshConfig, nil
	}
}

func PenseQuery(trcshDriverConfig *TrcshDriverConfig, pense string) (*string, error) {
	penseCode := randomString(12 + rand.Intn(7))
	penseArray := sha256.Sum256([]byte(penseCode))
	penseSum := hex.EncodeToString(penseArray[:])

	penseEye, capWriteErr := tap.TapWriter(penseSum)

	if trcHtSp, trcHSPOk := penseEye["trcHatSecretsPort"]; trcHSPOk {
		if gTrcHatSecretsPort != trcHtSp {
			gTrcHatSecretsPort = trcHtSp
		}
	}

	if capWriteErr != nil || gTrcHatSecretsPort == "" {
		fmt.Println("Code 54 failure...  Possible deploy components mismatch..")
		// 2023-06-30T01:29:21.7020686Z read unix @->/tmp/trccarrier/trcsnap.sock: read: connection reset by peer
		//		os.Exit(-1) // restarting carrier will rebuild necessary resources...
		return new(string), errors.New("tap writer error")
	}

	// TODO: add domain if it's missing because that might actually happen...  Pull the domain from
	// vaddress in config..  that should always be the same...

	creds, err := tls.GetTransportCredentials()
	if err != nil {
		return nil, err
	}
	dialOptions := grpc.WithTransportCredentials(creds)

	localHost, localHostErr := LocalAddr(trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis)
	if localHostErr != nil {
		return nil, localHostErr
	}
	conn, err := grpc.Dial(fmt.Sprintf("%s:%s", localHost, gTrcHatSecretsPort), dialOptions)
	if err != nil {
		return new(string), err
	}
	defer conn.Close()
	c := cap.NewCapClient(conn)

	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	localHostConfirm, localHostConfirmErr := LocalAddr(trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis)
	if localHostConfirmErr != nil {
		return nil, localHostConfirmErr
	}
	if localHost != localHostConfirm {
		return nil, errors.New("host selection flux - cannot continue")
	}

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
