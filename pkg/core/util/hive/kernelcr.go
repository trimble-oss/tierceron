package hive

import (
	"errors"
	"fmt"
	"log"
	"os"
	"plugin"
	"reflect"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"

	"github.com/trimble-oss/tierceron-core/v2/core"
	flowcore "github.com/trimble-oss/tierceron/atrium/trcflow/core"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/kernelopts"
	"github.com/trimble-oss/tierceron/buildopts/pluginopts"
	"github.com/trimble-oss/tierceron/pkg/capauth"
	trcvutils "github.com/trimble-oss/tierceron/pkg/core/util"
	certutil "github.com/trimble-oss/tierceron/pkg/core/util/cert"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
	"github.com/trimble-oss/tierceron/pkg/validator"
	"github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
	"github.com/trimble-oss/tierceron/pkg/vaulthelper/system"
)

// var PluginMods map[string]*plugin.Plugin = map[string]*plugin.Plugin{}
var logger *log.Logger
var dfstat *core.TTDINode

var m sync.Mutex

var globalCertCache *map[string]certValue

type certValue struct {
	CertBytes   *[]byte
	CreatedTime interface{}
}

type PluginHandler struct {
	Name          string //service
	State         int    //0 - initialized, 1 - running, 2 - failed
	Id            string //sha256 of plugin or if kernel - kernel id
	ConfigContext *core.ConfigContext
	Services      *map[string]*PluginHandler
	PluginMod     *plugin.Plugin
}

func InitKernel(id string) *PluginHandler {
	pluginMap := make(map[string]*PluginHandler)
	certCache := make(map[string]certValue)
	globalCertCache = &certCache
	return &PluginHandler{
		Name:          "Kernel",
		Id:            id,
		State:         0,
		Services:      &pluginMap,
		ConfigContext: &core.ConfigContext{},
	}
}

func (pH *PluginHandler) DynamicReloader(driverConfig *config.DriverConfig) {
	if pH == nil || pH.Name != "Kernel" {
		driverConfig.CoreConfig.Log.Println("Unsupported handler attempting to start dynamic reloading.")
		return
	}
	var mod *helperkv.Modifier

	for {
		if mod == nil {
			var err error
			driverConfig.CoreConfig.Log.Println("")
			pluginConfig := make(map[string]interface{})
			pluginConfig["vaddress"] = *driverConfig.CoreConfig.VaultAddressPtr
			currentTokenName := fmt.Sprintf("config_token_%s", driverConfig.CoreConfig.EnvBasis)
			pluginConfig["tokenptr"] = driverConfig.CoreConfig.TokenCache.GetToken(currentTokenName)
			pluginConfig["env"] = driverConfig.CoreConfig.EnvBasis

			_, mod, _, err = eUtils.InitVaultModForPlugin(pluginConfig, currentTokenName, driverConfig.CoreConfig.Log)
			if err != nil {
				driverConfig.CoreConfig.Log.Printf("Problem initializing mod: %s\n", err)
			}
		}
		if globalCertCache != nil && mod != nil {
			for k, v := range *globalCertCache {
				certPath := strings.TrimPrefix(k, "Common/")
				certPath = strings.TrimSuffix(certPath, ".crt.mf.tmpl")
				certPath = strings.TrimSuffix(certPath, ".key.mf.tmpl")
				certPath = strings.TrimSuffix(certPath, ".pem.mf.tmpl")
				metadata, err := mod.ReadMetadata(fmt.Sprintf("values/%s", certPath), driverConfig.CoreConfig.Log)
				if err != nil {
					mod.Release()
					mod = nil
					eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
					continue
				}
				if t, ok := metadata["created_time"]; ok {
					if t != v.CreatedTime {
						//validate cert and restart kernel
						configuredCert, err := certutil.LoadCertComponent(driverConfig,
							mod,
							k)

						if err != nil {
							eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
							continue
						}

						var valid bool = false

						if strings.HasSuffix(k, ".crt.mf.tmpl") {
							valid, err = capauth.IsCertValidBySupportedDomains(configuredCert, validator.VerifyCertificate)
							if err != nil {
								eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
							}
						} else {
							valid = true
						}
						if valid {
							for service, servPh := range *pH.Services {
								*servPh.ConfigContext.CmdSenderChan <- core.KernelCmd{
									PluginName: servPh.Name,
									Command:    core.PLUGIN_EVENT_STOP,
								}
								driverConfig.CoreConfig.Log.Printf("Shutting down service: %s\n", service)
							}
							driverConfig.CoreConfig.Log.Println("Shutting down kernel...")
							os.Exit(0)
						} else {
							continue
						}
					}
				}
			}
		}
		time.Sleep(time.Minute)
	}
}

