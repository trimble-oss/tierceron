package trcshauth

import (
	"fmt"
	"os"

	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/trcvault/carrierfactory/capauth"
	helperkv "github.com/trimble-oss/tierceron/vaulthelper/kv"
)

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

func (c *AgentConfigs) LoadConfigs(address string, agentToken string, deployments string, env string) (*TrcShConfig, error) {
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

		c.HandshakeHostPort = &trcHandshakeHostPort
		c.FeatherHostPort = &trcFeatherHostPort
		c.HandshakeCode = &trcHatHandshakeCode
		c.EncryptPass = &trcHatEncryptPass
		c.EncryptSalt = &trcHatEncryptSalt
		c.Deployments = &deployments
		c.Env = &trcHatEnv

		trcshConfig = &TrcShConfig{Env: trcHatEnv,
			EnvContext: trcHatEnv,
		}
		trcShConfigRole, penseError := capauth.PenseQuery(*c.EncryptPass,
			*c.EncryptSalt,
			*c.HandshakeHostPort,
			*c.HandshakeCode, *c.FeatherHostPort, "configrole")
		if penseError != nil {
			return nil, penseError
		}
		memprotectopts.MemProtect(nil, &trcShConfigRole)
		trcshConfig.ConfigRole = &trcShConfigRole

		trcShCToken, penseError := capauth.PenseQuery(*c.EncryptPass,
			*c.EncryptSalt,
			*c.HandshakeHostPort,
			*c.HandshakeCode, *c.FeatherHostPort, "ctoken")
		if penseError != nil {
			return nil, penseError
		}
		memprotectopts.MemProtect(nil, &trcShCToken)
		trcshConfig.CToken = &trcShCToken

	}

	return trcshConfig, nil
}
