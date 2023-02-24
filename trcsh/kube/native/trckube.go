package native

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/kubectl/pkg/cmd/apply"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util"

	"github.com/trimble-oss/tierceron/trcsh/trcshauth"
	eUtils "github.com/trimble-oss/tierceron/utils"
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

	return &TrcKubeConfig{RestConfig: restConfig, ApiConfig: kubeConfig}, nil
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
				if len(argsSlice) > 1 {
					trcKubeDirective.FromFilePath = argsSlice[1]
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

func KubeApply(trcKubeDeploymentConfig *TrcKubeConfig, config *eUtils.DriverConfig) error {
	memBuilder := &MemBuilder{}
	ioStreams := genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}
	o := apply.NewApplyOptions(ioStreams)

	memBuilder.
		Unstructured()

	memBuilder.
		MemSchema(o.Validator).
		ContinueOnError().
		NamespaceParam(o.Namespace).
		DefaultNamespace()

	memBuilder.MemFilenameParam(false, &o.DeleteOptions.FilenameOptions)

	r := memBuilder.LabelSelectorParam("").
		Flatten().
		Do()

	infos, err := r.Infos()
	if err != nil {
		return err
	}

	for _, info := range infos {
		if len(trcKubeDeploymentConfig.KubeDirective.Name) == 0 {
			metadata, _ := meta.Accessor(trcKubeDeploymentConfig.KubeDirective.Object)
			generatedName := metadata.GetGenerateName()
			if len(generatedName) > 0 {
				fmt.Errorf("from %s: cannot use generate name with apply", generatedName)
			}
		}

		helper := resource.NewHelper(info.Client, info.Mapping).
			DryRun(false).
			WithFieldManager(o.FieldManager)

		// Get the modified configuration of the object. Embed the result
		// as an annotation in the modified configuration, so that it will appear
		// in the patch sent to the server.
		modified, err := util.GetModifiedConfiguration(info.Object, true, unstructured.UnstructuredJSONScheme)
		if err != nil {
			cmdutil.AddSourceToErr(fmt.Sprintf("retrieving modified configuration from:\n%s\nfor:", info.String()), info.Source, err)
		}

		if err := info.Get(); err != nil {
			if !errors.IsNotFound(err) {
				cmdutil.AddSourceToErr(fmt.Sprintf("retrieving current configuration of:\n%s\nfrom server for:", info.String()), info.Source, err)
			}

			// Create the resource if it doesn't exist
			// First, update the annotation used by kubectl apply
			if err := util.CreateApplyAnnotation(info.Object, unstructured.UnstructuredJSONScheme); err != nil {
				cmdutil.AddSourceToErr("creating", info.Source, err)
			}

			// Then create the resource and skip the three-way merge
			obj, err := helper.Create(info.Namespace, true, info.Object)
			if err != nil {
				cmdutil.AddSourceToErr("creating", info.Source, err)
			}
			info.Refresh(obj, true)

		}

		metadata, _ := meta.Accessor(info.Object)
		annotationMap := metadata.GetAnnotations()
		if _, ok := annotationMap[corev1.LastAppliedConfigAnnotation]; !ok {
			fmt.Fprintf(os.Stdout, warningNoLastAppliedConfigAnnotation, info.ObjectName(), corev1.LastAppliedConfigAnnotation, "kubectl")
		}

		patcher, err := newPatcher(o, info, helper)
		if err != nil {
			return err
		}
		patchBytes, patchedObject, err := patcher.Patch(info.Object, modified, info.Source, info.Namespace, info.Name, os.Stderr)
		if err != nil {
			return cmdutil.AddSourceToErr(fmt.Sprintf("applying patch:\n%s\nto:\n%v\nfor:", patchBytes, info), info.Source, err)
		}

		info.Refresh(patchedObject, true)
	}

	return nil
}

// func main() {
// 	var ns string
// 	flag.StringVar(&ns, "namespace", "", "namespace")

// 	// Bootstrap k8s configuration from local 	Kubernetes config file
// 	kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
// 	log.Println("Using kubeconfig file: ", kubeconfig)
// 	// config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
// 	// if err != nil {
// 	// 	log.Fatal(err)
// 	// }

// 	// //
// 	// // This actually loades the context....
// 	// //
// 	// configFromMem, configLoadErr := clientcmd.Load([]byte{})

// 	// if configLoadErr != nil {
// 	// 	log.Fatal(configLoadErr)
// 	// }

// 	// spew.Dump(configFromMem)
// 	// set-context -- Just pick one of the contexts in .Contexts using --cluser, --user, and --namespace to guide.
// 	// configFromMem.Contexts

// 	//	cmdContext := clientcmdapi.NewContext()
// 	//	cmdContext.Cluster = "$ARN"
// 	//	cmdContext.AuthInfo = "$ARN"     // --User
// 	//	cmdContext.Namespace = "$KUBENV" // --namespace
// 	//	clientcmd.Write(cmdContext)

// 	// Create an rest client not targeting specific API version
// 	clientset, err := kubernetes.NewForConfig(config)
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	//	clientset.CoreV1().Secrets().Create()

// 	//	clientset.CoreV1().ConfigMaps().Create()
// 	/*
// 		newConfigMap := clientset.CoreV1().ConfigMaps().Create()
// 		clientset.CoreV1().Apply(newConfigMap)

// 		pods, err := clientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
// 		if err != nil {
// 			log.Fatalln("failed to get pods:", err)
// 		}

// 		// print pods
// 		for i, pod := range pods.Items {
// 			fmt.Printf("[%d] %s\n", i, pod.GetName())
// 		}
// 	*/
// }
