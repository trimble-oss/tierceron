package native

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/trimble-oss/tierceron/pkg/utils/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/klog/v2"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/kubectl/pkg/cmd"
	"k8s.io/kubectl/pkg/cmd/apply"
	"k8s.io/kubectl/pkg/cmd/plugin"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	kubectlutil "k8s.io/kubectl/pkg/util"

	"github.com/trimble-oss/tierceron-core/v2/core/coreconfig"
	"github.com/trimble-oss/tierceron-core/v2/trcshfs/trcshio"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcsh/kube/native/path"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcsh/kube/native/trccreate"
	"github.com/trimble-oss/tierceron/pkg/capauth"
)

const (
	maxPatchRetry                        = 5
	warningNoLastAppliedConfigAnnotation = "Warning: resource %[1]s is missing the %[2]s annotation which is required by %[3]s apply. %[3]s apply should only be used on resources created declaratively by either %[3]s create --save-config or %[3]s apply. The missing annotation will be patched automatically.\n"
)

type TrcKubeContext struct {
	User      string
	Cluster   string
	Context   string
	Namespace string
}

type TrcKubeDirective struct {
	Action       string // Probably not used...
	Object       string
	Type         string
	Name         string
	FromFilePath string
	DryRun       bool
}

type TrcKubeConfig struct {
	// .kube/config
	KubeConfigBytes []byte
	// .kube/config is parsed into these fields...
	RestConfig *rest.Config
	ApiConfig  map[string]*clientcmdapi.Config

	// config context stuff..
	KubeContext *TrcKubeContext
	// Current kubectl directive... configmap, secret, apply, etc...
	KubeDirective *TrcKubeDirective

	PipeOS trcshio.TrcshReadWriteCloser // Where to send output.
}

func loadInMemoryKubeConfig(trcKubeDeploymentConfig *TrcKubeConfig, filename string) (*clientcmdapi.Config, error) {
	if trcKubeDeploymentConfig == nil {
		return nil, fmt.Errorf("missing tierceron kube configuration")
	}
	if apiConfig, ok := trcKubeDeploymentConfig.ApiConfig[filename]; ok {
		return apiConfig, nil
	}
	if len(trcKubeDeploymentConfig.KubeConfigBytes) == 0 {
		return nil, fmt.Errorf("tierceron requires in-memory kubeconfig bytes; disk kubeconfig loading is disabled")
	}

	config, err := clientcmd.Load(trcKubeDeploymentConfig.KubeConfigBytes)
	if err != nil {
		return nil, err
	}
	klog.V(6).Infoln("Config loaded from memory for:", filename)

	for key, obj := range config.AuthInfos {
		obj.LocationOfOrigin = filename
		config.AuthInfos[key] = obj
	}
	for key, obj := range config.Clusters {
		obj.LocationOfOrigin = filename
		config.Clusters[key] = obj
	}
	for key, obj := range config.Contexts {
		obj.LocationOfOrigin = filename
		config.Contexts[key] = obj
	}

	if config.AuthInfos == nil {
		config.AuthInfos = map[string]*clientcmdapi.AuthInfo{}
	}
	if config.Clusters == nil {
		config.Clusters = map[string]*clientcmdapi.Cluster{}
	}
	if config.Contexts == nil {
		config.Contexts = map[string]*clientcmdapi.Context{}
	}

	trcKubeDeploymentConfig.ApiConfig[filename] = config
	return config, nil
}

