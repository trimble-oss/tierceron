package hive

import (
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"os"
	"plugin"
	"reflect"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/trimble-oss/tierceron-core/v2/core"
	flowcore "github.com/trimble-oss/tierceron/atrium/trcflow/core"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/pluginutil"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcdb/opts/prod"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/kernelopts"
	"github.com/trimble-oss/tierceron/buildopts/pluginopts"
	"github.com/trimble-oss/tierceron/pkg/capauth"
	"github.com/trimble-oss/tierceron/pkg/cli/trcconfigbase"
	"github.com/trimble-oss/tierceron/pkg/cli/trcsubbase"
	trcvutils "github.com/trimble-oss/tierceron/pkg/core/util"
	certutil "github.com/trimble-oss/tierceron/pkg/core/util/cert"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
	"github.com/trimble-oss/tierceron/pkg/validator"
	"github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
	"github.com/trimble-oss/tierceron/pkg/vaulthelper/system"
)

// var PluginMods map[string]*plugin.Plugin = map[string]*plugin.Plugin{}
var dfstat *core.TTDINode

var m sync.Mutex

var globalCertCache *cmap.ConcurrentMap[string, *certValue]

var globalPluginStatusChan chan string

type certValue struct {
	CertBytes   *[]byte
	CreatedTime interface{}
	NotAfter    *time.Time
	lastUpdate  *time.Time
}

type PluginHandler struct {
	Name          string //service
	State         int    //0 - initialized, 1 - running, 2 - failed
	Id            string
	Signature     string //sha256 of plugin
	ConfigContext *core.ConfigContext
	Services      *map[string]*PluginHandler
	PluginMod     *plugin.Plugin
	KernelCtx     *KernelCtx
}

type KernelCtx struct {
	DeployRestartChan *chan string
	PluginRestartChan *chan core.KernelCmd
}

func InitKernel(id string) *PluginHandler {
	pluginMap := make(map[string]*PluginHandler)
	certCache := cmap.New[*certValue]()
	globalCertCache = &certCache
	deployRestart := make(chan string)
	pluginRestart := make(chan core.KernelCmd)
	return &PluginHandler{
		Name:          "Kernel",
		Id:            id,
		State:         0,
		Services:      &pluginMap,
		ConfigContext: &core.ConfigContext{},
		KernelCtx: &KernelCtx{
			DeployRestartChan: &deployRestart,
			PluginRestartChan: &pluginRestart,
		},
	}
}