func addToCache(path string, driverConfig *config.DriverConfig, mod *kv.Modifier) (*[]byte, error) {
	// Trim path
	m.Lock()
	defer m.Unlock()
	if v, ok := (*globalCertCache)[path]; ok {
		driverConfig.CoreConfig.WantCerts = false
		return v.CertBytes, nil
	}
	certPath := strings.TrimPrefix(path, "Common/")
	certPath = strings.TrimSuffix(certPath, ".crt.mf.tmpl")
	certPath = strings.TrimSuffix(certPath, ".key.mf.tmpl")
	certPath = strings.TrimSuffix(certPath, ".pem.mf.tmpl")
	metadata, err := mod.ReadMetadata(fmt.Sprintf("values/%s", certPath), driverConfig.CoreConfig.Log)
	if err != nil {
		eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
		return nil, err
	}
	if t, ok := metadata["created_time"]; ok {
		configuredCert, err := certutil.LoadCertComponent(driverConfig,
			mod,
			path)
		if err != nil {
			eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
			return nil, err
		}
		var valid bool = false

		if strings.HasSuffix(path, ".crt.mf.tmpl") {
			valid, err = capauth.IsCertValidBySupportedDomains(configuredCert, validator.VerifyCertificate)
			if err != nil {
				eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
				return nil, err
			}
		} else {
			valid = true
		}

		if valid {
			(*globalCertCache)[path] = certValue{
				CreatedTime: t,
				CertBytes:   &configuredCert,
			}
			driverConfig.CoreConfig.WantCerts = false

			return &configuredCert, nil
		} else {
			driverConfig.CoreConfig.Log.Println("Invalid cert")
			return nil, errors.New("invalid cert")
		}
	}
	driverConfig.CoreConfig.Log.Println("Unable to access created time for cert.")
	return nil, errors.New("no created time for cert")
}

func (pH *PluginHandler) AddKernelPlugin(service string, driverConfig *config.DriverConfig) {
	if pH == nil || pH.Name != "Kernel" {
		driverConfig.CoreConfig.Log.Println("Unsupported handler attempting to add kernel service.")
		return
	}
	if pH.Services != nil {
		driverConfig.CoreConfig.Log.Printf("Added plugin to kernel: %s\n", service)
		(*pH.Services)[service] = &PluginHandler{
			Name:          service,
			ConfigContext: &core.ConfigContext{},
		}
	}
}

func (pH *PluginHandler) GetPluginHandler(service string, driverConfig *config.DriverConfig) *PluginHandler {
	if pH != nil && pH.Services != nil {
		if plugin, ok := (*pH.Services)[service]; ok {
			return plugin
		} else {
			driverConfig.CoreConfig.Log.Printf("Handler not initialized for plugin to start: %s\n", service)
		}
	} else {
		driverConfig.CoreConfig.Log.Printf("No handlers provided for plugin service to startup: %s\n", service)
	}
	return nil
}

func (pluginHandler *PluginHandler) Init(properties *map[string]interface{}) {
	if pluginHandler.Name == "" {
		logger.Println("No plugin name set for initializing plugin service.")
		return
	}

	if !pluginopts.BuildOptions.IsPluginHardwired() {
		if pluginHandler.PluginMod == nil {
			logger.Println("No plugin module set for initializing plugin service.")
			return
		}
		symbol, err := pluginHandler.PluginMod.Lookup("Init")
		if err != nil {
			logger.Printf("Unable to lookup plugin export: %s\n", err)
		}
		logger.Printf("Initializing plugin module for %s\n", pluginHandler.Name)
		reflect.ValueOf(symbol).Call([]reflect.Value{reflect.ValueOf(pluginHandler.Name), reflect.ValueOf(properties)})
	} else {
		pluginopts.BuildOptions.Init(pluginHandler.Name, properties)
	}
}