func LoadFromKube(kubeConfigBytes []byte, config *coreconfig.CoreConfig) (*TrcKubeConfig, error) {
	// kubeConfig, err := clientcmd.Load(kubeConfigBytes)
	// if err != nil {
	// 	return nil, err
	// }

	// restConfig, restErr := clientcmd.RESTConfigFromKubeConfig(kubeConfigBytes)
	// if restErr != nil {
	// 	eUtils.LogErrorObject(config, restErr, false)
	// }

	// // set LocationOfOrigin on every Cluster, User, and Context
	// for key, obj := range kubeConfig.AuthInfos {
	// 	obj.LocationOfOrigin = ".kube/config"
	// 	kubeConfig.AuthInfos[key] = obj
	// }
	// for key, obj := range kubeConfig.Clusters {
	// 	obj.LocationOfOrigin = ".kube/config"
	// 	kubeConfig.Clusters[key] = obj
	// }
	// for key, obj := range kubeConfig.Contexts {
	// 	obj.LocationOfOrigin = ".kube/config"
	// 	kubeConfig.Contexts[key] = obj
	// }

	// if kubeConfig.AuthInfos == nil {
	// 	kubeConfig.AuthInfos = map[string]*clientcmdapi.AuthInfo{}
	// }
	// if kubeConfig.Clusters == nil {
	// 	kubeConfig.Clusters = map[string]*clientcmdapi.Cluster{}
	// }
	// if kubeConfig.Contexts == nil {
	// 	kubeConfig.Contexts = map[string]*clientcmdapi.Context{}
	// }

	trcConfig := &TrcKubeConfig{KubeConfigBytes: kubeConfigBytes, ApiConfig: map[string]*clientcmdapi.Config{}}

	return trcConfig, nil
}

func InitTrcKubeConfig(trcshConfig *capauth.TrcShConfig, config *coreconfig.CoreConfig) (*TrcKubeConfig, error) {
	if trcshConfig == nil || trcshConfig.KubeConfigPtr == nil {
		if config != nil && config.Log != nil {
			config.Log.Println("Missing in-memory kubeconfig configuration.")
		}
		return nil, fmt.Errorf("missing in-memory kubeconfig configuration")
	}
	if len(*trcshConfig.KubeConfigPtr) == 0 {
		if config != nil && config.Log != nil {
			config.Log.Println("Empty in-memory kubeconfig configuration.")
		}
		return nil, fmt.Errorf("empty in-memory kubeconfig configuration")
	}

	kubeConfigBytes, decodeErr := base64.StdEncoding.DecodeString(*trcshConfig.KubeConfigPtr)
	if decodeErr != nil {
		if config != nil && config.Log != nil {
			config.Log.Println("Invalid in-memory kubeconfig encoding.")
		}
		return nil, fmt.Errorf("invalid in-memory kubeconfig encoding")
	}

	return LoadFromKube(kubeConfigBytes, config)
}

func newTierceronIOStreams(trcKubeDeploymentConfig *TrcKubeConfig, driverConfig *config.DriverConfig) (genericclioptions.IOStreams, error) {
	iostreams := genericclioptions.IOStreams{In: os.Stdin, Out: io.Discard, ErrOut: io.Discard}
	if trcKubeDeploymentConfig == nil || trcKubeDeploymentConfig.PipeOS == nil {
		return iostreams, nil
	}

	stat, err := driverConfig.MemFs.Stat(trcKubeDeploymentConfig.PipeOS.Name())
	if err != nil {
		if errors, ok := err.(*fs.PathError); ok && errors != nil {
			return iostreams, nil
		}
		return iostreams, err
	}

	if stat.Size() > 0 {
		pipeName := trcKubeDeploymentConfig.PipeOS.Name()
		trcKubeDeploymentConfig.PipeOS.Close()
		pipeReader, err := driverConfig.MemFs.Open(pipeName)
		if err != nil {
			return iostreams, err
		}
		trcKubeDeploymentConfig.PipeOS = pipeReader
		iostreams.In = trcKubeDeploymentConfig.PipeOS
		return iostreams, nil
	}

	iostreams.Out = trcKubeDeploymentConfig.PipeOS
	iostreams.ErrOut = trcKubeDeploymentConfig.PipeOS
	return iostreams, nil
}

