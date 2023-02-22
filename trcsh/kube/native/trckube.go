package kube

import (
	"encoding/base64"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog/v2"

	eUtils "github.com/trimble-oss/tierceron/utils"
)

type TrcKubeConfig struct {
	kubeConfig *rest.Config
	apiConfig  *clientcmdapi.Config
}

func LoadFromKube(kubeconfigBytes []byte) (*clientcmdapi.Config, error) {
	config, err := clientcmd.Load(kubeconfigBytes)
	if err != nil {
		return nil, err
	}
	klog.V(6).Infoln("Config loaded from file: ", ".kube/config")

	// set LocationOfOrigin on every Cluster, User, and Context
	for key, obj := range config.AuthInfos {
		obj.LocationOfOrigin = ".kube/config"
		config.AuthInfos[key] = obj
	}
	for key, obj := range config.Clusters {
		obj.LocationOfOrigin = ".kube/config"
		config.Clusters[key] = obj
	}
	for key, obj := range config.Contexts {
		obj.LocationOfOrigin = ".kube/config"
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

func InitKubeConfig(data string, config *eUtils.DriverConfig) (*clientcmdapi.Config, error) {
	kubeConfigBytes, decodeErr := base64.StdEncoding.DecodeString(data)
	if decodeErr != nil {
		eUtils.LogErrorObject(config, decodeErr, false)
	}

	return LoadFromKube(kubeConfigBytes)
}

func CreateSecret(kubeConfig *rest.Config, config *eUtils.DriverConfig) {
	// clientset, err := kubernetes.NewForConfig(kubeConfig)
	// if err != nil {
	// 	eUtils.LogErrorObject(config, err, false)
	// }
	//	clientset.CoreV1().Secrets().Create()
}

func CreateConfigMap(kubeConfig *rest.Config, config *eUtils.DriverConfig) {

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