func (pluginHandler *PluginHandler) PluginserviceStart(driverConfig *config.DriverConfig, pluginToolConfig map[string]interface{}, chatReceiverChan *chan *core.ChatMsg) {
	if driverConfig.CoreConfig.Log != nil {
		if logger == nil {
			logger = driverConfig.CoreConfig.Log
		}
	} else {
		fmt.Println("No logger passed in to plugin service")
		return
	}
	if kernelopts.BuildOptions.IsKernel() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Recovered with stack trace of" + string(debug.Stack()) + "\n")
			}
		}()
	}
	if pluginHandler.Name == "" {
		driverConfig.CoreConfig.Log.Println("No plugin name specified to start plugin service.")
		return
	}
	if !pluginopts.BuildOptions.IsPluginHardwired() && pluginHandler.PluginMod == nil {
		driverConfig.CoreConfig.Log.Printf("No plugin module initialized to start plugin service: %s\n", pluginHandler.Name)
		return
	}
	var service string
	if s, ok := driverConfig.DeploymentConfig["trcplugin"].(string); ok {
		service = s
	} else {
		driverConfig.CoreConfig.Log.Println("Unable to process plugin service.")
		return
	}
	driverConfig.CoreConfig.Log.Printf("Starting initialization for plugin service: %s Env: %s\n", service, driverConfig.CoreConfig.EnvBasis)
	pluginConfig := make(map[string]interface{})
	pluginConfig["vaddress"] = *driverConfig.CoreConfig.VaultAddressPtr
	currentTokenName := fmt.Sprintf("config_token_%s", driverConfig.CoreConfig.EnvBasis)
	pluginConfig["tokenptr"] = driverConfig.CoreConfig.TokenCache.GetToken(currentTokenName)
	pluginConfig["env"] = driverConfig.CoreConfig.EnvBasis

	_, mod, vault, err := eUtils.InitVaultModForPlugin(pluginConfig, currentTokenName, driverConfig.CoreConfig.Log)
	if err != nil {
		driverConfig.CoreConfig.Log.Printf("Problem initializing mod: %s\n", err)
		return
	}

	if vault != nil {
		defer vault.Close()
	}
	if pluginprojserv, ok := pluginToolConfig["trcprojectservice"]; ok {
		if projserv, k := pluginprojserv.(string); k {
			projServ := strings.Split(projserv, "/")
			if len(projServ) != 2 {
				driverConfig.CoreConfig.Log.Printf("Improper formatting of project/service for %s\n", service)
				return
			}
			properties, err := trcvutils.NewProperties(driverConfig.CoreConfig, vault, mod, mod.Env, projServ[0], projServ[1])
			if err != nil && !strings.Contains(err.Error(), "no data paths found when initing CDS") {
				driverConfig.CoreConfig.Log.Println("Couldn't create properties for regioned certify:" + err.Error())
				return
			}

			var paths []string
			if !pluginopts.BuildOptions.IsPluginHardwired() {
				getConfigPaths, err := pluginHandler.PluginMod.Lookup("GetConfigPaths")
				if err != nil {
					driverConfig.CoreConfig.Log.Printf("Unable to access config for %s\n", service)
					driverConfig.CoreConfig.Log.Printf("Returned with %v\n", err)
					return
				}
				pluginConfigPaths := getConfigPaths.(func(string) []string)
				paths = pluginConfigPaths(pluginHandler.Name)
			} else {
				paths = pluginopts.BuildOptions.GetConfigPaths(pluginHandler.Name)
			}

			serviceConfig := make(map[string]interface{})
			for _, path := range paths {
				if strings.HasPrefix(path, "Common") {
					if v, ok := (*globalCertCache)[path]; ok {
						driverConfig.CoreConfig.WantCerts = false
						serviceConfig[path] = *v.CertBytes
					} else {
						configuredCert, err := addToCache(path, driverConfig, mod)
						if err != nil {
							driverConfig.CoreConfig.Log.Printf("Unable to load cert: %v\n", err)
						} else {
							serviceConfig[path] = *configuredCert
						}
					}
				} else {
					sc, ok := properties.GetConfigValues(projServ[1], path)
					if !ok {
						driverConfig.CoreConfig.Log.Printf("Unable to access configuration data for %s\n", service)
						return
					}
					serviceConfig[path] = &sc
				}
			}
			// Initialize channels
			sender := make(chan core.KernelCmd)
			pluginHandler.ConfigContext.CmdSenderChan = &sender
			msg_sender := make(chan *core.ChatMsg)
			pluginHandler.ConfigContext.ChatSenderChan = &msg_sender

			err_receiver := make(chan error)
			pluginHandler.ConfigContext.ErrorChan = &err_receiver
			ttdi_receiver := make(chan *core.TTDINode)
			pluginHandler.ConfigContext.DfsChan = &ttdi_receiver
			status_receiver := make(chan core.KernelCmd)
			pluginHandler.ConfigContext.CmdReceiverChan = &status_receiver

			if chatReceiverChan == nil {
				driverConfig.CoreConfig.Log.Printf("Unable to access configuration data for %s\n", service)
				return
			}

			chan_map := make(map[string]interface{})

			chan_map[core.PLUGIN_CHANNEL_EVENT_IN] = make(map[string]interface{})
			chan_map[core.PLUGIN_CHANNEL_EVENT_IN].(map[string]interface{})[core.CMD_CHANNEL] = pluginHandler.ConfigContext.CmdSenderChan
			chan_map[core.PLUGIN_CHANNEL_EVENT_IN].(map[string]interface{})[core.CHAT_CHANNEL] = pluginHandler.ConfigContext.ChatSenderChan

			chan_map[core.PLUGIN_CHANNEL_EVENT_OUT] = make(map[string]interface{})
			chan_map[core.PLUGIN_CHANNEL_EVENT_OUT].(map[string]interface{})[core.ERROR_CHANNEL] = pluginHandler.ConfigContext.ErrorChan
			chan_map[core.PLUGIN_CHANNEL_EVENT_OUT].(map[string]interface{})[core.DATA_FLOW_STAT_CHANNEL] = pluginHandler.ConfigContext.DfsChan
			chan_map[core.PLUGIN_CHANNEL_EVENT_OUT].(map[string]interface{})[core.CMD_CHANNEL] = pluginHandler.ConfigContext.CmdReceiverChan
			chan_map[core.PLUGIN_CHANNEL_EVENT_OUT].(map[string]interface{})[core.CHAT_CHANNEL] = chatReceiverChan
			serviceConfig[core.PLUGIN_EVENT_CHANNELS_MAP_KEY] = chan_map
			serviceConfig["log"] = driverConfig.CoreConfig.Log
			serviceConfig["env"] = driverConfig.CoreConfig.Env
			go pluginHandler.handle_errors(driverConfig)
			go pluginHandler.handle_dataflowstat(driverConfig, mod, vault)
			go pluginHandler.receiver(driverConfig)
			pluginHandler.Init(&serviceConfig)
			driverConfig.CoreConfig.Log.Printf("Sending start message to plugin service %s\n", service)
			*pluginHandler.ConfigContext.CmdSenderChan <- core.KernelCmd{
				PluginName: pluginHandler.Name,
				Command:    core.PLUGIN_EVENT_START,
			}
			driverConfig.CoreConfig.Log.Printf("Successfully sent start message to plugin service %s\n", service)
		}
	}
}