func ParseTrcKubeContext(trcKubeContext *TrcKubeContext, deployArgs []string) *TrcKubeContext {
	if trcKubeContext == nil {
		trcKubeContext = &TrcKubeContext{}
	}

	for i := range deployArgs {
		if deployArgs[i] == "set-context" {
			if i+1 < len(deployArgs) {
				trcKubeContext.Context = deployArgs[i+1]
			}
		} else {
			argsSlice := strings.Split(deployArgs[i], "=")
			switch argsSlice[0] {
			case "--cluster":
				trcKubeContext.Cluster = argsSlice[1]
			case "--user":
				trcKubeContext.User = argsSlice[1]
			case "--namespace":
				trcKubeContext.Namespace = argsSlice[1]
			}
		}
	}

	return trcKubeContext
}

func ParseTrcKubeDeployDirective(trcKubeDirective *TrcKubeDirective, deployArgs []string) *TrcKubeDirective {
	if trcKubeDirective == nil {
		trcKubeDirective = &TrcKubeDirective{}
	} else {
		trcKubeDirective.Action = ""
		trcKubeDirective.FromFilePath = ""
		trcKubeDirective.Name = ""
		trcKubeDirective.Object = ""
		trcKubeDirective.Type = ""
		trcKubeDirective.DryRun = false
	}
	trcKubeDirective.Action = deployArgs[0]
	deployArgs = deployArgs[1:]

	for i := range deployArgs {
		if trcKubeDirective.Action == "create" && (deployArgs[i] == "secret" || deployArgs[i] == "configmap") {
			trcKubeDirective.Object = deployArgs[i]
			if i+1 < len(deployArgs) {
				if deployArgs[i] == "secret" {
					trcKubeDirective.Type = deployArgs[i+1]
					trcKubeDirective.Name = deployArgs[i+2]
				} else if deployArgs[i] == "configmap" {
					trcKubeDirective.Name = deployArgs[i+1]
				}
			}
		} else {
			argsSlice := strings.Split(deployArgs[i], "=")
			switch argsSlice[0] {
			case "--from-file":
				if len(argsSlice) > 1 {
					trcKubeDirective.FromFilePath = argsSlice[1]
				}
			case "--dry-run":
				trcKubeDirective.DryRun = true
			case "-f": // From apply...
				if len(deployArgs) > i {
					trcKubeDirective.FromFilePath = deployArgs[i+1]
				}
			}
		}
	}

	return trcKubeDirective
}

