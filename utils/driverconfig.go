package utils

import (
	"log"
	"math"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/go-git/go-billy/v5"
	"github.com/pavlo-v-chernykh/keystore-go/v4"
	"github.com/trimble-oss/tierceron-hat/cap"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
)

type ProcessContext interface{}

type ConfigDriver func(ctx ProcessContext, configCtx *ConfigContext, config *DriverConfig) (interface{}, error)

type ResultData struct {
	Done   bool
	InData *string
	InPath string
}

type ConfigContext struct {
	ResultMap            map[string]*string
	EnvSlice             []string
	ProjectSectionsSlice []string
	ResultChannel        chan *ResultData
	FileSysIndex         int
	DiffFileCount        int32
	EnvLength            int
	ConfigWg             sync.WaitGroup
	Mutex                *sync.Mutex
}

func (cfgContext *ConfigContext) SetDiffFileCount(cnt int) {
	if cnt < math.MaxInt32 {
		atomic.StoreInt32(&cfgContext.DiffFileCount, int32(cnt))
	}
}

func (cfgContext *ConfigContext) GetDiffFileCount() int32 {
	return cfgContext.DiffFileCount
}

// DriverConfig -- contains many structures necessary for Tierceron tool functionality.
type DriverConfig struct {
	// Process context used by new tool implementations in callbacks.
	// Tools can attach their own context here for later access in
	// certain callbacks.
	Context ProcessContext

	// Internal systems...
	ExitOnFailure     bool // Exit on a failure or try to continue
	Insecure          bool
	IsShell           bool // If tool running in shell.
	IsShellSubProcess bool // If subshell
	Log               *log.Logger

	// Vault Configurations...
	Token         string
	VaultAddress  string
	EnvRaw        string // May change depending on tool running...
	Env           string // Immutable...
	Regions       []string
	FileFilter    []string // Which systems to operate on.
	SubPathFilter []string // Which subpaths to operate on.

	SecretMode bool
	// Tierceron source and destination I/O
	StartDir       []string // Starting directory. possibly multiple
	EndDir         string
	OutputMemCache bool
	MemFs          billy.Filesystem
	FeatherCtx     *cap.FeatherContext
	FeatherCtlCb   func(*cap.FeatherContext, string) error

	// Config modes....
	WantCerts   bool
	ZeroConfig  bool
	GenAuth     bool
	TrcShellRaw string   //Used for TrcShell
	Trcxe       []string //Used for TRCXE
	Trcxr       bool     //Used for TRCXR

	Clean  bool
	Update func(*ConfigContext, *string, string)

	// KeyStore Output tooling
	KeyStore         *keystore.KeyStore
	KeystorePassword string
	WantKeystore     string // If provided and non nil, pem files will be put into a java compatible keystore.

	// Diff tooling
	Diff          bool
	DiffCounter   int
	VersionInfo   func(map[string]interface{}, bool, string, bool)
	VersionFilter []string

	// Vault Pathing....
	// This section stores information useful in directing I/O with Vault.
	AppRoleConfig     string
	ServicesWanted    []string
	ProjectSections   []string
	SectionKey        string // Restricted or Index
	SectionName       string // extension provided name
	SubSectionName    string // extension provided subsection name
	SubSectionValue   string
	ServiceFilter     []string // Which tables to use.
	DynamicPathFilter string   // Seeds from a specific path.

	DeploymentConfig         map[string]interface{} // For trcsh to indicate which deployment to work on
	DeploymentCtlMessageChan chan string
}

// ConfigControl Setup initializes the directory structures in preparation for parsing templates.
func ConfigControl(ctx ProcessContext, configCtx *ConfigContext, config *DriverConfig, drive ConfigDriver) {
	multiProject := false

	config.EndDir = strings.Replace(config.EndDir, "\\", "/", -1)
	if config.EndDir != "." && (strings.LastIndex(config.EndDir, "/") < (len(config.EndDir) - 1)) {
		config.EndDir = config.EndDir + "/"
	}

	startDirs := []string{}

	// Satisfy needs of templating tool with path cleanup.
	if config.StartDir[0] == coreopts.BuildOptions.GetFolderPrefix(config.StartDir)+"_templates" {
		// Set up for single service configuration when available.
		// This is the most common use of the tool.
		pwd, err := os.Getwd()
		if err == nil {
			config.StartDir[0] = pwd + string(os.PathSeparator) + config.StartDir[0]
		}

		projectFilesComplete, err := os.ReadDir(config.StartDir[0])
		projectFiles := []os.DirEntry{}
		for _, projectFile := range projectFilesComplete {
			if !strings.HasSuffix(projectFile.Name(), ".DS_Store") {
				projectFiles = append(projectFiles, projectFile)
			}
		}

		if len(projectFiles) == 2 && (projectFiles[0].Name() == "Common" || projectFiles[1].Name() == "Common") {
			for _, projectFile := range projectFiles {
				projectStartDir := config.StartDir[0]

				if projectFile.Name() != "Common" && config.WantCerts && config.WantKeystore == "" {
					// Ignore non-common if wantCerts
					continue
				}

				if projectFile.Name() == "Common" {
					projectStartDir = projectStartDir + string(os.PathSeparator) + projectFile.Name()
				} else if projectFile.IsDir() {
					projectStartDir = projectStartDir + string(os.PathSeparator) + projectFile.Name()
					serviceFiles, err := os.ReadDir(projectStartDir)
					if err == nil && len(serviceFiles) == 1 && serviceFiles[0].IsDir() {
						projectStartDir = projectStartDir + string(os.PathSeparator) + serviceFiles[0].Name()
						config.VersionFilter = append(config.VersionFilter, serviceFiles[0].Name())
					}
					if strings.LastIndex(projectStartDir, string(os.PathSeparator)) < (len(projectStartDir) - 1) {
						projectStartDir = projectStartDir + string(os.PathSeparator)
					}
				}
				projectStartDir = strings.Replace(projectStartDir, "\\", "/", -1)
				startDirs = append(startDirs, projectStartDir)
			}

			config.StartDir = startDirs
			// Drive this set of configurations.
			drive(ctx, configCtx, config)

			return
		}

		if err == nil && len(projectFiles) == 1 && projectFiles[0].IsDir() {
			config.StartDir[0] = config.StartDir[0] + string(os.PathSeparator) + projectFiles[0].Name()
		} else if len(projectFiles) > 1 {
			multiProject = true
		}
		serviceFiles, err := os.ReadDir(config.StartDir[0])

		if err == nil && len(serviceFiles) == 1 && serviceFiles[0].IsDir() {
			config.StartDir[0] = config.StartDir[0] + string(os.PathSeparator) + serviceFiles[0].Name()
			config.VersionFilter = append(config.VersionFilter, serviceFiles[0].Name())
		} else if len(projectFiles) > 1 {
			multiProject = true
		}

		if len(config.VersionFilter) == 0 {
			for _, projectFile := range projectFilesComplete {
				for _, projectSection := range config.ProjectSections {
					if !strings.HasSuffix(projectFile.Name(), ".DS_Store") && projectFile.Name() == projectSection {
						config.VersionFilter = append(config.VersionFilter, projectFile.Name())
					}
				}
			}
		}
	}

	if !multiProject && strings.LastIndex(config.StartDir[0], "/") < (len(config.StartDir[0])-1) {
		config.StartDir[0] = config.StartDir[0] + "/"
	}

	// Drive this set of configurations.
	drive(ctx, configCtx, config)
}
