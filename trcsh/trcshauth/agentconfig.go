package trcshauth

import (
	"fmt"
	"os"

	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	helperkv "github.com/trimble-oss/tierceron/vaulthelper/kv"
)

type AgentConfigs struct {
	CarrierCtlHostPort *string
	CarrierHostPort    *string
	DeployRoleID       *string
	EncryptPass        *string
	EncryptSalt        *string
	Deployments        *string
	Env                *string
}

func (c *AgentConfigs) LoadConfigs(address string, agentToken string, deployments string, env string) (*AgentConfigs, error) {

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
		trcCarrierCtlHostPort := trcHatHost + ":" + trcHatHandshakePort
		memprotectopts.MemProtect(nil, &trcCarrierCtlHostPort)
		trcCarrierHostPort := trcHatHost + ":" + trcHatSecretsPort
		memprotectopts.MemProtect(nil, &trcCarrierHostPort)

		c.CarrierCtlHostPort = &trcCarrierCtlHostPort
		c.CarrierHostPort = &trcCarrierHostPort
		c.DeployRoleID = &trcHatHandshakeCode
		c.EncryptPass = &trcHatEncryptPass
		c.EncryptSalt = &trcHatEncryptSalt
		c.Deployments = &deployments
		c.Env = &trcHatEnv
	}

	return c, nil
}