func (pluginHandler *PluginHandler) receiver(driverConfig *config.DriverConfig) {
	for {
		event := <-*pluginHandler.ConfigContext.CmdReceiverChan
		switch {
		case event.Command == core.PLUGIN_EVENT_START:
			pluginHandler.State = 1
			driverConfig.CoreConfig.Log.Printf("Kernel finished starting plugin: %s\n", pluginHandler.Name)
		case event.Command == core.PLUGIN_EVENT_STOP:
			driverConfig.CoreConfig.Log.Printf("Kernel finished stopping plugin: %s\n", pluginHandler.Name)
			pluginHandler.State = 0
			*pluginHandler.ConfigContext.ErrorChan <- errors.New(pluginHandler.Name + " shutting down")
			*pluginHandler.ConfigContext.DfsChan <- nil
			pluginHandler.PluginMod = nil
			return
		case event.Command == core.PLUGIN_EVENT_STATUS:
			//TODO
		default:
			//TODO
		}
	}
}

func (pluginHandler *PluginHandler) handle_errors(driverConfig *config.DriverConfig) {
	for {
		result := <-*pluginHandler.ConfigContext.ErrorChan
		switch {
		case result != nil:
			eUtils.LogErrorObject(driverConfig.CoreConfig, result, false)
			return
		}
	}
}

