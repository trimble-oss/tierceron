package utils

import (
	"fmt"
	"testing"

	"github.com/trimble-oss/tierceron/pkg/core"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
)

func TestGeneratePaths_nil(t *testing.T) {
	_, _, err := generatePaths(nil)
	if err == nil {
		fmt.Printf("Expected nil config error, got %s\n", err)
		t.Fatalf("Expected nil config error, got %v", err)
	}
}

func TestGeneratePaths_BaseCase(t *testing.T) {
	// Single starting Dir, ServicesWanted
	// If this test fails, drone will fail
	ConfiginatorOsPathSeparator = "/"
	driverConfig := &config.DriverConfig{
		CoreConfig: &core.CoreConfig{
			WantCerts: false,
		},
		StartDir: []string{
			"foo/bar/fake path./ to _see if it Panics....",
		},
		EndDir:         ".",
		ServicesWanted: []string{"Project/Service"},
	}

	_, _, err := generatePaths(driverConfig)

	if err != nil {
		fmt.Printf("Expected no error, got %s\n", err)
		t.Fatalf("Expected no error, got %v", err)
	}
}

func TestGeneratePaths_BaseCaseWin(t *testing.T) {
	// Single starting Dir, ServicesWanted
	// If this test fails, drone will fail
	ConfiginatorOsPathSeparator = "\\"
	driverConfig := &config.DriverConfig{
		CoreConfig: &core.CoreConfig{
			WantCerts: false,
		},
		StartDir: []string{
			"foo/bar/fake path./ to _see if it Panics....",
		},
		EndDir:         ".",
		ServicesWanted: []string{"Project/Service"},
	}

	templatePaths, endPaths, err := generatePaths(driverConfig)

	if err != nil {
		fmt.Printf("Expected no error, got %s\n", err)
		t.Fatalf("Expected no error, got %v", err)
	}
	if len(templatePaths) != 1 || len(endPaths) != 1 {
		fmt.Println("Expected different amount of paths returned")
		t.Fatal("Expected different amount of paths returned\n")
	}
	if templatePaths[0] != "foo\\bar\\fake path.\\ to _see if it Panics....\\" {
		fmt.Printf("Expected different template path, instead got: %s\n", templatePaths[0])
		t.Fatalf("Expected different template path, instead got: %s\n", templatePaths[0])
	}
	if endPaths[0] != "." {
		fmt.Printf("Expected different end path, instead got: %s\n", endPaths[0])
		t.Fatalf("Expected different end path, instead got: %s\n", endPaths[0])
	}
}

func FuzzBasicTestGeneratePaths_CaseOne(f *testing.F) {
	ConfiginatorOsPathSeparator = "/"
	f.Add(5, "hello")
	f.Fuzz(func(t *testing.T, i int, s string) {
		driverConfig := &config.DriverConfig{
			CoreConfig: &core.CoreConfig{
				WantCerts: false,
			},
			StartDir:         []string{},
			DeploymentConfig: make(map[string]interface{}),
			EndDir:           ".",
			ServicesWanted:   []string{"hello/Service"},
		}
		for j := 0; j < i; j++ {
			driverConfig.StartDir = append(driverConfig.StartDir, s)
		}

		templatePaths, endPaths, err := generatePaths(driverConfig)

		if err != nil {
			fmt.Printf("Expected no error, instead got %s\n", err)
			t.Errorf("Expected no error, instead got %v\n", err)
		}
		if len(templatePaths) != 5 || len(endPaths) != 5 {
			fmt.Println("Expected different amount of paths returned")
			t.Fatal("Expected different amount of paths returned\n")
		}
		for _, path := range templatePaths {
			if path != "hello/" {
				fmt.Printf("Expected different template path, instead got: %s\n", templatePaths[0])
				t.Fatalf("Expected different template path, instead got: %s\n", templatePaths[0])
			}
		}
		for _, ePath := range endPaths {
			if ePath != "." {
				fmt.Printf("Expected different end path, instead got: %s\n", endPaths[0])
				t.Fatalf("Expected different end path, instead got: %s\n", endPaths[0])
			}
		}
	})
}