func (pH *PluginHandler) DynamicReloader(driverConfig *config.DriverConfig) {
	if pH == nil || pH.Name != "Kernel" {
		driverConfig.CoreConfig.Log.Println("Unsupported handler attempting to start dynamic reloading.")
		return
	}
	var mod *kv.Modifier
	pHID, err := strconv.Atoi(pH.Id)
	if err != nil {
		driverConfig.CoreConfig.Log.Println("Setting default handler id for dynamic reloading.")
		pHID = 0
	}
	for {
		if mod == nil {
			var err error
			driverConfig.CoreConfig.Log.Println("")
			pluginConfig := make(map[string]interface{})
			pluginConfig["vaddress"] = *driverConfig.CoreConfig.VaultAddressPtr
			currentTokenName := fmt.Sprintf("config_token_%s", driverConfig.CoreConfig.EnvBasis)
			pluginConfig["tokenptr"] = driverConfig.CoreConfig.TokenCache.GetToken(currentTokenName)
			pluginConfig["env"] = driverConfig.CoreConfig.EnvBasis

			_, mod, _, err = eUtils.InitVaultModForPlugin(pluginConfig,
				driverConfig.CoreConfig.TokenCache,
				currentTokenName, driverConfig.CoreConfig.Log)
			if err != nil {
				driverConfig.CoreConfig.Log.Printf("DynamicReloader Problem initializing mod: %s  Trying again later\n", err)
			}
		}
		if globalCertCache != nil && mod != nil {
			for k, v := range globalCertCache.Items() {
				certPath := strings.TrimPrefix(k, "Common/")
				certPath = strings.TrimSuffix(certPath, ".crt.mf.tmpl")
				certPath = strings.TrimSuffix(certPath, ".key.mf.tmpl")
				certPath = strings.TrimSuffix(certPath, ".pem.mf.tmpl")
				metadata, err := mod.ReadMetadata(fmt.Sprintf("values/%s", certPath), driverConfig.CoreConfig.Log)
				if err != nil {
					mod.Release()
					mod = nil
					eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
					goto waitToReload
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
							valid, _, err = capauth.IsCertValidBySupportedDomains(configuredCert, validator.VerifyCertificate)
							if err != nil {
								eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
							}
						} else {
							valid = true
						}
						if valid {
							for s, sPh := range *pH.Services {
								if sPh != nil && sPh.ConfigContext != nil && (*sPh.ConfigContext).CmdSenderChan != nil {
									*sPh.ConfigContext.CmdSenderChan <- core.KernelCmd{
										PluginName: sPh.Name,
										Command:    core.PLUGIN_EVENT_STOP,
									}
									driverConfig.CoreConfig.Log.Printf("Shutting down service: %s\n", s)
								} else {
									driverConfig.CoreConfig.Log.Printf("Service not properly initialized to shut down for cert reloading: %s\n", s)
									goto waitToReload
								}
							}
							//TODO: Get rid of os.Exit
							// 0. Reload certificates
							// 1. Recall Init function for each plugin
							// 2. Start each plugin
							driverConfig.CoreConfig.Log.Println("Shutting down kernel...")
							os.Exit(0)
						} else {
							continue
						}
					}
				} else if v != nil && v.NotAfter != nil && v.lastUpdate != nil && !(*v.NotAfter).IsZero() && globalPluginStatusChan != nil && len(globalPluginStatusChan) == 0 {
					timeDiff := (*v.NotAfter).Sub(time.Now())
					if timeDiff <= 0 && ((*v.lastUpdate).IsZero() || time.Now().Sub(*v.lastUpdate) < time.Hour) {
						response := fmt.Sprintf("Expired cert %s in kernel, shutting down services.", k)
						*pH.ConfigContext.ChatReceiverChan <- &core.ChatMsg{
							Name:        &pH.Name,
							Query:       &[]string{"trcshtalk"},
							IsBroadcast: true,
							Response:    &response,
						}
						tiNow := time.Now()
						v.lastUpdate = &tiNow
						for s, sPh := range *pH.Services {
							if sPh != nil && sPh.ConfigContext != nil && (*sPh.ConfigContext).CmdSenderChan != nil {
								if sPh.Name != "healthcheck" {
									*sPh.ConfigContext.CmdSenderChan <- core.KernelCmd{
										PluginName: sPh.Name,
										Command:    core.PLUGIN_EVENT_STOP,
									}
									driverConfig.CoreConfig.Log.Printf("Shutting down service: %s\n", s)
								}
							} else {
								driverConfig.CoreConfig.Log.Printf("Service not properly initialized to shut down for cert expiration: %s\n", s)
							}
						}
					} else if timeDiff <= time.Hour*24 && ((*v.lastUpdate).IsZero() || time.Now().Sub(*v.lastUpdate) < time.Hour) && pHID == 0 {
						response := fmt.Sprintf("Cert %s expiring in %.2f hours.", k, timeDiff.Hours())
						*pH.ConfigContext.ChatReceiverChan <- &core.ChatMsg{
							Name:        &pH.Name,
							Query:       &[]string{"trcshtalk"},
							IsBroadcast: true,
							Response:    &response,
						}
						tiNow := time.Now()
						v.lastUpdate = &tiNow
					} else if timeDiff <= time.Hour*168 && ((*v.lastUpdate).IsZero() || time.Now().Sub(*v.lastUpdate) < time.Hour*24) && pHID == 0 {
						daysLeft := timeDiff.Hours() / 24.0
						response := fmt.Sprintf("Cert %s expiring in %d days.", k, int(daysLeft))
						*pH.ConfigContext.ChatReceiverChan <- &core.ChatMsg{
							Name:        &pH.Name,
							Query:       &[]string{"trcshtalk"},
							IsBroadcast: true,
							Response:    &response,
						}
						tiNow := time.Now()
						v.lastUpdate = &tiNow
					}
				}
			}
		}
		if pH.KernelCtx != nil &&
			pH.KernelCtx.DeployRestartChan != nil &&
			pH.KernelCtx.PluginRestartChan != nil &&
			mod != nil &&
			!pluginopts.BuildOptions.IsPluginHardwired() {
			for service, servPh := range *pH.Services {
				certifyMap, err := mod.ReadData(fmt.Sprintf("super-secrets/Index/TrcVault/trcplugin/%s/Certify", service))
				if err != nil {
					pH.ConfigContext.Log.Printf("Unable to read certification data for %s %s\n", service, err)
					continue
				}

				if new_sha, ok := certifyMap["trcsha256"]; ok && new_sha.(string) != servPh.Signature && servPh.Signature != "" {
					driverConfig.CoreConfig.Log.Printf("Kernel shutdown, installing new service: %s\n", service)

					if t, ok := certifyMap["trctype"]; ok && t.(string) == "trcshkubeservice" {
						if servPh != nil && servPh.ConfigContext == nil || (*servPh.ConfigContext).CmdSenderChan == nil {
							driverConfig.CoreConfig.Log.Printf("Kube service not properly initialized to shut down: %s\n", service)
							goto waitToReload
						}
						driverConfig.CoreConfig.Log.Printf("Shutting down service: %s\n", service)
						*servPh.ConfigContext.CmdSenderChan <- core.KernelCmd{
							PluginName: servPh.Name,
							Command:    core.PLUGIN_EVENT_STOP,
						}
						cmd := <-*pH.KernelCtx.PluginRestartChan
						if cmd.Command == core.PLUGIN_EVENT_STOP {
							(*pH.Services)[service] = &PluginHandler{
								Name: service,
								ConfigContext: &core.ConfigContext{
									Log: driverConfig.CoreConfig.Log,
								},
								KernelCtx: &KernelCtx{
									PluginRestartChan: pH.KernelCtx.PluginRestartChan,
								},
							}
							driverConfig.CoreConfig.Log.Printf("Restarting service: %s\n", service)
							*pH.KernelCtx.DeployRestartChan <- service
						}
					} else {
						for s, sPh := range *pH.Services {
							if sPh != nil && sPh.ConfigContext != nil && (*sPh.ConfigContext).CmdSenderChan != nil {
								driverConfig.CoreConfig.Log.Printf("Shutting down service: %s\n", s)
								*sPh.ConfigContext.CmdSenderChan <- core.KernelCmd{
									PluginName: sPh.Name,
									Command:    core.PLUGIN_EVENT_STOP,
								}
								cmd := <-*pH.KernelCtx.PluginRestartChan
								if cmd.Command == core.PLUGIN_EVENT_STOP {
									driverConfig.CoreConfig.Log.Printf("Shut down service: %s\n", s)
								}
							} else {
								driverConfig.CoreConfig.Log.Printf("Service not properly initialized to shut down: %s\n", s)
								goto waitToReload
							}
						}
						driverConfig.CoreConfig.Log.Println("Shutting down kernel...")
						os.Exit(0)
					}
				}
			}
		}
	waitToReload:
		time.Sleep(time.Minute)
	}
}