func (pluginHandler *PluginHandler) handle_dataflowstat(driverConfig *config.DriverConfig, mod *kv.Modifier, vault *system.Vault) {
	// tfmContext := &flowcore.TrcFlowMachineContext{
	// 	Env:                       driverConfig.CoreConfig.Env,
	// 	GetAdditionalFlowsByState: flowopts.BuildOptions.GetAdditionalFlowsByState,
	// 	FlowMap:                   map[flowcore.FlowNameType]*flowcore.TrcFlowContext{},
	// }
	// tfContext := &flowcore.TrcFlowContext{
	// 	GoMod:    mod,
	// 	Vault:    vault,
	// 	FlowLock: &sync.Mutex{},
	// }
	for {
		dfstat = <-*pluginHandler.ConfigContext.DfsChan
		switch {
		case dfstat != nil:
			driverConfig.CoreConfig.Log.Printf("Received dataflow statistic: %s\n", dfstat.Name)
			tenantIndexPath, tenantDFSIdPath := coreopts.BuildOptions.GetDFSPathName()
			if len(tenantIndexPath) == 0 || len(tenantDFSIdPath) == 0 {
				driverConfig.CoreConfig.Log.Println("GetDFSPathName returned an empty index path value.")
				return
			}
			flowcore.DeliverStatistic(nil, nil, mod, dfstat, dfstat.Name, tenantIndexPath, tenantDFSIdPath, driverConfig.CoreConfig.Log, true)
			driverConfig.CoreConfig.Log.Printf("Delivered dataflow statistic: %s\n", dfstat.Name)
		case dfstat == nil:
			driverConfig.CoreConfig.Log.Println("Shutting down dataflow statistic receiver in kernel")
			return
		}
	}
}

func (pluginHandler *PluginHandler) PluginserviceStop(driverConfig *config.DriverConfig) {
	if kernelopts.BuildOptions.IsKernel() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Recovered with stack trace of" + string(debug.Stack()) + "\n")
			}
		}()
	}
	pluginName := pluginHandler.Name
	if len(pluginName) == 0 {
		driverConfig.CoreConfig.Log.Printf("No plugin name provided to stop plugin service.\n")
		return
	}
	if pluginHandler.PluginMod == nil {
		driverConfig.CoreConfig.Log.Printf("No plugin mod initialized or set for %s to stop plugin\n", pluginName)
		return
	}
	driverConfig.CoreConfig.Log.Printf("Sending stop message to plugin: %s\n", pluginName)
	*pluginHandler.ConfigContext.CmdSenderChan <- core.KernelCmd{
		pluginName,
		core.PLUGIN_EVENT_STOP,
	}
	driverConfig.CoreConfig.Log.Printf("Stop message successfully sent to plugin: %s\n", pluginName)
}

func LoadPluginPath(driverConfig *config.DriverConfig, pluginToolConfig map[string]interface{}) string {
	var deployroot string
	var service string
	if s, ok := pluginToolConfig["trcplugin"].(string); ok {
		driverConfig.CoreConfig.Log.Printf("Loading plugin path for service: %s\n", s)
		service = s
	} else {
		driverConfig.CoreConfig.Log.Println("Unable to stop plugin service.")
		return ""
	}
	if d, ok := pluginToolConfig["trcdeployroot"].(string); ok {
		driverConfig.CoreConfig.Log.Printf("Loading plugin deploy root for service: %s\n", d)
		deployroot = d
	} else {
		driverConfig.CoreConfig.Log.Println("Unable to stop plugin service.")
		return ""
	}
	pluginPath := fmt.Sprintf("%s/%s%s", deployroot, service, ".so")
	return pluginPath
}