func TestGeneratePaths_CaseOne(t *testing.T) {
	// Multiple invalid starting directories, multiple project/services defined, ServicesWanted specified
	ConfiginatorOsPathSeparator = "/"
	driverConfig := &config.DriverConfig{
		CoreConfig: &core.CoreConfig{
			WantCerts: false,
		},
		StartDir: []string{
			"foo/project/",
			"bar/notProject/",
			"hello/nProject ",
			"aijfiosdfc/Project /",
			".Project/",
			"foo/nopeProject/Service",
		},
		DeploymentConfig: make(map[string]interface{}),
		EndDir:           "~/checking...if\\other characters _/will_cause_panic-!",
		ServicesWanted:   []string{"Project/Service"},
	}
	driverConfig.DeploymentConfig["trcprojectservice"] = "Project1/Service1"

	_, _, err := generatePaths(driverConfig)
	if err == nil {
		fmt.Printf("Expected invalid starting directory error, got %s\n", err)
		t.Fatalf("Expected invalid starting directory error, got %v", err)
	}
}

func TestGeneratePaths_BadProjServ(t *testing.T) {
	// Multiple invalid starting directories, multiple project/services defined, ServicesWanted specified incorrectly
	ConfiginatorOsPathSeparator = "/"
	driverConfig := &config.DriverConfig{
		CoreConfig: &core.CoreConfig{
			WantCerts: false,
		},
		StartDir: []string{
			"foo/project/",
			"bar/notProject/",
			"hello/Project ",
			"aijfiosdfc/Project /",
			".Project/",
			"foo/Project/Service",
		},
		DeploymentConfig: make(map[string]interface{}),
		EndDir:           "~/checking...if\\other characters _/will_cause_panic-!",
		ServicesWanted:   []string{"ProjectService"},
	}
	driverConfig.DeploymentConfig["trcprojectservice"] = "Project1/Service1"

	_, _, err := generatePaths(driverConfig)
	if err == nil {
		fmt.Printf("Expected Project/Service formatting error, got %s\n", err)
		t.Fatalf("Expected Project/Service formatting error, got %v", err)
	}
}

func TestGeneratePaths_CaseTwo(t *testing.T) {
	// Multiple starting directories, multiple project/services defined, ServicesWanted specified
	ConfiginatorOsPathSeparator = "/"
	driverConfig := &config.DriverConfig{
		CoreConfig: &core.CoreConfig{
			WantCerts: false,
		},
		StartDir: []string{
			"~/foo/Project1",
			"~/bar/Project/",
			"~/hello.world/Project",
		},
		DeploymentConfig: make(map[string]interface{}),
		EndDir:           "~/checking...if\\other characters _/will_cause_panic-!",
		ServicesWanted:   []string{"Project/Service"},
	}
	driverConfig.DeploymentConfig["trcprojectservice"] = "Project1/Service1"

	templatePaths, endPaths, err := generatePaths(driverConfig)

	if err != nil {
		fmt.Printf("Expected no error, got %s\n", err)
		t.Fatalf("Expected no error, got %v", err)
	}
	if len(templatePaths) != 2 || len(endPaths) != 2 {
		fmt.Println("Expected different amount of paths returned")
		t.Fatal("Expected different amount of paths returned\n")
	}
	if templatePaths[0] != "~/bar/Project/" && templatePaths[1] != "~/hello.world/Project/" {
		fmt.Printf("Expected different template path, instead got: %s\n", templatePaths[0])
		t.Fatalf("Expected different template path, instead got: %s\n", templatePaths[0])
	}

	if endPaths[0] != "~/checking...if\\other characters _/will_cause_panic-!" && endPaths[1] != "~/checking...if\\other characters _/will_cause_panic-!" {
		fmt.Printf("Expected different end path, instead got: %s\n", endPaths[0])
		t.Fatalf("Expected different end path, instead got: %s\n", endPaths[0])
	}
}

