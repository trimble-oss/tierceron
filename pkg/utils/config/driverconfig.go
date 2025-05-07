package config

import (
	"math"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/pavlo-v-chernykh/keystore-go/v4"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcsh/trcshio"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"

	"github.com/trimble-oss/tierceron/pkg/core"
)

type ProcessContext interface{}

type ConfigDriver func(ctx ProcessContext, configCtx *ConfigContext, driverConfig *DriverConfig) (interface{}, error)

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
	CoreConfig *core.CoreConfig

	// Process context used by new tool implementations in callbacks.
	// Tools can attach their own context here for later access in
	// certain callbacks.
	Context ProcessContext

	// Internal systems...
	IsShellSubProcess bool // If subshell

	FileFilter    []string // Which systems to operate on.
	SubPathFilter []string // Which subpaths to operate on.
	PathParam     string   // Path parameter for dynamic pathing...

	SecretMode bool
	// Tierceron source and destination I/O
	StartDir          []string // Starting directory. possibly multiple
	EndDir            string
	SubOutputMemCache bool
	ReadMemCache      bool
	OutputMemCache    bool
	MemFs             trcshio.MemoryFileSystem
	CertPathOverrides map[string]string // certFileName -> certDest

	// Config modes....
	NoVault     bool //Working straight from seed files
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
	ServicesWanted  []string
	ProjectSections []string
	SectionKey      string // Restricted or Index
	SectionName     string // extension provided name
	SubSectionName  string // extension provided subsection name
	SubSectionValue string
	ServiceFilter   []string // Which tables to use.

	DeploymentConfig         map[string]interface{} // For trcsh to indicate which deployment to work on
	DeploymentCtlMessageChan chan string
}

// ConfigControl Setup initializes the directory structures in preparation for parsing templates.
func ConfigControl(ctx ProcessContext, configCtx *ConfigContext, driverConfig *DriverConfig, drive ConfigDriver) {
	multiProject := false

	driverConfig.EndDir = strings.Replace(driverConfig.EndDir, "\\", "/", -1)
	if driverConfig.EndDir != "." && (strings.LastIndex(driverConfig.EndDir, "/") < (len(driverConfig.EndDir) - 1)) {
		driverConfig.EndDir = driverConfig.EndDir + "/"
	}

	startDirs := []string{}

	// Satisfy needs of templating tool with path cleanup.
	if driverConfig.StartDir[0] == coreopts.BuildOptions.GetFolderPrefix(driverConfig.StartDir)+"_templates" {
		// Set up for single service configuration when available.
		// This is the most common use of the tool.
		projectFileNames := map[string]bool{}

		if driverConfig.ReadMemCache {
			projectFilesComplete, err := driverConfig.MemFs.ReadDir(driverConfig.StartDir[0])
			if err == nil {
				for _, projectFile := range projectFilesComplete {
					projectFileNames[projectFile.Name()] = projectFile.IsDir()
				}
			}
		} else {
			pwd, err := os.Getwd()
			if err == nil {
				driverConfig.StartDir[0] = pwd + string(os.PathSeparator) + driverConfig.StartDir[0]
			}
			projectFilesComplete, err := os.ReadDir(driverConfig.StartDir[0])
			if err == nil {
				for _, projectFile := range projectFilesComplete {
					projectFileNames[projectFile.Name()] = projectFile.IsDir()
				}
			}
		}

		projectFiles := []string{}
		for projectFileName, _ := range projectFileNames {
			if !strings.HasSuffix(projectFileName, ".DS_Store") {
				projectFiles = append(projectFiles, projectFileName)
			}
		}

		if len(projectFiles) == 2 && (projectFiles[0] == "Common" || projectFiles[1] == "Common") {
			for _, projectFile := range projectFiles {
				projectStartDir := driverConfig.StartDir[0]

				if projectFile != "Common" && driverConfig.CoreConfig.WantCerts && driverConfig.WantKeystore == "" {
					// Ignore non-common if wantCerts
					continue
				}

				if projectFile == "Common" {
					projectStartDir = projectStartDir + string(os.PathSeparator) + projectFile
				} else if projectFileNames[projectFile] {
					projectStartDir = projectStartDir + string(os.PathSeparator) + projectFile
					if driverConfig.ReadMemCache {
						serviceFiles, err := driverConfig.MemFs.ReadDir(projectStartDir)
						if err == nil && len(serviceFiles) == 1 && serviceFiles[0].IsDir() {
							projectStartDir = projectStartDir + string(os.PathSeparator) + serviceFiles[0].Name()
							driverConfig.VersionFilter = append(driverConfig.VersionFilter, serviceFiles[0].Name())
						}
					} else {
						serviceFiles, err := os.ReadDir(projectStartDir)
						if err == nil && len(serviceFiles) == 1 && serviceFiles[0].IsDir() {
							projectStartDir = projectStartDir + string(os.PathSeparator) + serviceFiles[0].Name()
							driverConfig.VersionFilter = append(driverConfig.VersionFilter, serviceFiles[0].Name())
						}
					}
					if strings.LastIndex(projectStartDir, string(os.PathSeparator)) < (len(projectStartDir) - 1) {
						projectStartDir = projectStartDir + string(os.PathSeparator)
					}
				}
				projectStartDir = strings.Replace(projectStartDir, "\\", "/", -1)
				startDirs = append(startDirs, projectStartDir)
			}

			driverConfig.StartDir = startDirs
			// Drive this set of configurations.
			drive(ctx, configCtx, driverConfig)

			return
		}

		if len(projectFiles) == 1 && projectFileNames[projectFiles[0]] {
			driverConfig.StartDir[0] = driverConfig.StartDir[0] + string(os.PathSeparator) + projectFiles[0]
		} else if len(projectFiles) > 1 {
			multiProject = true
		}
		serviceFiles, err := os.ReadDir(driverConfig.StartDir[0])

		if err == nil && len(serviceFiles) == 1 && serviceFiles[0].IsDir() {
			driverConfig.StartDir[0] = driverConfig.StartDir[0] + string(os.PathSeparator) + serviceFiles[0].Name()
			driverConfig.VersionFilter = append(driverConfig.VersionFilter, serviceFiles[0].Name())
		} else if len(projectFiles) > 1 {
			multiProject = true
		}

		if len(driverConfig.VersionFilter) == 0 {
			for projectFileName, _ := range projectFileNames {
				for _, projectSection := range driverConfig.ProjectSections {
					if !strings.HasSuffix(projectFileName, ".DS_Store") && projectFileName == projectSection {
						driverConfig.VersionFilter = append(driverConfig.VersionFilter, projectFileName)
					}
				}
			}
		}
	}

	if !multiProject && strings.LastIndex(driverConfig.StartDir[0], "/") < (len(driverConfig.StartDir[0])-1) {
		driverConfig.StartDir[0] = driverConfig.StartDir[0] + "/"
	}

	// Drive this set of configurations.
	drive(ctx, configCtx, driverConfig)
}
