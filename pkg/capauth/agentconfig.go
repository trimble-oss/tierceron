package capauth

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"strings"
	"time"

	prod "github.com/trimble-oss/tierceron-core/v2/prod"
	"github.com/trimble-oss/tierceron-hat/cap"
	"github.com/trimble-oss/tierceron-hat/cap/tap"
	captiplib "github.com/trimble-oss/tierceron-hat/captip/captiplib"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/cursoropts"
	"github.com/trimble-oss/tierceron/buildopts/kernelopts"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/buildopts/saltyopts"
	"github.com/trimble-oss/tierceron/pkg/core/cache"
	"github.com/trimble-oss/tierceron/pkg/tls"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
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
	DriverConfig *config.DriverConfig
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

// IsCertValidBySupportedDomains accepts a certificate
func IsCertValidBySupportedDomains(byteCert []byte,
	certValidationHelper func(cert *x509.Certificate, host string, selfSignedOk bool) (bool, error),
) (bool, *x509.Certificate, error) {
	var ok bool
	var err error
	block, _ := pem.Decode(byteCert)
	if block == nil {
		return false, nil, errors.New("failed to parse certificate PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false, nil, errors.New("failed to parse certificate: " + err.Error())
	}

	for _, domain := range coreopts.BuildOptions.GetSupportedDomains(prod.IsProd()) {
		if ok, err = certValidationHelper(cert, domain, prod.IsProd()); ok {
			return ok, cert, err
		}
	}
	return ok, cert, err
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

	creds, credErr := tls.GetTransportCredentials(false, agentconfig.Drone)

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

	var r *cap.PenseReply
	retry := 0

	for {
		// Contact the server and print out its response.
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
		defer cancel()
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
		penseCtx, penseCancel := context.WithTimeout(context.Background(), time.Second*3)
		defer penseCancel()
		r, err = c.Pense(penseCtx, &cap.PenseRequest{Pense: penseCode, PenseIndex: pense})
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

func NewAgentConfig(tokenCache *cache.TokenCache,
	agentTokenName string,
	env string,
	acceptRemoteFunc func(*cap.FeatherContext, int, string) (bool, error),
	interruptedFunc func(*cap.FeatherContext) error,
	initNewTrcsh bool,
	isShellRunner bool,
	logger *log.Logger,
	drone ...*bool) (*AgentConfigs, *TrcShConfig, error) {

	if isShellRunner {
		tokenCache.SetVaultAddress(tokenCache.VaultAddressPtr)
		return &AgentConfigs{Env: &env}, &TrcShConfig{
			IsShellRunner: isShellRunner,
			Env:           env,
			EnvContext:    env,
			TokenCache:    tokenCache,
		}, nil
	}
	agentTokenPtr := tokenCache.GetToken(agentTokenName)
	if agentTokenPtr == nil {
		logger.Println("trcsh Failed to bootstrap")
		return nil, nil, errors.New("missing required agent auth")
	}
	if logger != nil {
		logger.Printf(".")
	} else {
		fmt.Printf(".")
	}

	mod, modErr := helperkv.NewModifier(false, agentTokenPtr, tokenCache.VaultAddressPtr, env, nil, true, nil)
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

	data, readErr := mod.ReadData(cursoropts.BuildOptions.GetCursorConfigPath())
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
		if logger != nil {
			logger.Printf(".")
		} else {
			fmt.Printf(".")
		}
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
			agentTokenPtr,
			&hatFeatherHostAddr,
			new(string),
			&deployments,
			&trcHatEnv,
			&isDrone,
		}

		if !initNewTrcsh {
			return agentconfig, nil, nil
		}

		trcshConfig := &TrcShConfig{Env: trcHatEnv,
			EnvContext: trcHatEnv,
		}

		// TODO: Chewbacca -- Local debug
		// configRole := os.Getenv("CONFIG_ROLE")
		// vaddress := os.Getenv("VAULT_ADDR")
		// tokenCache.AddRoleStr("bamboo", &configRole)
		// tokenCache.SetVaultAddress(&vaddress)
		// return agentconfig, trcshConfig, nil
		// End Chewbacca
		var penseError error
		bambooRole, penseError := agentconfig.RetryingPenseFeatherQuery("configrole")
		if penseError != nil {
			return nil, nil, penseError
		}
		if logger != nil {
			logger.Printf(".")
		} else {
			fmt.Printf(".")
		}
		tokenCache.AddRoleStr("bamboo", bambooRole)

		vaultAddressPtr, penseError := agentconfig.RetryingPenseFeatherQuery("caddress")
		if penseError != nil {
			return nil, nil, penseError
		}
		if logger != nil {
			logger.Printf(".")
		} else {
			fmt.Printf(".")
		}
		tokenCache.SetVaultAddress(vaultAddressPtr)

		if kernelopts.BuildOptions.IsKernel() && tokenCache.GetToken("config_token_pluginany") == nil {
			tokenPtr, penseError := agentconfig.RetryingPenseFeatherQuery("token")
			if penseError != nil {
				return nil, nil, penseError
			}
			tokenCache.AddToken("config_token_pluginany", tokenPtr)

			if logger != nil {
				logger.Printf(".")
			} else {
				fmt.Printf(".")
			}
		}

		return agentconfig, trcshConfig, nil
	}
}

func PenseQuery(trcshDriverConfig *TrcshDriverConfig, capPath string, pense string) (*string, error) {
	penseCode := randomString(12 + rand.Intn(7))
	penseArray := sha256.Sum256([]byte(penseCode))
	penseSum := hex.EncodeToString(penseArray[:])

	penseEye, capWriteErr := tap.TapWriter(capPath, penseSum)

	if trcHtSp, trcHSPOk := penseEye["trcHatSecretsPort"]; trcHSPOk {
		if gTrcHatSecretsPort != trcHtSp {
			gTrcHatSecretsPort = trcHtSp
		}
	}

	wantsFeathering := false
	if trcHtWf, trcHWFOk := penseEye["trcHatWantsFeathering"]; trcHWFOk {
		if trcHtWf == "true" {
			wantsFeathering = true
		}
	}

	if capWriteErr != nil || gTrcHatSecretsPort == "" {
		fmt.Println("Code 54 failure...  Possible deploy components mismatch..")
		return new(string), errors.New("tap writer error")
	}

	creds, err := tls.GetTransportCredentials(true)
	if err != nil {
		return nil, err
	}
	dialOptions := grpc.WithTransportCredentials(creds)

	capHatIpAddr := LoopBackAddr()
	if wantsFeathering {
		capHatIpAddr, err = TrcNetAddr()
		if err != nil {
			fmt.Println("Code 55 failure...  Possible deploy components mismatch..")
			return new(string), errors.New("tap writer error")
		}
	}

	conn, err := grpc.Dial(fmt.Sprintf("%s:%s", capHatIpAddr, gTrcHatSecretsPort), dialOptions)
	if err != nil {
		return new(string), err
	}
	defer conn.Close()
	c := cap.NewCapClient(conn)

	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
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
