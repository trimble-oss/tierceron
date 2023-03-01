package native

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog/v2"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/kubectl/pkg/cmd/apply"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/trimble-oss/tierceron/trcsh/kube/native/path"
	"github.com/trimble-oss/tierceron/trcsh/trcshauth"
	eUtils "github.com/trimble-oss/tierceron/utils"
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
	ApiConfig  *clientcmdapi.Config

	// config context stuff..
	KubeContext *TrcKubeContext
	// Current kubectl directive... configmap, secret, apply, etc...
	KubeDirective *TrcKubeDirective
}

func LoadFromKube(kubeConfigBytes []byte, config *eUtils.DriverConfig) (*TrcKubeConfig, error) {
	kubeConfig, err := clientcmd.Load(kubeConfigBytes)
	if err != nil {
		return nil, err
	}

	restConfig, restErr := clientcmd.RESTConfigFromKubeConfig(kubeConfigBytes)
	if restErr != nil {
		eUtils.LogErrorObject(config, restErr, false)
	}

	// set LocationOfOrigin on every Cluster, User, and Context
	for key, obj := range kubeConfig.AuthInfos {
		obj.LocationOfOrigin = ".kube/config"
		kubeConfig.AuthInfos[key] = obj
	}
	for key, obj := range kubeConfig.Clusters {
		obj.LocationOfOrigin = ".kube/config"
		kubeConfig.Clusters[key] = obj
	}
	for key, obj := range kubeConfig.Contexts {
		obj.LocationOfOrigin = ".kube/config"
		kubeConfig.Contexts[key] = obj
	}

	if kubeConfig.AuthInfos == nil {
		kubeConfig.AuthInfos = map[string]*clientcmdapi.AuthInfo{}
	}
	if kubeConfig.Clusters == nil {
		kubeConfig.Clusters = map[string]*clientcmdapi.Cluster{}
	}
	if kubeConfig.Contexts == nil {
		kubeConfig.Contexts = map[string]*clientcmdapi.Context{}
	}

	return &TrcKubeConfig{KubeConfigBytes: kubeConfigBytes, RestConfig: restConfig, ApiConfig: kubeConfig}, nil
}

func InitTrcKubeConfig(trcshConfig *trcshauth.TrcShConfig, config *eUtils.DriverConfig) (*TrcKubeConfig, error) {
	kubeConfigBytes, decodeErr := base64.StdEncoding.DecodeString(trcshConfig.KubeConfig)
	if decodeErr != nil {
		eUtils.LogErrorObject(config, decodeErr, false)
	}

	return LoadFromKube(kubeConfigBytes, config)
}