func addToCache(path string, driverConfig *config.DriverConfig, mod *kv.Modifier) (*[]byte, error) {
	// Trim path
	m.Lock()
	defer m.Unlock()
	if v, ok := globalCertCache.Get(path); ok {
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
		var cert *x509.Certificate
		var certNotAfter *time.Time
		if strings.HasSuffix(path, ".crt.mf.tmpl") {
			valid, cert, err = capauth.IsCertValidBySupportedDomains(configuredCert, validator.VerifyCertificate)
			if err != nil || cert == nil {
				eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
				return nil, err
			}
			certNotAfter = &cert.NotAfter
		} else {
			valid = true
			certNotAfter = &time.Time{}
		}

		if valid {
			var zeroTime time.Time
			globalCertCache.Set(path, &certValue{
				CreatedTime: t,
				CertBytes:   &configuredCert,
				NotAfter:    certNotAfter,
				lastUpdate:  &zeroTime,
			})

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
			Name: service,
			ConfigContext: &core.ConfigContext{
				Log: driverConfig.CoreConfig.Log,
			},
			KernelCtx: &KernelCtx{
				PluginRestartChan: pH.KernelCtx.PluginRestartChan,
			},
		}
	}
}

func (pH *PluginHandler) InitPluginStatus(driverConfig *config.DriverConfig) {
	if pH == nil || pH.Name != "Kernel" {
		driverConfig.CoreConfig.Log.Println("Unsupported handler attempting to add kernel service.")
		return
	}
	if pH.Services != nil {
		globalPluginStatusChan = make(chan string, len(*pH.Services))
		for k, _ := range *pH.Services {
			globalPluginStatusChan <- k
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
		pluginHandler.ConfigContext.Log.Println("No plugin name set for initializing plugin service.")
		return
	}

	if !pluginopts.BuildOptions.IsPluginHardwired() && pluginHandler.PluginMod != nil {
		if pluginHandler.PluginMod == nil {
			pluginHandler.ConfigContext.Log.Println("No plugin module set for initializing plugin service.")
			return
		}
		symbol, err := pluginHandler.PluginMod.Lookup("Init")
		if err != nil {
			pluginHandler.ConfigContext.Log.Printf("Unable to lookup plugin export: %s\n", err)
		}
		pluginHandler.ConfigContext.Log.Printf("Initializing plugin module for %s\n", pluginHandler.Name)
		reflect.ValueOf(symbol).Call([]reflect.Value{reflect.ValueOf(pluginHandler.Name), reflect.ValueOf(properties)})
	} else {
		pluginopts.BuildOptions.Init(pluginHandler.Name, properties)
	}
}

func (pluginHandler *PluginHandler) RunPlugin(
	driverConfig *config.DriverConfig,
	service string,
	serviceConfig *map[string]interface{},
	chatReceiverChan *chan *core.ChatMsg,
) {
	// Initialize channels
	sender := make(chan core.KernelCmd)
	pluginHandler.ConfigContext.CmdSenderChan = &sender
	msg_sender := make(chan *core.ChatMsg)
	pluginHandler.ConfigContext.ChatSenderChan = &msg_sender

	broadcastChan := make(chan *core.ChatMsg)
	pluginHandler.ConfigContext.ChatBroadcastChan = &broadcastChan

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

	chan_map[core.CHAT_BROADCAST_CHANNEL] = pluginHandler.ConfigContext.ChatBroadcastChan

	(*serviceConfig)[core.PLUGIN_EVENT_CHANNELS_MAP_KEY] = chan_map
	(*serviceConfig)["log"] = driverConfig.CoreConfig.Log
	(*serviceConfig)["env"] = driverConfig.CoreConfig.Env
	go pluginHandler.handle_errors(driverConfig)
	statPluginConfig := make(map[string]interface{})
	statPluginConfig["vaddress"] = *driverConfig.CoreConfig.VaultAddressPtr
	wantedTokenName := "config_token_pluginany"
	statPluginConfig["env"] = driverConfig.CoreConfig.EnvBasis

	_, kernelmod, kernelvault, err := eUtils.InitVaultModForPlugin(statPluginConfig,
		driverConfig.CoreConfig.TokenCache,
		wantedTokenName,
		driverConfig.CoreConfig.Log)
	if err != nil {
		driverConfig.CoreConfig.Log.Printf("Problem initializing stat mod: %s\n", err)
		return
	}
	if kernelvault != nil {
		defer kernelvault.Close()
	}

	pluginMap := map[string]interface{}{"pluginName": pluginHandler.Name}

	certifyMap, err := pluginutil.GetPluginCertifyMap(kernelmod, pluginMap)
	if err != nil {
		fmt.Printf("Kernel Missing plugin certification: %s.\n", pluginHandler.Name)
		return
	}
	(*serviceConfig)["certify"] = certifyMap

	go pluginHandler.handle_dataflowstat(driverConfig, kernelmod, kernelvault)
	go pluginHandler.receiver(driverConfig)
	pluginHandler.Init(serviceConfig)
	driverConfig.CoreConfig.Log.Printf("Sending start message to plugin service %s\n", service)
	*pluginHandler.ConfigContext.CmdSenderChan <- core.KernelCmd{
		PluginName: pluginHandler.Name,
		Command:    core.PLUGIN_EVENT_START,
	}
	driverConfig.CoreConfig.Log.Printf("Successfully sent start message to plugin service %s\n", service)
}

func (pluginHandler *PluginHandler) PluginserviceStart(driverConfig *config.DriverConfig, pluginToolConfig map[string]interface{}, chatReceiverChan *chan *core.ChatMsg) {
	if driverConfig.CoreConfig.Log != nil {
		if pluginHandler.ConfigContext.Log == nil {
			pluginHandler.ConfigContext.Log = driverConfig.CoreConfig.Log
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
		if s, ok := pluginToolConfig["trctype"].(string); ok && s == "trcshpluginservice" {
			driverConfig.CoreConfig.Log.Printf("No plugin module initialized to start plugin service: %s\n", pluginHandler.Name)
			return
		}
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
	if len(driverConfig.CoreConfig.Regions) > 0 {
		pluginConfig["regions"] = driverConfig.CoreConfig.Regions
	}
	currentTokenName := fmt.Sprintf("config_token_%s", driverConfig.CoreConfig.EnvBasis)
	pluginConfig["env"] = driverConfig.CoreConfig.EnvBasis

	if !pluginopts.BuildOptions.IsPluginHardwired() {
		if pluginHandler.PluginMod != nil {
			set_prod, err := pluginHandler.PluginMod.Lookup("SetProd")
			if err == nil && set_prod != nil {
				driverConfig.CoreConfig.Log.Printf("Setting production environment for %s\n", service)
				setProd := set_prod.(func(bool))
				setProd(prod.IsProd())
			} else {
				driverConfig.CoreConfig.Log.Printf("Setting production environment is not implemented for %s\n", service)
			}
		} else {
			driverConfig.CoreConfig.Log.Printf("Setting production environment is not implemented for %s\n", service)
		}
	}

	_, mod, vault, err := eUtils.InitVaultModForPlugin(pluginConfig,
		driverConfig.CoreConfig.TokenCache,
		currentTokenName,
		driverConfig.CoreConfig.Log)
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
			if !pluginopts.BuildOptions.IsPluginHardwired() && pluginHandler.PluginMod != nil {
				getConfigPaths, err := pluginHandler.PluginMod.Lookup("GetConfigPaths")
				if err != nil {
					driverConfig.CoreConfig.Log.Printf("Unable to access config for %s\n", service)
					driverConfig.CoreConfig.Log.Printf("Returned with %v\n", err)
					return
				}
				pluginConfigPaths := getConfigPaths.(func(string) []string)
				paths = pluginConfigPaths(pluginHandler.Name)
			} else {
				if s, ok := pluginToolConfig["trctype"].(string); pluginopts.BuildOptions.IsPluginHardwired() || (ok && s == "trcshkubeservice") {
					paths = pluginopts.BuildOptions.GetConfigPaths(pluginHandler.Name)
				} else {
					driverConfig.CoreConfig.Log.Printf("Unable to access config for %s\n", service)
					driverConfig.CoreConfig.Log.Printf("Returned with %v\n", err)
					return
				}
			}

			serviceConfig := make(map[string]interface{})
			for _, path := range paths {
				if strings.HasPrefix(path, "Common") {
					if v, ok := globalCertCache.Get(path); ok {
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
					if pluginToolConfig["trctype"] == "trcshkubeservice" {
						envArg := fmt.Sprintf("-env=%s", driverConfig.CoreConfig.EnvBasis)
						restrictedMappingSub := append([]string{"", envArg}, paths[0])
						ctl := "pluginrun"
						flagset := flag.NewFlagSet(ctl, flag.ExitOnError)
						flagset.String("env", "dev", "Environment to configure")

						wantedTokenName := fmt.Sprintf("config_token_%s", eUtils.GetEnvBasis(driverConfig.CoreConfig.Env))
						driverConfig.CoreConfig.CurrentTokenNamePtr = nil
						trcsubbase.CommonMain(&driverConfig.CoreConfig.EnvBasis,
							driverConfig.CoreConfig.VaultAddressPtr,
							&driverConfig.CoreConfig.EnvBasis,
							new(string),
							new(string),
							&wantedTokenName,
							flagset,
							restrictedMappingSub,
							driverConfig)

						driverConfig.EndDir = "."
						restrictedMappingConfig := []string{"", envArg}
						flagset = flag.NewFlagSet(ctl, flag.ExitOnError)
						flagset.String("env", "dev", "Environment to configure")

						// Get certs...
						driverConfig.CoreConfig.WantCerts = true
						trcconfigbase.CommonMain(&driverConfig.CoreConfig.Env,
							driverConfig.CoreConfig.VaultAddressPtr,
							&driverConfig.CoreConfig.Env,
							new(string),      // secretId
							new(string),      // approleId
							&wantedTokenName, // wantedTokenName
							nil,              // regionPtr
							flagset,
							restrictedMappingConfig,
							driverConfig)

						if strings.HasPrefix(paths[0], "-templateFilter=") {
							filter := paths[0][strings.Index(paths[0], "=")+1:]
							filterParts := strings.Split(filter, ",")
							for _, filterPart := range filterParts {
								if !strings.HasPrefix(filterPart, "Common") {
									restrictedMappingConfig = append(restrictedMappingConfig, fmt.Sprintf("-servicesWanted=%s", filterPart))
									break
								}
							}
						}

						driverConfig.CoreConfig.WantCerts = false
						flagset = flag.NewFlagSet(ctl, flag.ExitOnError)
						flagset.String("env", "dev", "Environment to configure")
						trcconfigbase.CommonMain(&driverConfig.CoreConfig.Env,
							driverConfig.CoreConfig.VaultAddressPtr,
							&driverConfig.CoreConfig.Env,
							new(string),      // secretId
							new(string),      // approleId
							&wantedTokenName, // tokenName
							nil,              // regionPtr
							flagset,
							restrictedMappingConfig,
							driverConfig)

						driverConfig.MemFs.ClearCache("./trc_templates")
						driverConfig.MemFs.ClearCache("./deploy")
						driverConfig.MemFs.SerializeToMap(".", serviceConfig)
						driverConfig.IsShellSubProcess = true
					} else {
						sc, ok := properties.GetRegionConfigValues(projServ[1], path)
						if !ok {
							driverConfig.CoreConfig.Log.Printf("Unable to access configuration data for %s\n", service)
							return
						}
						serviceConfig[path] = &sc
					}
				}
			}
			// Initialize channels
			sender := make(chan core.KernelCmd)
			pluginHandler.ConfigContext.CmdSenderChan = &sender
			msg_sender := make(chan *core.ChatMsg)
			pluginHandler.ConfigContext.ChatSenderChan = &msg_sender

			broadcastChan := make(chan *core.ChatMsg)
			pluginHandler.ConfigContext.ChatBroadcastChan = &broadcastChan

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

			chan_map[core.CHAT_BROADCAST_CHANNEL] = pluginHandler.ConfigContext.ChatBroadcastChan

			serviceConfig[core.PLUGIN_EVENT_CHANNELS_MAP_KEY] = chan_map
			serviceConfig["log"] = driverConfig.CoreConfig.Log
			serviceConfig["env"] = driverConfig.CoreConfig.Env
			go pluginHandler.handle_errors(driverConfig)
			statPluginConfig := make(map[string]interface{})
			statPluginConfig["vaddress"] = *driverConfig.CoreConfig.VaultAddressPtr
			currentStatTokenName := "config_token_pluginany"
			statPluginConfig["env"] = driverConfig.CoreConfig.EnvBasis

			_, kernelmod, kernelvault, err := eUtils.InitVaultModForPlugin(statPluginConfig,
				driverConfig.CoreConfig.TokenCache,
				currentStatTokenName,
				driverConfig.CoreConfig.Log)
			if err != nil {
				driverConfig.CoreConfig.Log.Printf("Problem initializing stat mod: %s\n", err)
				return
			}
			if kernelvault != nil {
				defer kernelvault.Close()
			}

			pluginMap := map[string]interface{}{"pluginName": pluginHandler.Name}

			certifyMap, err := pluginutil.GetPluginCertifyMap(kernelmod, pluginMap)
			if err != nil {
				fmt.Printf("Kernel Missing plugin certification: %s.\n", pluginHandler.Name)
				return
			}
			(serviceConfig)["certify"] = certifyMap

			go pluginHandler.handle_dataflowstat(driverConfig, kernelmod, kernelvault)
			go pluginHandler.receiver(driverConfig)
			pluginHandler.Init(&serviceConfig)
			driverConfig.CoreConfig.Log.Printf("Sending start message to plugin service %s\n", service)
			go func(senderChan chan core.KernelCmd, pluginName string) {
				senderChan <- core.KernelCmd{
					PluginName: pluginName,
					Command:    core.PLUGIN_EVENT_START,
				}
			}(*pluginHandler.ConfigContext.CmdSenderChan, pluginHandler.Name)
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
			if globalPluginStatusChan != nil {
				<-globalPluginStatusChan
			}
			driverConfig.CoreConfig.Log.Printf("Kernel finished starting plugin: %s\n", pluginHandler.Name)
		case event.Command == core.PLUGIN_EVENT_STOP:
			driverConfig.CoreConfig.Log.Printf("Kernel finished stopping plugin: %s\n", pluginHandler.Name)
			pluginHandler.State = 0
			*pluginHandler.ConfigContext.ErrorChan <- errors.New(pluginHandler.Name + " shutting down")
			*pluginHandler.ConfigContext.DfsChan <- nil
			pluginHandler.PluginMod = nil
			if pluginHandler.KernelCtx != nil && pluginHandler.KernelCtx.PluginRestartChan != nil {
				go func(e core.KernelCmd) {
					*pluginHandler.KernelCtx.PluginRestartChan <- e
				}(event)
			}
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
		case pluginHandler.State == 2 && result != nil:
			if globalPluginStatusChan != nil {
				<-globalPluginStatusChan
			}
			eUtils.LogErrorObject(driverConfig.CoreConfig, result, false)
			return
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
		PluginName: pluginName,
		Command:    core.PLUGIN_EVENT_STOP,
	}
	driverConfig.CoreConfig.Log.Printf("Stop message successfully sent to plugin: %s\n", pluginName)
}

func LoadPluginPath(driverConfig *config.DriverConfig, pluginToolConfig map[string]interface{}) string {
	var deployroot string
	var service string
	var ext = ".so"
	if s, ok := pluginToolConfig["trctype"].(string); ok && s == "trcshkubeservice" {
		if s, ok := pluginToolConfig["trccodebundle"].(string); ok {
			driverConfig.CoreConfig.Log.Printf("Loading plugin path for service: %s\n", s)
			service = s
			ext = ""
		} else {
			driverConfig.CoreConfig.Log.Println("Unable to load plugin path for service.")
			return ""
		}
	} else {
		if s, ok := pluginToolConfig["trcplugin"].(string); ok {
			driverConfig.CoreConfig.Log.Printf("Loading plugin path for service: %s\n", s)
			service = s
		} else {
			driverConfig.CoreConfig.Log.Println("Unable to load plugin path for service.")
			return ""
		}
	}
	if d, ok := pluginToolConfig["trcdeployroot"].(string); ok {
		driverConfig.CoreConfig.Log.Printf("Loading plugin deploy root for service: %s\n", d)
		deployroot = d
	} else {
		driverConfig.CoreConfig.Log.Println("Unable to load plugin path for service.")
		return ""
	}
	pluginPath := fmt.Sprintf("%s/%s%s", deployroot, service, ext)
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

func (pluginHandler *PluginHandler) sendInitBroadcast(driverConfig *config.DriverConfig) {
	if pluginHandler == nil || (*pluginHandler).Name != "Kernel" || len(*pluginHandler.Services) == 0 {
		driverConfig.CoreConfig.Log.Printf("Initial broadcasting not supported for plugin: %s\n", pluginHandler.Name)
		return
	}
	if globalCertCache == nil {
		driverConfig.CoreConfig.Log.Printf("No cert information to broadcast\n")
		return
	}
	pHID, err := strconv.Atoi(pluginHandler.Id)
	if err != nil {
		pHID = 0
	}
	if pHID != 0 {
		driverConfig.CoreConfig.Log.Printf("Initial broadcasting not supported for kernel id: %s\n", pluginHandler.Id)
		return
	}
	response := ""
	for k, v := range globalCertCache.Items() {
		if v != nil && v.NotAfter != nil && !(*v.NotAfter).IsZero() && (*v.lastUpdate).IsZero() {
			info := fmt.Sprintf("%d-%02d-%02dT%02d:%02d:%02d",
				v.NotAfter.Year(), v.NotAfter.Month(), v.NotAfter.Day(),
				v.NotAfter.Hour(), v.NotAfter.Minute(), v.NotAfter.Second())
			response = response + fmt.Sprintf("Cert %s expires on %s\n", k, info)
			tiNow := time.Now()
			v.lastUpdate = &tiNow
		}
	}
	msg := &core.ChatMsg{
		Name:        &pluginHandler.Name,
		Query:       &[]string{"trcshtalk"},
		IsBroadcast: true,
		Response:    &response,
	}
	go func(recChan chan *core.ChatMsg, m *core.ChatMsg) {
		recChan <- m
	}(*pluginHandler.ConfigContext.ChatReceiverChan, msg)

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

	for len(globalPluginStatusChan) != 0 {
		time.Sleep(100 * time.Millisecond)
	}
	if !pluginopts.BuildOptions.IsPluginHardwired() {
		driverConfig.CoreConfig.Log.Println("All plugins have loaded, sending broadcast message...")
		pluginHandler.sendInitBroadcast(driverConfig)
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
					go func(sender chan *core.ChatMsg, message *core.ChatMsg) {
						sender <- message
					}(*p.ConfigContext.ChatSenderChan, &core.ChatMsg{
						Name:     msg.Name,
						KernelId: &pluginHandler.Id,
					})
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
				var chatSenderChan chan *core.ChatMsg
				if (*msg).IsBroadcast {
					if (*plugin.ConfigContext).ChatBroadcastChan != nil {
						new_msg.IsBroadcast = true
						chatSenderChan = *plugin.ConfigContext.ChatBroadcastChan
					} else {
						driverConfig.CoreConfig.Log.Printf("Service unavailable to broadcast query from %s\n", *msg.Name)
						continue
					}
				} else if (*plugin.ConfigContext).ChatSenderChan != nil {
					chatSenderChan = *plugin.ConfigContext.ChatSenderChan
				} else {
					driverConfig.CoreConfig.Log.Printf("Unable to send query from %s\n", *msg.Name)
					continue
				}
				go func(sender chan *core.ChatMsg, message *core.ChatMsg) {
					sender <- message
				}(chatSenderChan, new_msg)
			} else if eUtils.RefLength(msg.Name) > 0 && !msg.IsBroadcast {
				driverConfig.CoreConfig.Log.Printf("Service unavailable to process query from %s\n", *msg.Name)
				if plugin, ok := (*pluginHandler.Services)[*msg.Name]; ok {
					responseError := "Service unavailable"
					msg.Response = &responseError
					go func(sender chan *core.ChatMsg, message *core.ChatMsg) {
						sender <- message
					}(*plugin.ConfigContext.ChatSenderChan, msg)
				}
				continue
			} else {
				driverConfig.CoreConfig.Log.Println("Unable to interpret message.")
			}
		}
	}
}