func TestGeneratePaths_CaseTwoWin(t *testing.T) {
	// Multiple starting directories, multiple project/services defined, ServicesWanted specified
	ConfiginatorOsPathSeparator = "\\"
	driverConfig := &config.DriverConfig{
		CoreConfig: &core.CoreConfig{
			WantCerts: false,
		},
		StartDir: []string{
			"~/foo/Project1",
			"~/bar/Project/",
			"~/hello.world/Project",
		},
		DeploymentConfig: make(map[string]interface{}),
		EndDir:           "~/checking...if\\other characters _/will_cause_panic-!",
		ServicesWanted:   []string{"Project/Service"},
	}
	driverConfig.DeploymentConfig["trcprojectservice"] = "Project1/Service1"

	templatePaths, endPaths, err := generatePaths(driverConfig)

	if err != nil {
		fmt.Printf("Expected no error, got %s\n", err)
		t.Fatalf("Expected no error, got %v", err)
	}
	if len(templatePaths) != 2 || len(endPaths) != 2 {
		fmt.Println("Expected different amount of paths returned")
		t.Fatal("Expected different amount of paths returned\n")
	}
	if templatePaths[0] != "~\\bar\\Project\\" && templatePaths[1] != "~\\hello.world\\Project\\" {
		fmt.Printf("Expected different template path, instead got: %s and %s\n", templatePaths[0], templatePaths[1])
		t.Fatalf("Expected different template path, instead got: %sand %s\n", templatePaths[0], templatePaths[1])
	}

	if endPaths[0] != "~/checking...if\\other characters _/will_cause_panic-!" && endPaths[1] != "~/checking...if\\other characters _/will_cause_panic-!" {
		fmt.Printf("Expected different end path, instead got: %s and %s\n", endPaths[0], endPaths[1])
		t.Fatalf("Expected different end path, instead got: %s and %s\n", endPaths[0], endPaths[1])
	}
}

func TestGeneratePaths_CaseThree(t *testing.T) {
	// Multiple starting directories, multiple project/services defined, ServicesWanted specified
	ConfiginatorOsPathSeparator = "/"
	driverConfig := &config.DriverConfig{
		CoreConfig: &core.CoreConfig{
			WantCerts: false,
		},
		StartDir: []string{
			"hellos/bonjour/fake.tmpl",
			"hello",
		},
		DeploymentConfig: make(map[string]interface{}),
		EndDir:           "hello/bonjour",
		ServicesWanted:   []string{"hello/world/seeing/if/it/works//with random words"},
	}

	_, _, err := generatePaths(driverConfig)
	if err == nil {
		fmt.Printf("Expected error, got %s\n", err)
		t.Fatalf("Expected error, got %v", err)
	}
}

func TestGeneratePaths_CaseThreeWin(t *testing.T) {
	// Multiple starting directories, multiple project/services defined, ServicesWanted specified
	ConfiginatorOsPathSeparator = "\\"
	driverConfig := &config.DriverConfig{
		CoreConfig: &core.CoreConfig{
			WantCerts: false,
		},
		StartDir: []string{
			"hellos/bonjour/fake.tmpl",
			"hello",
		},
		DeploymentConfig: make(map[string]interface{}),
		EndDir:           "hello/bonjour",
		ServicesWanted:   []string{"hello/world/seeing/if/it/works//with random words"},
	}

	_, _, err := generatePaths(driverConfig)
	if err == nil {
		fmt.Printf("Expected error, got %s\n", err)
		t.Fatalf("Expected error, got %v", err)
	}
}