func ParseTrcKubeContext(trcKubeContext *TrcKubeContext, deployArgs []string) *TrcKubeContext {
	if trcKubeContext == nil {
		trcKubeContext = &TrcKubeContext{}
	}

	for i, _ := range deployArgs {
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

	for i, _ := range deployArgs {
		if trcKubeDirective.Action == "create" && (deployArgs[i] == "secret" || deployArgs[i] == "configmap") {
			trcKubeDirective.Object = deployArgs[i]
			if i+1 < len(deployArgs) {
				if deployArgs[i] == "secret" {
					trcKubeDirective.Type = deployArgs[i+1]
					trcKubeDirective.Name = deployArgs[i+2]
				} else if deployArgs[i] == "configmap" {
					trcKubeDirective.Name = deployArgs[i+1]
				} else {
					fmt.Println("Unsupported element: " + deployArgs[i])
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

func CreateKubeResource(trcKubeDeploymentConfig *TrcKubeConfig, config *eUtils.DriverConfig) {
	clientset, err := corev1client.NewForConfig(trcKubeDeploymentConfig.RestConfig)
	if err != nil {
		eUtils.LogErrorObject(config, err, false)
		return
	}

	switch trcKubeDeploymentConfig.KubeDirective.Object {
	case "secret":
		var secretType corev1.SecretType
		if trcKubeDeploymentConfig.KubeDirective.Type == "generic" {
			secretType = corev1.SecretType("")
		} else {
			fmt.Println("Unsupported secret type: " + trcKubeDeploymentConfig.KubeDirective.Type)
		}

		secret := &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      trcKubeDeploymentConfig.KubeDirective.Name, // vault-cert
				Namespace: "",                                         // I think it can always be blank...
			},
			Type: secretType,
			Data: map[string][]byte{},
		}

		keyParts := strings.Split(trcKubeDeploymentConfig.KubeDirective.FromFilePath, "/")
		keyName := keyParts[len(keyParts)-1]

		if errs := validation.IsConfigMapKey(keyName); len(errs) != 0 {
			eUtils.LogErrorObject(config, fmt.Errorf("%q invalid keyname having errors %s", keyName, strings.Join(errs, ";")), false)
			return
		} else {
			if secretData, secretDataOk := config.MemCache[trcKubeDeploymentConfig.KubeDirective.FromFilePath]; secretDataOk {
				secret.Data[keyName] = secretData.Bytes()
			} else if secretData, secretDataOk := config.MemCache["./"+trcKubeDeploymentConfig.KubeDirective.FromFilePath]; secretDataOk {
				secret.Data[keyName] = secretData.Bytes()
			}
		}

		switch trcKubeDeploymentConfig.KubeDirective.Action {
		case "create":
			createOptions := metav1.CreateOptions{}
			createOptions.FieldManager = "kubectl-create" //
			fmt.Println(clientset)
			//			clientset.Secrets(trcKubeDeploymentConfig.KubeContext.Namespace).Create(context.TODO(), secret, createOptions)
		case "update":
			updateOptions := metav1.UpdateOptions{}
			updateOptions.FieldManager = "" //
			fmt.Println(clientset)
			//			clientset.Secrets(trcKubeDeploymentConfig.KubeContext.Namespace).Update(context.TODO(), secret, updateOptions)
		}
	case "configmap":
		configMap := &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      trcKubeDeploymentConfig.KubeDirective.Name, // vault-cert
				Namespace: "",                                         // I think it can always be blank...
			},
			Data: map[string]string{},
		}

		keyParts := strings.Split(trcKubeDeploymentConfig.KubeDirective.FromFilePath, "/")
		keyName := keyParts[len(keyParts)-1]

		if errs := validation.IsConfigMapKey(keyName); len(errs) != 0 {
			eUtils.LogErrorObject(config, fmt.Errorf("%q invalid keyname having errors %s", keyName, strings.Join(errs, ";")), false)
			return
		} else {
			if configData, configDataOk := config.MemCache[trcKubeDeploymentConfig.KubeDirective.FromFilePath]; configDataOk {
				configMap.Data[keyName] = string(configData.Bytes())
			} else if configData, configDataOk := config.MemCache["./"+trcKubeDeploymentConfig.KubeDirective.FromFilePath]; configDataOk {
				configMap.Data[keyName] = string(configData.Bytes())
			}
		}

		switch trcKubeDeploymentConfig.KubeDirective.Action {
		case "create":
			createOptions := metav1.CreateOptions{}
			createOptions.FieldManager = "" //
			fmt.Println(clientset)
			//			clientset.ConfigMaps(trcKubeDeploymentConfig.KubeContext.Namespace).Create(context.TODO(), configMap, createOptions)
		case "update":
			updateOptions := metav1.UpdateOptions{}
			updateOptions.FieldManager = "" //
			fmt.Println(clientset)
			//			clientset.ConfigMaps(trcKubeDeploymentConfig.KubeContext.Namespace).Update(context.TODO(), configMap, updateOptions)
		}
	}
}

// KubeApply applies an in memory yaml file to a kubernetes cluster
func KubeApply(trcKubeDeploymentConfig *TrcKubeConfig, config *eUtils.DriverConfig) error {
	configFlags := genericclioptions.
		NewConfigFlags(true).
		WithDeprecatedPasswordFlag()
	configFlags.KubeConfigLoader = func(filename string) (*clientcmdapi.Config, error) {
		var kubeconfigBytes []byte
		var err error
		if len(trcKubeDeploymentConfig.KubeConfigBytes) > 0 {
			kubeconfigBytes = trcKubeDeploymentConfig.KubeConfigBytes
		}
		config, err := clientcmd.Load(kubeconfigBytes)
		if err != nil {
			return nil, err
		}
		klog.V(6).Infoln("Config loaded from file: ", filename)

		// set LocationOfOrigin on every Cluster, User, and Context
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

		return config, nil
	}

	f := cmdutil.NewFactory(
		cmdutil.NewMatchVersionFlags(configFlags),
		&path.MemPathVisitor{MemCache: config.MemCache})

	ioStreams := genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}

	flags := apply.NewApplyFlags(ioStreams)
	fileNamesPtr := flags.DeleteFlags.FileNameFlags.Filenames
	*fileNamesPtr = append(*fileNamesPtr, trcKubeDeploymentConfig.KubeDirective.FromFilePath)

	o, err := flags.ToOptions(f, apply.NewCmdApply("kubectl", f, ioStreams), "kubectl", []string{})
	if err != nil {
		return err
	}
	o.Run()

	return nil
}
