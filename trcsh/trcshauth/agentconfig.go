package trcshauth

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

type AgentConfigs struct {
	CarrierCtlHostPort *string `yaml:"carrierCtlHostPort"`
	CarrierHostPort    *string `yaml:"carrierHostPort"`
	DeployRoleID       *string `yaml:"deployRoleID"`
	EncryptPass        *string `yaml:"encryptPass"`
	EncryptSalt        *string `yaml:"encryptSalt"`
}

func (c *AgentConfigs) LoadConfigs() (*AgentConfigs, error) {
	c.CarrierCtlHostPort = new(string)
	c.CarrierHostPort = new(string)
	c.DeployRoleID = new(string)
	c.EncryptPass = new(string)
	c.EncryptSalt = new(string)

	yamlFile, err := os.ReadFile("trcshagent.yaml")
	if err != nil {
		fmt.Printf("Failure to load config file #%v ", err)
	}

	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		return nil, err
	}

	return c, err
}