func TestGeneratePaths_CaseFour(t *testing.T) {
	// Single starting directory, single project/service defined, ServicesWanted not specified, no scrubbing
	ConfiginatorOsPathSeparator = "/"
	driverConfig := &config.DriverConfig{
		CoreConfig: &core.CoreConfig{
			WantCerts: false,
		},
		StartDir: []string{
			"hello/bonjour/Project",
		},
		DeploymentConfig: make(map[string]interface{}),
		EndDir:           "hello/bonjour",
		ServicesWanted:   []string{},
	}
	driverConfig.DeploymentConfig["trcprojectservice"] = "Project/Service"

	templatePaths, endPaths, err := generatePaths(driverConfig)

	if err != nil {
		fmt.Printf("Expected no error, got %s\n", err)
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(templatePaths) != 1 || len(endPaths) != 1 {
		fmt.Println("Expected different amount of paths returned")
		t.Fatal("Expected different amount of paths returned\n")
	}
	if templatePaths[0] != "hello/bonjour/Project/" {
		fmt.Printf("Expected different template path, instead got: %s\n", templatePaths[0])
		t.Fatalf("Expected different template path, instead got: %s\n", templatePaths[0])
	}
	if endPaths[0] != "hello/bonjour" {
		fmt.Printf("Expected different end path, instead got: %s\n", endPaths[0])
		t.Fatalf("Expected different end path, instead got: %s\n", endPaths[0])
	}
}

func TestGeneratePaths_CaseFourWin(t *testing.T) {
	// Single starting directory, single project/service defined, ServicesWanted not specified, no scrubbing
	ConfiginatorOsPathSeparator = "\\"
	driverConfig := &config.DriverConfig{
		CoreConfig: &core.CoreConfig{
			WantCerts: false,
		},
		StartDir: []string{
			"hello/bonjour/Project",
		},
		DeploymentConfig: make(map[string]interface{}),
		EndDir:           "hello/bonjour",
		ServicesWanted:   []string{},
	}
	driverConfig.DeploymentConfig["trcprojectservice"] = "Project/Service"

	templatePaths, endPaths, err := generatePaths(driverConfig)

	if err != nil {
		fmt.Printf("Expected no error, got %s\n", err)
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(templatePaths) != 1 || len(endPaths) != 1 {
		fmt.Println("Expected different amount of paths returned")
		t.Fatal("Expected different amount of paths returned\n")
	}
	if templatePaths[0] != "hello\\bonjour\\Project\\" {
		fmt.Printf("Expected different template path, instead got: %s\n", templatePaths[0])
		t.Fatalf("Expected different template path, instead got: %s\n", templatePaths[0])
	}
	if endPaths[0] != "hello/bonjour" {
		fmt.Printf("Expected different end path, instead got: %s\n", endPaths[0])
		t.Fatalf("Expected different end path, instead got: %s\n", endPaths[0])
	}
}

func TestGeneratePaths_CaseFive(t *testing.T) {
	// Single starting directory, single project/service defined w/out separator, ServicesWanted not specified, no scrubbing
	ConfiginatorOsPathSeparator = "/"
	driverConfig := &config.DriverConfig{
		CoreConfig: &core.CoreConfig{},
		StartDir: []string{
			"hello/bonjour",
		},
		DeploymentConfig: make(map[string]interface{}),
		EndDir:           "hello/bonjour",
		ServicesWanted:   []string{},
	}
	driverConfig.DeploymentConfig["trcprojectservice"] = "ProjectService"

	_, _, err := generatePaths(driverConfig)

	if err == nil {
		fmt.Printf("Expected project and service specified incorrecly error, instead got %s\n", err)
		t.Fatalf("Expected project and service specified incorrecly error, instead got %s\n", err)
	}
}

func TestGeneratePaths_CaseFiveWin(t *testing.T) {
	// Single starting directory, single project/service defined w/out separator, ServicesWanted not specified, no scrubbing
	ConfiginatorOsPathSeparator = "\\"
	driverConfig := &config.DriverConfig{
		CoreConfig: &core.CoreConfig{},
		StartDir: []string{
			"hello/bonjour",
		},
		DeploymentConfig: make(map[string]interface{}),
		EndDir:           "hello/bonjour",
		ServicesWanted:   []string{},
	}
	driverConfig.DeploymentConfig["trcprojectservice"] = "ProjectService"

	_, _, err := generatePaths(driverConfig)

	if err == nil {
		fmt.Printf("Expected project and service specified incorrecly error, instead got %s\n", err)
		t.Fatalf("Expected project and service specified incorrecly error, instead got %s\n", err)
	}
}

func TestGeneratePaths_CaseSix(t *testing.T) {
	// Single starting directory, single project/service, ServicesWanted not specified, scrubbing
	ConfiginatorOsPathSeparator = "/"
	driverConfig := &config.DriverConfig{
		CoreConfig: &core.CoreConfig{
			WantCerts: false,
		},
		StartDir: []string{
			"hello/bonjour/Project",
			"~./Project",
			"~\\hello\\/Project",
		},
		DeploymentConfig: make(map[string]interface{}),
		EndDir:           "hello/world/Project/Service/bonjour/monde",
		ServicesWanted:   []string{},
	}
	driverConfig.DeploymentConfig["trcprojectservice"] = "Project/Service"

	templatePaths, endPaths, err := generatePaths(driverConfig)

	if err != nil {
		fmt.Printf("Expected no error, got %s\n", err)
		t.Fatalf("Expected no error, got %v", err)
	}
	if len(templatePaths) != 3 || len(endPaths) != 3 {
		fmt.Println("Expected different amount of paths returned")
		t.Fatal("Expected different amount of paths returned\n")
	}
	if templatePaths[0] != "hello/bonjour/Project/" {
		fmt.Printf("Expected different template path, instead got: %s\n", templatePaths[0])
		t.Fatalf("Expected different template path, instead got: %s\n", templatePaths[0])
	}
	if templatePaths[1] != "~./Project/" {
		fmt.Printf("Expected different template path, instead got: %s\n", templatePaths[1])
		t.Fatalf("Expected different template path, instead got: %s\n", templatePaths[1])
	}
	if templatePaths[2] != "~/hello//Project/" {
		fmt.Printf("Expected different template path, instead got: %s\n", templatePaths[2])
		t.Fatalf("Expected different template path, instead got: %s\n", templatePaths[2])
	}
	for i := 0; i < 3; i++ {
		if endPaths[i] != "hello/world/bonjour/monde" {
			fmt.Printf("Expected different end path, instead got: %s\n", endPaths[i])
			t.Fatalf("Expected different end path, instead got: %s\n", endPaths[i])
		}
	}
}

func TestGeneratePaths_CaseSixWin(t *testing.T) {
	// Single starting directory, single project/service, ServicesWanted not specified, scrubbing
	ConfiginatorOsPathSeparator = "\\"
	driverConfig := &config.DriverConfig{
		CoreConfig: &core.CoreConfig{
			WantCerts: false,
		},
		StartDir: []string{
			"hello/bonjour/Project",
			"~./Project",
			"~\\hello\\/Project",
		},
		DeploymentConfig: make(map[string]interface{}),
		EndDir:           "hello\\world\\Project\\Service\\bonjour\\monde",
		ServicesWanted:   []string{},
	}
	driverConfig.DeploymentConfig["trcprojectservice"] = "Project/Service"

	templatePaths, endPaths, err := generatePaths(driverConfig)

	if err != nil {
		fmt.Printf("Expected no error, got %s\n", err)
		t.Fatalf("Expected no error, got %v", err)
	}
	if len(templatePaths) != 3 || len(endPaths) != 3 {
		fmt.Println("Expected different amount of paths returned")
		t.Fatal("Expected different amount of paths returned\n")
	}
	if templatePaths[0] != "hello\\bonjour\\Project\\" {
		fmt.Printf("Expected different template path, instead got: %s\n", templatePaths[0])
		t.Fatalf("Expected different template path, instead got: %s\n", templatePaths[0])
	}
	if templatePaths[1] != "~.\\Project\\" {
		fmt.Printf("Expected different template path, instead got: %s\n", templatePaths[1])
		t.Fatalf("Expected different template path, instead got: %s\n", templatePaths[1])
	}
	if templatePaths[2] != "~\\hello\\\\Project\\" {
		fmt.Printf("Expected different template path, instead got: %s\n", templatePaths[2])
		t.Fatalf("Expected different template path, instead got: %s\n", templatePaths[2])
	}
}

func TestGeneratePaths_CaseSeven(t *testing.T) {
	// Single starting directory, single project/service, ServicesWanted not specified, no scrubbing
	ConfiginatorOsPathSeparator = "/"
	driverConfig := &config.DriverConfig{
		CoreConfig: &core.CoreConfig{
			WantCerts: true,
		},
		StartDir: []string{
			"hello/bonjour",
		},
		DeploymentConfig: make(map[string]interface{}),
		EndDir:           "hello/world/Project/Service/bonjour/monde",
		ServicesWanted:   []string{},
	}
	driverConfig.DeploymentConfig["trcprojectservice"] = "Project/Service"

	templatePaths, endPaths, err := generatePaths(driverConfig)

	if err != nil {
		fmt.Printf("Expected no error, got %s\n", err)
		t.Fatalf("Expected no error, got %v", err)
	}
	if len(templatePaths) != 1 || len(endPaths) != 1 {
		fmt.Println("Expected different amount of paths returned")
		t.Fatal("Expected different amount of paths returned\n")
	}
	if templatePaths[0] != "hello/bonjour/" {
		fmt.Printf("Expected different template path, instead got: %s\n", templatePaths[0])
		t.Fatalf("Expected different template path, instead got: %s\n", templatePaths[0])
	}
	if endPaths[0] != "hello/world/Project/Service/bonjour/monde" {
		fmt.Printf("Expected different end path, instead got: %s\n", endPaths[0])
		t.Fatalf("Expected different end path, instead got: %s\n", endPaths[0])
	}
}

func TestGeneratePaths_CaseSevenWin(t *testing.T) {
	// Single starting directory, single project/service, ServicesWanted not specified, no scrubbing
	ConfiginatorOsPathSeparator = "\\"
	driverConfig := &config.DriverConfig{
		CoreConfig: &core.CoreConfig{
			WantCerts: true,
		},
		StartDir: []string{
			"C:\\hello\\bonjour",
		},
		DeploymentConfig: make(map[string]interface{}),
		EndDir:           "hello/world/Project/Service/bonjour/monde",
		ServicesWanted:   []string{},
	}
	driverConfig.DeploymentConfig["trcprojectservice"] = "Project/Service"

	templatePaths, endPaths, err := generatePaths(driverConfig)

	if err != nil {
		fmt.Printf("Expected no error, got %s\n", err)
		t.Fatalf("Expected no error, got %v", err)
	}
	if len(templatePaths) != 1 || len(endPaths) != 1 {
		fmt.Println("Expected different amount of paths returned")
		t.Fatal("Expected different amount of paths returned\n")
	}
	if templatePaths[0] != "C:\\hello\\bonjour\\" {
		fmt.Printf("Expected different template path, instead got: %s\n", templatePaths[0])
		t.Fatalf("Expected different template path, instead got: %s\n", templatePaths[0])
	}
	if endPaths[0] != "hello/world/Project/Service/bonjour/monde" {
		fmt.Printf("Expected different end path, instead got: %s\n", endPaths[0])
		t.Fatalf("Expected different end path, instead got: %s\n", endPaths[0])
	}
}

func TestGeneratePaths_CaseEightWin(t *testing.T) {
	// Single starting directory, single project/service, ServicesWanted not specified, scrubbing
	ConfiginatorOsPathSeparator = "\\"
	driverConfig := &config.DriverConfig{
		CoreConfig: &core.CoreConfig{
			WantCerts: false,
		},
		StartDir: []string{
			"D:\\hello\\bonjour",
		},
		DeploymentConfig: make(map[string]interface{}),
		EndDir:           "hello/world/Project/Service/bonjour/monde",
		ServicesWanted:   []string{},
	}
	driverConfig.DeploymentConfig["trcprojectservice"] = "Project/Service"

	templatePaths, endPaths, err := generatePaths(driverConfig)

	if err != nil {
		fmt.Printf("Expected no error, got %s\n", err)
		t.Fatalf("Expected no error, got %v", err)
	}
	if len(templatePaths) != 1 || len(endPaths) != 1 {
		fmt.Println("Expected different amount of paths returned")
		t.Fatal("Expected different amount of paths returned\n")
	}
	if templatePaths[0] != "D:\\hello\\bonjour\\" {
		fmt.Printf("Expected different template path, instead got: %s\n", templatePaths[0])
		t.Fatalf("Expected different template path, instead got: %s\n", templatePaths[0])
	}
	if endPaths[0] != "hello/world/Project/Service/bonjour/monde" {
		fmt.Printf("Expected different end path, instead got: %s\n", endPaths[0])
		t.Fatalf("Expected different end path, instead got: %s\n", endPaths[0])
	}
}

func TestGeneratePaths_CaseNineWin(t *testing.T) {
	// Single starting directory, single project/service, ServicesWanted not specified, scrubbing
	ConfiginatorOsPathSeparator = "\\"
	driverConfig := &config.DriverConfig{
		CoreConfig: &core.CoreConfig{
			WantCerts: false,
		},
		StartDir: []string{
			"D:\\hello\\bonjour",
			"C:\\hello\\Project\\",
		},
		DeploymentConfig: make(map[string]interface{}),
		EndDir:           "hello/world/Project/Service/bonjour/monde",
		ServicesWanted:   []string{},
	}
	driverConfig.DeploymentConfig["trcprojectservice"] = "Project/Service"

	templatePaths, endPaths, err := generatePaths(driverConfig)

	if err != nil {
		fmt.Printf("Expected no error, got %s\n", err)
		t.Fatalf("Expected no error, got %v", err)
	}
	if len(templatePaths) != 1 || len(endPaths) != 1 {
		fmt.Println("Expected different amount of paths returned")
		t.Fatal("Expected different amount of paths returned\n")
	}
	if templatePaths[0] != "C:\\hello\\Project\\" {
		fmt.Printf("Expected different template path, instead got: %s\n", templatePaths[0])
		t.Fatalf("Expected different template path, instead got: %s\n", templatePaths[0])
	}
	if endPaths[0] != "hello/world/Project/Service/bonjour/monde" {
		fmt.Printf("Expected different end path, instead got: %s\n", endPaths[0])
		t.Fatalf("Expected different end path, instead got: %s\n", endPaths[0])
	}
}
