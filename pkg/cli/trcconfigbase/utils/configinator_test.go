package utils

import (
	"fmt"
	"strings"
	"testing"

	"github.com/trimble-oss/tierceron/pkg/core"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
)

func TestGeneratePaths_nil(t *testing.T) {
	_, _, err := generatePaths(nil)
	if err == nil {
		fmt.Printf("Expected nil config error, got %s\n", err)
		t.Fatalf("Expected nil config error, got %v", err)
	}
}

func TestGeneratePaths(t *testing.T) {
	// var driverConfig *eUtils.DriverConfig

	driverConfig := &eUtils.DriverConfig{
		CoreConfig: core.CoreConfig{
			IsShell:           false,
			Insecure:          false,
			Token:             "",
			AppRoleConfig:     "",
			VaultAddress:      "",
			EnvBasis:          "",
			Env:               "",
			Regions:           []string{},
			DynamicPathFilter: "",
			WantCerts:         false,
			ExitOnFailure:     true,
		},
		IsShellSubProcess: false,
		FileFilter:        []string{""},
		PathParam:         "",
		SecretMode:        true,
		StartDir: []string{
			"~/workspace/Github/trimble-oss/pendentive/trc_templates/Common/",
		},
		EndDir:           ".",
		OutputMemCache:   false,
		ZeroConfig:       false,
		GenAuth:          false,
		TrcShellRaw:      "",
		Trcxr:            false,
		Clean:            false,
		KeystorePassword: "",
		WantKeystore:     "",
		Diff:             false,
		DiffCounter:      0,
		ServicesWanted:   []string{"Spectrum/SpectrumAurora"},
		SectionKey:       "",
		SectionName:      "",
		SubSectionName:   "",
		SubSectionValue:  "",
	}

	templatePaths, endPaths, err := generatePaths(driverConfig)

	if err != nil {
		fmt.Printf("Expected no error, got %s\n", err)
		t.Fatalf("Expected no error, got %v", err)
	}
	for i := 0; i < len(templatePaths); i++ {
		if !strings.Contains(templatePaths[i], "workspace/Github/trimble-oss/pendentive/trc_templates/Common/") {
			fmt.Printf("Expected different template path, instead got: %s\n", templatePaths[i])
			t.Fatalf("Expected different template path, instead got: %s\n", templatePaths[i])
		}
	}

	for i := 0; i < len(endPaths); i++ {
		if !strings.Contains(endPaths[i], ".") {
			fmt.Printf("Expected different end path, instead got: %s\n", endPaths[i])
			t.Fatalf("Expected different end path, instead got: %s\n", endPaths[i])
		}
	}
}
