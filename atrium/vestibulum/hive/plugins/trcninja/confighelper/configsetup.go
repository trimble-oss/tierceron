package confighelper

import (
	"flag"
	"sync"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcninja/kafkautil"
)

var properties *map[string]interface{}

// KafkaManager - manager of Kafka
var KafkaManager *kafkautil.KafkaManager

var KafkaCert []byte

var configInit = false

// var config *tccore.CoreConfig
var configLock sync.Mutex

func InitKafkaPropertiesWithConfig(configContext *tccore.ConfigContext,
	kafkaClientCertPath string,
	kafkaShemaClientCertPath string,
) error {
	kafkaInitErr := InitKafkaProperties(configContext, kafkaClientCertPath, kafkaShemaClientCertPath)
	if kafkaInitErr != nil {
		return kafkaInitErr
	}
	return nil
}

// call this from the plugin
func InitKafkaProperties(configContext *tccore.ConfigContext,
	kafkaClientCertPath string,
	kafkaShemaClientCertPath string,
) error {
	// var envContext string
	properties = configContext.Config

	configContext.Log.Printf("Running on environment %s\n", (*configContext.Config)["env"].(string))

	KafkaCert = (*configContext.ConfigCerts)[kafkaClientCertPath]

	KafkaManager = kafkautil.InitKafkaManager(
		(*configContext.ConfigCerts)[kafkaShemaClientCertPath],
		(*configContext.Config)["schemaRegistryUrl"].(string),
		(*configContext.Config)["schemaRegistryUsername"].(string),
		(*configContext.Config)["schemaRegistryPassword"].(string))

	return nil
}

func InitCommon() error {
	// addrPtr := flag.String("addr", "", "API endpoint for the vault")
	// secretIDPtr := flag.String("secretID", "", "Public app role ID")
	// appRoleIDPtr := flag.String("appRoleID", "", "Secret app role ID")
	envPtr := flag.String("env", "dev", "Environment")
	// authEnvPtr := envPtr // Auth env is always same as env...
	// projectPtr := flag.String("project", "ETL", "Seeding vault with a single project")
	// logFilePtr := flag.String("log", "./etlninja.log", "Output path for log file")

	// pingPtr := flag.Bool("ping", false, "Ping vault.")
	// tokenPtr := flag.String("token", "", "Vault access token, only use if in dev mode or reseeding")
	// tokenNamePtr := flag.String("tokenName", "", "Token name used by this vaultconfig to access the vault")
	// isClean := flag.Bool("clean", false, "Clean data associated with tests")
	// skipCertCache := flag.Bool("skipCertCache", false, "Cache our configuration files")

	flag.Parse()

	*envPtr = "QA"

	// eUtils.InitHeadless(true)

	// f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	// if err != nil {
	// 	fmt.Fprintln(os.Stderr, "Log setup failure.")
	// }
	// logger := log.New(f, "[etlninja]", log.LstdFlags)

	// This is part of flow
	// kafkaInitErr := InitKafkaProperties(addrPtr, secretIDPtr, appRoleIDPtr, envPtr, authEnvPtr, projectPtr, logger, pingPtr, tokenPtr, tokenNamePtr, isClean, skipCertCache)
	// if kafkaInitErr != nil {
	// 	return kafkaInitErr
	// }
	return nil
}

// GetProperties -- returns vault configured properties.
func GetProperties() *map[string]interface{} {
	return properties
}

func resolveTokenName(env string) string {
	tokenNamePtr := ""
	switch env {
	case "local":
		tokenNamePtr = "config_token_local"
		break
	case "dev":
		tokenNamePtr = "config_token_dev"
		break
	case "QA":
		tokenNamePtr = "config_token_QA"
		break
	case "RQA":
		tokenNamePtr = "config_token_RQA"
		break
	case "staging":
		tokenNamePtr = "config_token_staging"
		break
	default:
		tokenNamePtr = "config_token_local"
		break
	}
	return tokenNamePtr
}