func KubeCtl(trcKubeDeploymentConfig *TrcKubeConfig, driverConfig *config.DriverConfig) error {
	configFlags := genericclioptions.NewConfigFlags(true).
		WithDeprecatedPasswordFlag().
		WithDiscoveryBurst(300).
		WithDiscoveryQPS(50.0)

	configFlags.KubeConfigLoader = func(filename string) (*clientcmdapi.Config, error) {
		return loadInMemoryKubeConfig(trcKubeDeploymentConfig, filename)
	}
	iostreams, err := newTierceronIOStreams(trcKubeDeploymentConfig, driverConfig)
	if err != nil {
		return err
	}

	configFlags.PathVisitorLoader = func() resource.PathVisitor {
		return &path.MemPathVisitor{MemFs: driverConfig.MemFs, Iostreams: iostreams}
	}

	configFlags.HandleSecretFromFileSources = func(secret *corev1.Secret, fileSources []string) error {
		for _, fileSource := range fileSources {
			keyName, filePath, err := kubectlutil.ParseFileSource(fileSource)
			if err != nil {
				return err
			}
			var data []byte

			var memFile trcshio.TrcshReadWriteCloser
			var memFileErr error

			if memFile, memFileErr = driverConfig.MemFs.Open(filePath); memFileErr == nil {
				buf := bytes.NewBuffer(nil)
				io.Copy(buf, memFile) // Error handling elided for brevity.
				data = buf.Bytes()
			} else {
				return fmt.Errorf("Error could not find %s for deployment instructions", fileSource)
			}

			if errs := validation.IsConfigMapKey(keyName); len(errs) != 0 {
				return fmt.Errorf("%q is not valid key name for a Secret %s", keyName, strings.Join(errs, ";"))
			}
			if _, entryExists := secret.Data[keyName]; entryExists {
				return fmt.Errorf("cannot add key %s, another key by that name already exists", keyName)
			}
			secret.Data[keyName] = data
		}
		return nil
	}

	configFlags.HandleConfigMapFromFileSources = func(configMap *corev1.ConfigMap, fileSources []string) error {
		for _, fileSource := range fileSources {
			keyName, filePath, err := kubectlutil.ParseFileSource(fileSource)
			if err != nil {
				return err
			}
			var data []byte

			var memFile trcshio.TrcshReadWriteCloser
			var memFileErr error

			if memFile, memFileErr = driverConfig.MemFs.Open(fileSource); memFileErr == nil {
				buf := bytes.NewBuffer(nil)
				io.Copy(buf, memFile) // Error handling elided for brevity.
				data = buf.Bytes()
			} else {
				return fmt.Errorf("Error could not find %s for deployment instructions", fileSource)
			}

			if err := trccreate.HandleConfigMapFromFileSource(configMap, keyName, filePath, data); err != nil {
				return err
			}
		}
		return nil
	}

	configFlags.HandleConfigMapFromEnvFileSources = func(configMap *corev1.ConfigMap, fileSources []string) error {
		for _, fileSource := range fileSources {
			var data []byte

			var memFile trcshio.TrcshReadWriteCloser
			var memFileErr error

			if memFile, memFileErr = driverConfig.MemFs.Open(fileSource); memFileErr == nil {
				buf := bytes.NewBuffer(nil)
				io.Copy(buf, memFile) // Error handling elided for brevity.
				data = buf.Bytes()
			} else {
				return fmt.Errorf("Error could not find %s for deployment instructions", fileSource)
			}

			err := trccreate.HandleConfigMapFromEnvFileSource(configMap, fileSource, data)
			if err != nil {
				return err
			}
		}
		return nil
	}

	command := cmd.NewDefaultKubectlCommandWithArgs(cmd.KubectlOptions{
		PluginHandler: cmd.NewDefaultPluginHandler(plugin.ValidPluginFilenamePrefixes),
		Arguments:     os.Args,
		ConfigFlags:   configFlags,
		IOStreams:     iostreams,
	})
	command.SilenceErrors = true
	command.SilenceUsage = true

	if err := command.Execute(); err != nil {
		return err
	}

	return nil
}

// KubeApply applies an in memory yaml file to a kubernetes cluster
func KubeApply(trcKubeDeploymentConfig *TrcKubeConfig, driverConfig *config.DriverConfig) error {
	configFlags := genericclioptions.
		NewConfigFlags(true).
		WithDeprecatedPasswordFlag()
	configFlags.KubeConfigLoader = func(filename string) (*clientcmdapi.Config, error) {
		return loadInMemoryKubeConfig(trcKubeDeploymentConfig, filename)
	}

	ioStreams, err := newTierceronIOStreams(trcKubeDeploymentConfig, driverConfig)
	if err != nil {
		return err
	}
	configFlags.PathVisitorLoader = func() resource.PathVisitor {
		return &path.MemPathVisitor{MemFs: driverConfig.MemFs, Iostreams: ioStreams}
	}

	f := cmdutil.NewFactory(cmdutil.NewMatchVersionFlags(configFlags))

	flags := apply.NewApplyFlags(ioStreams)
	fileNamesPtr := flags.DeleteFlags.FileNameFlags.Filenames
	*fileNamesPtr = append(*fileNamesPtr, trcKubeDeploymentConfig.KubeDirective.FromFilePath)

	o, err := flags.ToOptions(f, apply.NewCmdApply("kubectl", f, ioStreams), "kubectl", []string{})
	if err != nil {
		return err
	}
	if err := o.Run(); err != nil {
		return err
	}

	return nil
}