func (pluginHandler *PluginHandler) LoadPluginMod(driverConfig *config.DriverConfig, pluginPath string) {
	driverConfig.CoreConfig.Log.Printf("Loading plugin: %s\n", pluginPath)

	var pluginM *plugin.Plugin
	if !pluginopts.BuildOptions.IsPluginHardwired() {
		pM, err := plugin.Open(pluginPath)
		if err != nil {
			driverConfig.CoreConfig.Log.Printf("Unable to open plugin module for service: %s\n", pluginPath)
			pluginHandler.State = 2
			return
		}
		pluginM = pM
	}
	pluginName := pluginHandler.Name
	if len(pluginName) > 0 {
		driverConfig.CoreConfig.Log.Printf("Successfully opened plugin module for %s\n", pluginName)
		// PluginMods[pluginName] = pluginM
		pluginHandler.PluginMod = pluginM
		pluginHandler.State = 0
	} else {
		driverConfig.CoreConfig.Log.Println("Unable to load plugin module because missing plugin name")
		pluginHandler.State = 2
		return
	}
}

func (pluginHandler *PluginHandler) Handle_Chat(driverConfig *config.DriverConfig) {
	if pluginHandler == nil || (*pluginHandler).Name != "Kernel" || len(*pluginHandler.Services) == 0 {
		driverConfig.CoreConfig.Log.Printf("Chat handling not supported for plugin: %s\n", pluginHandler.Name)
		return
	}
	if pluginHandler.ConfigContext.ChatReceiverChan == nil {
		msg_receiver := make(chan *core.ChatMsg)
		pluginHandler.ConfigContext.ChatReceiverChan = &msg_receiver
		pluginHandler.State = 1
	}
	for {
		msg := <-*pluginHandler.ConfigContext.ChatReceiverChan
		if msg.KernelId == nil || *(msg.KernelId) == "" {
			msg.KernelId = &pluginHandler.Id
		}
		driverConfig.CoreConfig.Log.Println("Kernel received message from chat.")
		if eUtils.RefEquals(msg.Name, "SHUTDOWN") {
			driverConfig.CoreConfig.Log.Println("Shutting down chat receiver.")
			for _, p := range *pluginHandler.Services {
				if p.ConfigContext.ChatSenderChan != nil && *msg.Query != nil && len(*msg.Query) > 0 && (*msg.Query)[0] == p.Name {
					*p.ConfigContext.ChatSenderChan <- &core.ChatMsg{
						Name:     msg.Name,
						KernelId: &pluginHandler.Id,
					}
				}
			}
			return
		}
		for _, q := range *msg.Query {
			driverConfig.CoreConfig.Log.Println("Kernel processing chat query.")
			if plugin, ok := (*pluginHandler.Services)[q]; ok && plugin.State == 1 {
				driverConfig.CoreConfig.Log.Printf("Sending query to service: %s.\n", plugin.Name)
				new_msg := &core.ChatMsg{
					Name:     &q,
					KernelId: &pluginHandler.Id,
					Query:    &[]string{},
				}
				if eUtils.RefLength(msg.Name) > 0 {
					*new_msg.Query = append(*new_msg.Query, *msg.Name)
				} else {
					driverConfig.CoreConfig.Log.Printf("Warning, self identification through Name is required for all messages. Dropping query...\n")
					return
				}
				if eUtils.RefLength(msg.Response) > 0 && eUtils.RefLength((*msg).Response) > 0 {
					new_msg.Response = (*msg).Response
				}
				if eUtils.RefLength(msg.ChatId) > 0 && eUtils.RefLength((*msg).ChatId) > 0 {
					new_msg.ChatId = (*msg).ChatId
				}
				*plugin.ConfigContext.ChatSenderChan <- new_msg
			} else if eUtils.RefLength(msg.Name) > 0 {
				driverConfig.CoreConfig.Log.Printf("Service unavailable to process query from %s\n", *msg.Name)
				if plugin, ok := (*pluginHandler.Services)[*msg.Name]; ok {
					responseError := "Service unavailable"
					msg.Response = &responseError
					*plugin.ConfigContext.ChatSenderChan <- msg //update msg with error response
				}
				continue
			} else {
				driverConfig.CoreConfig.Log.Println("Unable to interpret message.")
			}
		}
	}
}
