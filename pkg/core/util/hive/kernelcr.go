package hive

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"plugin"
	"reflect"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/trimble-oss/tierceron-core/v2/buildopts/plugincoreopts"
	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	"github.com/trimble-oss/tierceron-core/v2/core/coreconfig"
	"github.com/trimble-oss/tierceron-core/v2/core/coreconfig/cache"
	"github.com/trimble-oss/tierceron-core/v2/flow"
	"github.com/trimble-oss/tierceron/atrium/buildopts/flowopts"

	"github.com/trimble-oss/tierceron-core/v2/core/pluginsync"
	prod "github.com/trimble-oss/tierceron-core/v2/prod"
	flowcore "github.com/trimble-oss/tierceron/atrium/trcflow/core"
	trcflow "github.com/trimble-oss/tierceron/atrium/vestibulum/trcflow/flumen"
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
	"gopkg.in/yaml.v3"
)

// var PluginMods map[string]*plugin.Plugin = map[string]*plugin.Plugin{}
var dfstat *tccore.TTDINode

var m sync.Mutex

var globalCertCache *cmap.ConcurrentMap[string, *certValue]

var globalPluginStatusChan chan string

type certValue struct {
	CertBytes   *[]byte
	CreatedTime any
	NotAfter    *time.Time
	lastUpdate  *time.Time
	sha256      string
}

type PluginHandler struct {
	Name             string // service
	State            int    // 0 - initialized, 1 - running, 2 - failed
	Id               string
	KernelId         int
	Signature        string // sha256 of plugin
	ConfigContext    *tccore.ConfigContext
	Services         *map[string]*PluginHandler
	PluginMod        *plugin.Plugin
	KernelCtx        *KernelCtx
	ServiceResource  any
	DeploymentConfig map[string]interface{} // Full deployment configuration from Vault Certify
}

// IsRunningInKubernetes detects if the process is running in a Kubernetes/AKS environment
func IsRunningInKubernetes() bool {
	// Check for Kubernetes service host environment variable
	if _, exists := os.LookupEnv("KUBERNETES_SERVICE_HOST"); exists {
		return true
	}
	// Check for Kubernetes service account directory
	if _, err := os.Stat("/var/run/secrets/kubernetes.io"); err == nil {
		return true
	}
	return false
}

type KernelCtx struct {
	DeployRestartChan *chan string
	PluginRestartChan *chan tccore.KernelCmd
}

func InitKernel(id string) *PluginHandler {
	pluginMap := make(map[string]*PluginHandler)
	certCache := cmap.New[*certValue]()
	globalCertCache = &certCache
	deployRestart := make(chan string)
	pluginRestart := make(chan tccore.KernelCmd)
	chatReceiverChan := make(chan *tccore.ChatMsg)

	return &PluginHandler{
		Name:     "Kernel",
		Id:       id,
		KernelId: -1,
		State:    0,
		Services: &pluginMap,
		ConfigContext: &tccore.ConfigContext{
			ChatReceiverChan: &chatReceiverChan,
		},
		KernelCtx: &KernelCtx{
			DeployRestartChan: &deployRestart,
			PluginRestartChan: &pluginRestart,
		},
	}
}

func (pluginHandler *PluginHandler) GetKernelID() int {
	if pluginHandler == nil {
		return 0
	}
	if pluginHandler.KernelId == -1 && len(pluginHandler.Id) > 0 {
		idParts := strings.Split(pluginHandler.Id, "-")
		if len(idParts) > 1 {
			var kernParseErr error
			pluginHandler.KernelId, kernParseErr = strconv.Atoi(idParts[1])
			if kernParseErr != nil {
				pluginHandler.KernelId = 0
			}
		}
	}
	return pluginHandler.KernelId
}

var pendingPluginHandlers = make(chan *PluginHandler, 50) // Buffered to avoid blocking

// safeChannelSend safely sends to a channel with nil checks and closed channel protection
func safeChannelSend[T any](ch *chan T, value T, logPrefix string, log *log.Logger) (success bool) {
	if ch == nil || *ch == nil {
		if log != nil {
			log.Printf("safeChannelSend panic %s: attempted to send to nil channel\n", logPrefix)
		}
		success = false
		return
	}

	success = true
	defer func() {
		if r := recover(); r != nil {
			success = false
			if log != nil {
				log.Printf("safeChannelSend panic %s: sending to channel (likely closed): %v\n", logPrefix, r)
			}
		}
	}()

	*ch <- value
	return
}

func (pluginHandler *PluginHandler) DynamicReloader(driverConfig *config.DriverConfig) {
	if driverConfig == nil || driverConfig.CoreConfig == nil || driverConfig.CoreConfig.Log == nil {
		fmt.Fprintln(os.Stderr, "DriverConfig not properly initialized while attempting to start dynamic reloading.")
		return
	}
	if pluginHandler == nil || pluginHandler.Name != "Kernel" {
		driverConfig.CoreConfig.Log.Println("Unsupported handler attempting to start dynamic reloading.")
		return
	}

	defer func() {
		if r := recover(); r != nil {
			driverConfig.CoreConfig.Log.Printf("DynamicReloader panic recovered: %v\n%s", r, debug.Stack())
		}
	}()

	var mod *kv.Modifier
	pHID := 0
	pHIDs := strings.Split(pluginHandler.Id, "-")
	if len(pHIDs) > 0 {
		id, err := strconv.Atoi(pHIDs[len(pHIDs)-1])
		if err != nil {
			driverConfig.CoreConfig.Log.Println("Setting default handler id for dynamic reloading.")
		} else {
			pHID = id
		}
	}
	for {
		if mod == nil {
			var err error
			driverConfig.CoreConfig.Log.Println("")
			pluginConfig := make(map[string]any)
			pluginConfig["vaddress"] = *driverConfig.CoreConfig.TokenCache.VaultAddressPtr
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
						// validate cert and restart kernel
						configuredCert, err := certutil.LoadCertComponent(driverConfig,
							mod,
							k)
						if err != nil {
							eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
							continue
						}

						valid := false

						if strings.HasSuffix(k, ".crt.mf.tmpl") {
							valid, _, err = capauth.IsCertValidBySupportedDomains(configuredCert, validator.VerifyCertificate)
							if err != nil {
								eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
							}
						} else {
							valid = true
						}
						if valid {
							certSha256 := ""
							if len(configuredCert) > 0 {
								certHash := sha256.Sum256(configuredCert)
								certSha256 = hex.EncodeToString(certHash[:])
							} else {
								driverConfig.CoreConfig.Log.Println("Empty cert bytes loaded for cert reload check")
							}
							if certSha256 != v.sha256 {
								if pluginHandler.Services == nil {
									driverConfig.CoreConfig.Log.Println("Services map is nil, cannot iterate for cert reload")
									goto waitToReload
								}
								for s, sPluginHandler := range *pluginHandler.Services {
									if sPluginHandler == nil || sPluginHandler.ConfigContext == nil || sPluginHandler.ConfigContext.CmdSenderChan == nil {
										driverConfig.CoreConfig.Log.Printf("Service not properly initialized to shut down for cert reloading: %s\n", s)
										continue
									}
									if kernelopts.BuildOptions.IsKernel() {
										safeChannelSend(sPluginHandler.ConfigContext.CmdSenderChan, tccore.KernelCmd{
											PluginName: sPluginHandler.Name,
											Command:    tccore.PLUGIN_EVENT_STOP,
										}, fmt.Sprintf("cert reload shutdown %s", s), driverConfig.CoreConfig.Log)
									}
									driverConfig.CoreConfig.Log.Printf("Shutting down service: %s\n", s)
								}
								// TODO: Get rid of os.Exit
								// 0. Reload certificates
								// 1. Recall Init function for each plugin
								// 2. Start each plugin
								eUtils.LogSyncAndExit(driverConfig.CoreConfig.Log, "Shutting down kernel...", 0)
							} else {
								continue
							}
						} else {
							continue
						}
					}
				} else if v != nil && v.NotAfter != nil && v.lastUpdate != nil && !(*v.NotAfter).IsZero() && globalPluginStatusChan != nil && len(globalPluginStatusChan) == 0 {
					timeDiff := time.Until((*v.NotAfter))
					if timeDiff <= 0 && ((*v.lastUpdate).IsZero() || time.Since(*v.lastUpdate) < time.Hour) {
						response := fmt.Sprintf("Expired cert %s in kernel, shutting down services.", k)
						safeChannelSend(pluginHandler.ConfigContext.ChatReceiverChan, &tccore.ChatMsg{
							Name:        &pluginHandler.Name,
							Query:       &[]string{"trcshtalk"},
							IsBroadcast: true,
							Response:    &response,
						}, "expired cert notification", driverConfig.CoreConfig.Log)
						tiNow := time.Now()
						v.lastUpdate = &tiNow
						if pluginHandler.Services != nil {
							for s, sPluginHandler := range *pluginHandler.Services {
								if sPluginHandler != nil && sPluginHandler.ConfigContext != nil && (*sPluginHandler.ConfigContext).CmdSenderChan != nil {
									if sPluginHandler.Name != "healthcheck" {
										safeChannelSend(sPluginHandler.ConfigContext.CmdSenderChan, tccore.KernelCmd{
											PluginName: sPluginHandler.Name,
											Command:    tccore.PLUGIN_EVENT_STOP,
										}, fmt.Sprintf("cert expiration shutdown %s", s), driverConfig.CoreConfig.Log)
										driverConfig.CoreConfig.Log.Printf("Shutting down service: %s\n", s)
									}
								} else {
									driverConfig.CoreConfig.Log.Printf("Service not properly initialized to shut down for cert expiration: %s\n", s)
								}
							}
						}
					} else if timeDiff <= time.Hour*24 && ((*v.lastUpdate).IsZero() || time.Since(*v.lastUpdate) < time.Hour) && pHID == 0 {
						response := fmt.Sprintf("Cert %s expiring in %.2f hours.", k, timeDiff.Hours())
						safeChannelSend(pluginHandler.ConfigContext.ChatReceiverChan, &tccore.ChatMsg{
							Name:        &pluginHandler.Name,
							Query:       &[]string{"trcshtalk"},
							IsBroadcast: true,
							Response:    &response,
						}, "cert expiring hours", driverConfig.CoreConfig.Log)
						tiNow := time.Now()
						v.lastUpdate = &tiNow
					} else if timeDiff <= time.Hour*168 && ((*v.lastUpdate).IsZero() || time.Since(*v.lastUpdate) < time.Hour*24) && pHID == 0 {
						daysLeft := timeDiff.Hours() / 24.0
						response := fmt.Sprintf("Cert %s expiring in %d days.", k, int(daysLeft))
						safeChannelSend(pluginHandler.ConfigContext.ChatReceiverChan, &tccore.ChatMsg{
							Name:        &pluginHandler.Name,
							Query:       &[]string{"trcshtalk"},
							IsBroadcast: true,
							Response:    &response,
						}, "cert expiring days", driverConfig.CoreConfig.Log)
						tiNow := time.Now()
						v.lastUpdate = &tiNow
					}
				}
			}
		}
		if pluginHandler.KernelCtx != nil &&
			pluginHandler.KernelCtx.DeployRestartChan != nil &&
			pluginHandler.KernelCtx.PluginRestartChan != nil &&
			mod != nil &&
			!plugincoreopts.BuildOptions.IsPluginHardwired() {
			for service, servPh := range *pluginHandler.Services {
				certifyMap, err := mod.ReadData(fmt.Sprintf("super-secrets/Index/TrcVault/trcplugin/%s/Certify", service))
				if err != nil {
					pluginHandler.ConfigContext.Log.Printf("Unable to read certification data for %s %s\n", service, err)
					continue
				}

				if newSha, ok := certifyMap["trcsha256"]; ok {
					newShaStr, shaOk := newSha.(string)
					if !shaOk {
						driverConfig.CoreConfig.Log.Printf("Invalid trcsha256 type for service: %s\n", service)
						continue
					}
					if newShaStr != servPh.Signature && servPh.Signature != "" {
						driverConfig.CoreConfig.Log.Printf("Kernel shutdown, installing new service: %s\n", service)

						if t, ok := certifyMap["trctype"]; ok {
							trcTypeStr, typeOk := t.(string)
							if typeOk && trcTypeStr == "trcshkubeservice" {
								if servPh == nil || servPh.ConfigContext == nil || servPh.ConfigContext.CmdSenderChan == nil {
									driverConfig.CoreConfig.Log.Printf("Kube service not properly initialized to shut down: %s\n", service)
									goto waitToReload
								}
								driverConfig.CoreConfig.Log.Printf("Shutting down service: %s\n", service)
								safeChannelSend(servPh.ConfigContext.CmdSenderChan, tccore.KernelCmd{
									PluginName: servPh.Name,
									Command:    tccore.PLUGIN_EVENT_STOP,
								}, fmt.Sprintf("kube service restart %s", service), driverConfig.CoreConfig.Log)

								if pluginHandler.KernelCtx != nil && pluginHandler.KernelCtx.PluginRestartChan != nil && *pluginHandler.KernelCtx.PluginRestartChan != nil {
									cmd := <-*pluginHandler.KernelCtx.PluginRestartChan
									if cmd.Command == tccore.PLUGIN_EVENT_STOP {
										(*pluginHandler.Services)[service] = &PluginHandler{
											Name: service,
											ConfigContext: &tccore.ConfigContext{
												Log: driverConfig.CoreConfig.Log,
											},
											KernelCtx: &KernelCtx{
												PluginRestartChan: pluginHandler.KernelCtx.PluginRestartChan,
											},
										}
										driverConfig.CoreConfig.Log.Printf("Restarting service: %s\n", service)
										if pluginHandler.KernelCtx.DeployRestartChan != nil && *pluginHandler.KernelCtx.DeployRestartChan != nil {
											safeChannelSend(pluginHandler.KernelCtx.DeployRestartChan, service, fmt.Sprintf("kube service restart %s", service), driverConfig.CoreConfig.Log)
										}
									}
								}
								continue
							}
						}

						// Non-kube service shutdown
						if pluginHandler.Services != nil {
							for s, sPluginHandler := range *pluginHandler.Services {
								if sPluginHandler != nil && sPluginHandler.ConfigContext != nil && (*sPluginHandler.ConfigContext).CmdSenderChan != nil {
									driverConfig.CoreConfig.Log.Printf("Shutting down service: %s\n", s)
									safeChannelSend(sPluginHandler.ConfigContext.CmdSenderChan, tccore.KernelCmd{
										PluginName: sPluginHandler.Name,
										Command:    tccore.PLUGIN_EVENT_STOP,
									}, fmt.Sprintf("service shutdown %s", s), driverConfig.CoreConfig.Log)

									if pluginHandler.KernelCtx != nil && pluginHandler.KernelCtx.PluginRestartChan != nil && *pluginHandler.KernelCtx.PluginRestartChan != nil {
										cmd := <-*pluginHandler.KernelCtx.PluginRestartChan
										if cmd.Command == tccore.PLUGIN_EVENT_STOP {
											driverConfig.CoreConfig.Log.Printf("Shut down service: %s\n", s)
										}
									}
								} else {
									driverConfig.CoreConfig.Log.Printf("Service not properly initialized to shut down: %s\n", s)
									goto waitToReload
								}
							}
						}
						eUtils.LogAndSafeExit(driverConfig.CoreConfig, "Non kube shutting down kernel...", 0)
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
		valid := false
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
			certSha256 := ""
			if len(configuredCert) > 0 {
				certHash := sha256.Sum256(configuredCert)
				certSha256 = hex.EncodeToString(certHash[:])
			} else {
				driverConfig.CoreConfig.Log.Println("Empty cert bytes loaded for adding to cert cache")
			}
			globalCertCache.Set(path, &certValue{
				CreatedTime: t,
				CertBytes:   &configuredCert,
				NotAfter:    certNotAfter,
				lastUpdate:  &zeroTime,
				sha256:      certSha256,
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

func (pluginHandler *PluginHandler) AddKernelPlugin(service string, driverConfig *config.DriverConfig, deploymentConfig *map[string]interface{}) {
	if pluginHandler == nil || pluginHandler.Name != "Kernel" {
		driverConfig.CoreConfig.Log.Println("Unsupported handler attempting to add kernel service.")
		return
	}
	if pluginHandler.Services != nil {
		driverConfig.CoreConfig.Log.Printf("Added plugin to kernel: %s\n", service)
		var deployConfig map[string]interface{}
		if deploymentConfig != nil {
			deployConfig = *deploymentConfig
		}
		(*pluginHandler.Services)[service] = &PluginHandler{
			Name:             service,
			DeploymentConfig: deployConfig,
			ConfigContext: &tccore.ConfigContext{
				Log:              driverConfig.CoreConfig.Log,
				ChatReceiverChan: pluginHandler.ConfigContext.ChatReceiverChan,
			},
			KernelCtx: &KernelCtx{
				PluginRestartChan: pluginHandler.KernelCtx.PluginRestartChan,
			},
		}
	}
}

func (pluginHandler *PluginHandler) InitPluginStatus(driverConfig *config.DriverConfig) {
	if pluginHandler == nil || pluginHandler.Name != "Kernel" {
		driverConfig.CoreConfig.Log.Println("Unsupported handler attempting to add kernel service.")
		return
	}
	if pluginHandler.Services != nil {
		globalPluginStatusChan = make(chan string, len(*pluginHandler.Services))
		for k := range *pluginHandler.Services {
			globalPluginStatusChan <- k
		}
	}
}

func (pluginHandler *PluginHandler) GetPluginHandler(service string, driverConfig *config.DriverConfig) *PluginHandler {
	if pluginHandler != nil && pluginHandler.Services != nil {
		if plugin, ok := (*pluginHandler.Services)[service]; ok {
			return plugin
		} else {
			driverConfig.CoreConfig.Log.Printf("Handler not initialized for plugin to start: %s\n", service)
		}
	} else {
		driverConfig.CoreConfig.Log.Printf("No handlers provided for plugin service to startup: %s\n", service)
	}
	return nil
}

func (pluginHandler *PluginHandler) Init(properties *map[string]any) {
	if pluginHandler == nil || pluginHandler.ConfigContext == nil || pluginHandler.ConfigContext.Log == nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			if pluginHandler.ConfigContext != nil && pluginHandler.ConfigContext.Log != nil {
				pluginHandler.ConfigContext.Log.Printf("Init panic recovered for %s: %v\n%s", pluginHandler.Name, r, debug.Stack())
			}
		}
	}()

	if pluginHandler.Name == "" {
		pluginHandler.ConfigContext.Log.Println("No plugin name set for initializing plugin service.")
		return
	}

	// Check if this is a kernel-type plugin (uses callback pattern)
	var isKernelPlugin bool
	if properties != nil {
		if certify, ok := (*properties)["certify"].(map[string]interface{}); ok {
			if trctype, ok := certify["trctype"].(string); ok {
				isKernelPlugin = (trctype == "kernelplugin")
			}
		}
	}

	if isKernelPlugin {
		// Use callback-based initialization for kernel plugins
		pluginHandler.ConfigContext.Log.Printf("Initializing kernel plugin %s via callbacks\n", pluginHandler.Name)
		CallPluginInit(pluginHandler.Name, pluginHandler.Name, properties)
	} else if !plugincoreopts.BuildOptions.IsPluginHardwired() && pluginHandler.PluginMod != nil {
		if pluginHandler.PluginMod == nil {
			pluginHandler.ConfigContext.Log.Println("No plugin module set for initializing plugin service.")
			return
		}
		symbol, err := pluginHandler.PluginMod.Lookup("Init")
		if err != nil {
			pluginHandler.ConfigContext.Log.Printf("Unable to lookup plugin export: %s\n", err)
			return
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
	serviceConfig *map[string]any,
) {
	// Initialize channels
	sender := make(chan tccore.KernelCmd)
	pluginHandler.ConfigContext.CmdSenderChan = &sender
	msgSender := make(chan *tccore.ChatMsg)
	pluginHandler.ConfigContext.ChatSenderChan = &msgSender

	broadcastChan := make(chan *tccore.ChatMsg)
	pluginHandler.ConfigContext.ChatBroadcastChan = &broadcastChan

	errReceiver := make(chan error)
	pluginHandler.ConfigContext.ErrorChan = &errReceiver
	ttdiReceiver := make(chan *tccore.TTDINode)
	pluginHandler.ConfigContext.DfsChan = &ttdiReceiver
	statusReceiver := make(chan tccore.KernelCmd)
	pluginHandler.ConfigContext.CmdReceiverChan = &statusReceiver

	if pluginHandler.ConfigContext.ChatReceiverChan == nil {
		driverConfig.CoreConfig.Log.Printf("Unable to access chat configuration data for %s\n", service)
		return
	}

	chan_map := make(map[string]any)

	chan_map[tccore.PLUGIN_CHANNEL_EVENT_IN] = make(map[string]any)
	chan_map[tccore.PLUGIN_CHANNEL_EVENT_IN].(map[string]any)[tccore.CMD_CHANNEL] = pluginHandler.ConfigContext.CmdSenderChan
	chan_map[tccore.PLUGIN_CHANNEL_EVENT_IN].(map[string]any)[tccore.CHAT_CHANNEL] = pluginHandler.ConfigContext.ChatSenderChan

	chan_map[tccore.PLUGIN_CHANNEL_EVENT_OUT] = make(map[string]any)
	chan_map[tccore.PLUGIN_CHANNEL_EVENT_OUT].(map[string]any)[tccore.ERROR_CHANNEL] = pluginHandler.ConfigContext.ErrorChan
	chan_map[tccore.PLUGIN_CHANNEL_EVENT_OUT].(map[string]any)[tccore.DATA_FLOW_STAT_CHANNEL] = pluginHandler.ConfigContext.DfsChan
	chan_map[tccore.PLUGIN_CHANNEL_EVENT_OUT].(map[string]any)[tccore.CMD_CHANNEL] = pluginHandler.ConfigContext.CmdReceiverChan
	chan_map[tccore.PLUGIN_CHANNEL_EVENT_OUT].(map[string]any)[tccore.CHAT_CHANNEL] = pluginHandler.ConfigContext.ChatReceiverChan

	chan_map[tccore.CHAT_BROADCAST_CHANNEL] = pluginHandler.ConfigContext.ChatBroadcastChan

	(*serviceConfig)[tccore.PLUGIN_EVENT_CHANNELS_MAP_KEY] = chan_map
	(*serviceConfig)["log"] = driverConfig.CoreConfig.Log
	(*serviceConfig)["env"] = driverConfig.CoreConfig.Env
	(*serviceConfig)["isKubernetes"] = IsRunningInKubernetes()
	(*serviceConfig)["isKernelZ"] = kernelopts.BuildOptions.IsKernelZ()

	// Security: KernelZ only allows trcshcmd, trcsh, and rosea plugins
	if kernelopts.BuildOptions.IsKernelZ() {
		if service != "trcshcmd" && service != "trcsh" && service != "rosea" {
			driverConfig.CoreConfig.Log.Printf("Security: Plugin %s not allowed in KernelZ.", service)
			return
		}
	}

	go pluginHandler.handleErrors(driverConfig)
	*driverConfig.CoreConfig.CurrentTokenNamePtr = "config_token_pluginany"

	// Use cached DeploymentConfig instead of reading from Vault
	if pluginHandler.DeploymentConfig == nil {
		fmt.Fprintf(os.Stderr, "Kernel Missing plugin certification: %s.\n", pluginHandler.Name)
		return
	}
	(*serviceConfig)["certify"] = pluginHandler.DeploymentConfig

	// Determine kernel plugin type before starting receiver to avoid race
	var isKernelPlugin bool
	if pluginHandler.DeploymentConfig != nil {
		if trctype, ok := pluginHandler.DeploymentConfig["trctype"].(string); ok {
			isKernelPlugin = (trctype == "kernelplugin")
		}
	}
	go pluginHandler.receiver(driverConfig, isKernelPlugin)
	pluginHandler.Init(serviceConfig)

	// Check if plugin refused to initialize
	if refused, ok := (*serviceConfig)["pluginRefused"].(bool); ok && refused {
		driverConfig.CoreConfig.Log.Printf("Plugin %s refused to initialize. Skipping start.", service)
		return
	}

	driverConfig.CoreConfig.Log.Printf("Sending start message to plugin service %s\n", service)
	safeChannelSend(pluginHandler.ConfigContext.CmdSenderChan, tccore.KernelCmd{
		PluginName: pluginHandler.Name,
		Command:    tccore.PLUGIN_EVENT_START,
	}, fmt.Sprintf("start message to %s", service), driverConfig.CoreConfig.Log)
	driverConfig.CoreConfig.Log.Printf("Successfully sent start message to plugin service %s\n", service)
}

func (pluginHandler *PluginHandler) PluginserviceStart(driverConfig *config.DriverConfig, pluginToolConfig map[string]any) {
	if driverConfig == nil || driverConfig.CoreConfig == nil || driverConfig.CoreConfig.Log == nil {
		fmt.Fprintln(os.Stderr, "No logger or config passed in to plugin service")
		return
	}

	if pluginHandler == nil || pluginHandler.ConfigContext == nil {
		driverConfig.CoreConfig.Log.Println("Invalid plugin handler state")
		return
	}

	if pluginHandler.ConfigContext.Log == nil {
		pluginHandler.ConfigContext.Log = driverConfig.CoreConfig.Log
	}

	// Always add panic recovery, not just in kernel mode
	defer func() {
		if r := recover(); r != nil {
			if pluginHandler.ConfigContext != nil && pluginHandler.ConfigContext.Log != nil {
				pluginHandler.ConfigContext.Log.Printf("PluginserviceStart panic recovered for %s: %v\n%s", pluginHandler.Name, r, debug.Stack())
			} else {
				fmt.Fprintf(os.Stderr, "PluginserviceStart panic: %v\n%s", r, debug.Stack())
			}
		}
	}()

	if pluginHandler.Name == "" {
		driverConfig.CoreConfig.Log.Println("No plugin name specified to start plugin service.")
		return
	}
	if !plugincoreopts.BuildOptions.IsPluginHardwired() && pluginHandler.PluginMod == nil {
		if s, ok := pluginToolConfig["trctype"].(string); ok && (s == "trcshpluginservice" || s == "trcflowpluginservice") {
			driverConfig.CoreConfig.Log.Printf("No plugin module initialized to start plugin service: %s\n", pluginHandler.Name)
			return
		}
	}
	var service string
	if s, ok := (*driverConfig.DeploymentConfig)["trcplugin"].(string); ok {
		service = s
	} else {
		driverConfig.CoreConfig.Log.Println("Unable to process plugin service.")
		return
	}
	driverConfig.CoreConfig.Log.Printf("Starting initialization for plugin service: %s Env: %s\n", service, driverConfig.CoreConfig.EnvBasis)
	var pluginConfig map[string]any
	var flowMachineInitContext any
	if s, ok := pluginToolConfig["trctype"].(string); ok && s == "trcflowpluginservice" {
		if !plugincoreopts.BuildOptions.IsPluginHardwired() && pluginHandler.PluginMod != nil {
			getFlowMachineInitContext, err := pluginHandler.PluginMod.Lookup("GetFlowMachineInitContext")
			if err != nil {
				driverConfig.CoreConfig.Log.Printf("GetFlowMachineInitContext not set up for %s\n", service)
				driverConfig.CoreConfig.Log.Printf("Returned with %v\n", err)
				return
			}

			getFlowMachineInitContextFunc, ok := getFlowMachineInitContext.(func(*coreconfig.CoreConfig, string) *flow.FlowMachineInitContext)
			if !ok {
				driverConfig.CoreConfig.Log.Printf("GetFlowMachineInitContext has wrong type for %s\n", service)
				return
			}
			flowMachineInitContext = getFlowMachineInitContextFunc(driverConfig.CoreConfig, pluginHandler.Name)
		} else if plugincoreopts.BuildOptions.IsPluginHardwired() {
			flowMachineInitContext = pluginopts.BuildOptions.GetFlowMachineInitContext(driverConfig.CoreConfig, pluginHandler.Name)
		} else {
			driverConfig.CoreConfig.Log.Printf("Missing flow machine context %s\n", service)
			return
		}
		if flowMachineInitContext != nil {
			if fmCtx, ok := flowMachineInitContext.(*flow.FlowMachineInitContext); ok {
				pluginConfig = fmCtx.GetFlowMachineTemplates()
			} else {
				driverConfig.CoreConfig.Log.Printf("Flow machine context has wrong type for %s\n", service)
				return
			}
		} else {
			driverConfig.CoreConfig.Log.Printf("Missing flow machine context %s\n", service)
			return
		}
	} else {
		pluginConfig = make(map[string]any)
	}
	pluginConfig["vaddress"] = *driverConfig.CoreConfig.TokenCache.VaultAddressPtr
	if len(driverConfig.CoreConfig.Regions) > 0 {
		pluginConfig["regions"] = driverConfig.CoreConfig.Regions
	}
	currentTokenNamePtr := driverConfig.CoreConfig.GetCurrentToken("config_token_%s")
	driverConfig.CoreConfig.CurrentTokenNamePtr = currentTokenNamePtr
	pluginConfig["env"] = driverConfig.CoreConfig.EnvBasis

	if !plugincoreopts.BuildOptions.IsPluginHardwired() {
		if pluginHandler.PluginMod != nil {
			setProdFuncHandle, err := pluginHandler.PluginMod.Lookup("SetProd")
			if err == nil && setProdFuncHandle != nil {
				if setProdFunc, ok := setProdFuncHandle.(func(bool)); ok {
					driverConfig.CoreConfig.Log.Printf("Setting production environment for %s\n", service)
					setProdFunc(prod.IsProd())
				} else {
					driverConfig.CoreConfig.Log.Printf("SetProd has wrong type for %s\n", service)
				}
			} else {
				driverConfig.CoreConfig.Log.Printf("Setting production environment is not implemented for %s\n", service)
			}
		} else {
			driverConfig.CoreConfig.Log.Printf("Setting production environment is not implemented for %s\n", service)
		}
	}

	_, mod, vault, err := eUtils.InitVaultModForPlugin(pluginConfig,
		driverConfig.CoreConfig.TokenCache,
		*currentTokenNamePtr,
		driverConfig.CoreConfig.Log)
	if err != nil {
		driverConfig.CoreConfig.Log.Printf("Problem initializing mod: %s\n", err)
		return
	}

	if vault != nil {
		defer vault.Close()
	}
	if pluginprojserv, ok := pluginToolConfig["trcprojectservice"]; ok {
		var bootDriverConfig *config.DriverConfig
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
			if !plugincoreopts.BuildOptions.IsPluginHardwired() && pluginHandler.PluginMod != nil {
				getConfigPaths, err := pluginHandler.PluginMod.Lookup("GetConfigPaths")
				if err != nil {
					driverConfig.CoreConfig.Log.Printf("Unable to access config for %s\n", service)
					driverConfig.CoreConfig.Log.Printf("Returned with %v\n", err)
					return
				}

				if pluginConfigPaths, ok := getConfigPaths.(func(string) []string); ok {
					paths = pluginConfigPaths(pluginHandler.Name)
				} else {
					driverConfig.CoreConfig.Log.Printf("GetConfigPaths has wrong type for %s\n", service)
					return
				}
			} else {
				if s, ok := pluginToolConfig["trctype"].(string); plugincoreopts.BuildOptions.IsPluginHardwired() || (ok && (s == "trcshkubeservice") || (s == "trcshcmdtoolplugin")) {
					paths = pluginopts.BuildOptions.GetConfigPaths(pluginHandler.Name)
				} else {
					driverConfig.CoreConfig.Log.Printf("Unable to access config for %s\n", service)
					driverConfig.CoreConfig.Log.Printf("Returned with %v\n", err)
					return
				}
			}

			serviceConfig := make(map[string]any)
			for _, path := range paths {
				if strings.HasPrefix(path, "Common") {
					if v, ok := globalCertCache.Get(path); ok {
						driverConfig.CoreConfig.WantCerts = false
						serviceConfig[path] = *v.CertBytes
					} else {
						configuredCert, err := addToCache(path, driverConfig, mod)
						if err != nil {
							driverConfig.CoreConfig.Log.Printf("Unable to load cert: %v for plugin: %s\n", err, service)
							if pluginHandler.ConfigContext.ChatReceiverChan != nil {
								go func(recChan *chan *tccore.ChatMsg, log *log.Logger) {
									for {
										if len(globalPluginStatusChan) == 0 {
											break
										}
										time.Sleep(5 * time.Second)
									}
									reportMsg := fmt.Sprintf("🚨Critical failure loading plugin: %s. Unable to load cert: %s\n", service, path)
									safeChannelSend(recChan, &tccore.ChatMsg{
										Name:        &service,
										Query:       &[]string{"trcshtalk"},
										IsBroadcast: true,
										Response:    &reportMsg,
									}, fmt.Sprintf("critical cert failure %s", service), log)
								}(pluginHandler.ConfigContext.ChatReceiverChan, driverConfig.CoreConfig.Log)
							} else {
								driverConfig.CoreConfig.Log.Printf("Unable to broadcast invalid cert: %s loading for plugin: %s\n", path, service)
							}
						} else {
							serviceConfig[path] = *configuredCert
						}
					}
				} else {
					if pluginToolConfig["trctype"] == "trcshkubeservice" || pluginToolConfig["trctype"] == "trcflowpluginservice" {
						driverConfig.CoreConfig.Log.Printf("Preparing to load HARBINGER_INTERFACE_CONFIG\n")

						envArg := fmt.Sprintf("-env=%s", driverConfig.CoreConfig.EnvBasis)
						restrictedMappingSub := append([]string{"", envArg}, paths[0])
						ctl := "pluginrun"
						flagset := flag.NewFlagSet(ctl, flag.ExitOnError)
						flagset.String("env", "dev", "Environment to configure")

						wantedTokenName := driverConfig.CoreConfig.GetCurrentToken("config_token_%s")
						subErr := trcsubbase.CommonMain(&driverConfig.CoreConfig.EnvBasis,
							&driverConfig.CoreConfig.EnvBasis,
							wantedTokenName,
							flagset,
							restrictedMappingSub,
							driverConfig)

						if subErr != nil {
							driverConfig.CoreConfig.Log.Printf("Could not load templates for plugin: %s error: %v\n", pluginHandler.Name, err)
							return
						}

						driverConfig.EndDir = "."
						restrictedMappingConfig := []string{"", envArg}
						flagset = flag.NewFlagSet(ctl, flag.ExitOnError)
						flagset.String("env", "dev", "Environment to configure")

						// Get certs...
						driverConfig.CoreConfig.WantCerts = true
						kernelEnvBasis := driverConfig.CoreConfig.EnvBasis
						configErr := trcconfigbase.CommonMain(&kernelEnvBasis,
							&kernelEnvBasis,
							wantedTokenName, // wantedTokenName
							nil,             // regionPtr
							flagset,
							restrictedMappingConfig,
							driverConfig)

						if configErr != nil {
							driverConfig.CoreConfig.Log.Printf("Could not prepare certificates for plugin: %s using token named: %s\n", pluginHandler.Name, *wantedTokenName)
							return
						}

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
						kernelEnvBasis = driverConfig.CoreConfig.EnvBasis
						configErr = trcconfigbase.CommonMain(&kernelEnvBasis,
							&kernelEnvBasis,
							wantedTokenName, // tokenName
							nil,             // regionPtr
							flagset,
							restrictedMappingConfig,
							driverConfig)

						if configErr != nil {
							driverConfig.CoreConfig.Log.Printf("Could not generate configs for plugin: %s using token named: %s\n", pluginHandler.Name, *wantedTokenName)
							return
						}

						driverConfig.MemFs.ClearCache("./trc_templates")
						driverConfig.MemFs.ClearCache("./deploy")
						driverConfig.MemFs.SerializeToMap(".", serviceConfig)
						if flowopts.BuildOptions.AllowTrcdbInterfaceOverride() {
							if s, ok := pluginToolConfig["trctype"].(string); ok && s == "trcflowpluginservice" {
								// Make plugin configs available to flowMachineContext
								var harbingerConfig map[string]any
								if configBytes, ok := serviceConfig[flow.HARBINGER_INTERFACE_CONFIG].([]byte); ok {
									err := yaml.Unmarshal(configBytes, &harbingerConfig)
									if err == nil {
										flowMachineInitContext.(*flow.FlowMachineInitContext).FlowMachineInterfaceConfigs = harbingerConfig
										serviceConfig[flow.HARBINGER_INTERFACE_CONFIG] = harbingerConfig
									} else {
										driverConfig.CoreConfig.Log.Printf("Unsupported secret values for plugin %s\n", service)
										return
									}
								}
							}
						} else {
							if s, ok := pluginToolConfig["trctype"].(string); ok && s == "trcflowpluginservice" {
								// Make plugin configs available to flowMachineContext
								var harbingerConfig map[string]any
								if configBytes, ok := serviceConfig[flow.HARBINGER_INTERFACE_CONFIG].([]byte); ok {
									err := yaml.Unmarshal(configBytes, &harbingerConfig)
									delete(harbingerConfig, "controllerdbport")
									delete(harbingerConfig, "controllerdbuser")
									delete(harbingerConfig, "controllerdbpassword")
									delete(harbingerConfig, "dbport")
									delete(harbingerConfig, "dbuser")
									delete(harbingerConfig, "dbpassword")
									if err == nil {
										serviceConfig[flow.HARBINGER_INTERFACE_CONFIG] = harbingerConfig
									} else {
										driverConfig.CoreConfig.Log.Printf("Unsupported secret values for plugin %s\n", service)
										return
									}
								} else {
									driverConfig.CoreConfig.Log.Printf("Critical error.  Missing config. Cannot load plugin %s\n", service)
									return
								}
							}
						}
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
			sender := make(chan tccore.KernelCmd)
			pluginHandler.ConfigContext.CmdSenderChan = &sender
			msgSender := make(chan *tccore.ChatMsg)
			pluginHandler.ConfigContext.ChatSenderChan = &msgSender

			broadcastChan := make(chan *tccore.ChatMsg)
			pluginHandler.ConfigContext.ChatBroadcastChan = &broadcastChan

			errReceiver := make(chan error)
			pluginHandler.ConfigContext.ErrorChan = &errReceiver
			ttdiReceiver := make(chan *tccore.TTDINode)
			pluginHandler.ConfigContext.DfsChan = &ttdiReceiver
			statusReceiver := make(chan tccore.KernelCmd)
			pluginHandler.ConfigContext.CmdReceiverChan = &statusReceiver

			if pluginHandler.ConfigContext.ChatReceiverChan == nil {
				driverConfig.CoreConfig.Log.Printf("Unable to access chat configuration data for %s\n", service)
				return
			}

			chan_map := make(map[string]any)

			chan_map[tccore.PLUGIN_CHANNEL_EVENT_IN] = make(map[string]any)
			chan_map[tccore.PLUGIN_CHANNEL_EVENT_IN].(map[string]any)[tccore.CMD_CHANNEL] = pluginHandler.ConfigContext.CmdSenderChan
			chan_map[tccore.PLUGIN_CHANNEL_EVENT_IN].(map[string]any)[tccore.CHAT_CHANNEL] = pluginHandler.ConfigContext.ChatSenderChan

			chan_map[tccore.PLUGIN_CHANNEL_EVENT_OUT] = make(map[string]any)
			chan_map[tccore.PLUGIN_CHANNEL_EVENT_OUT].(map[string]any)[tccore.ERROR_CHANNEL] = pluginHandler.ConfigContext.ErrorChan
			chan_map[tccore.PLUGIN_CHANNEL_EVENT_OUT].(map[string]any)[tccore.DATA_FLOW_STAT_CHANNEL] = pluginHandler.ConfigContext.DfsChan
			chan_map[tccore.PLUGIN_CHANNEL_EVENT_OUT].(map[string]any)[tccore.CMD_CHANNEL] = pluginHandler.ConfigContext.CmdReceiverChan
			chan_map[tccore.PLUGIN_CHANNEL_EVENT_OUT].(map[string]any)[tccore.CHAT_CHANNEL] = pluginHandler.ConfigContext.ChatReceiverChan

			chan_map[tccore.CHAT_BROADCAST_CHANNEL] = pluginHandler.ConfigContext.ChatBroadcastChan

			serviceConfig[tccore.PLUGIN_EVENT_CHANNELS_MAP_KEY] = chan_map
			serviceConfig["log"] = driverConfig.CoreConfig.Log
			serviceConfig["env"] = driverConfig.CoreConfig.Env
			serviceConfig["isKubernetes"] = IsRunningInKubernetes()
			serviceConfig["isKernelZ"] = kernelopts.BuildOptions.IsKernelZ()
			go pluginHandler.handleErrors(driverConfig)
			*driverConfig.CoreConfig.CurrentTokenNamePtr = "config_token_pluginany"

			// Use cached DeploymentConfig instead of reading from Vault
			if pluginHandler.DeploymentConfig == nil {
				fmt.Fprintf(os.Stderr, "Kernel Missing plugin certification: %s.\n", pluginHandler.Name)
				return
			}
			(serviceConfig)["certify"] = pluginHandler.DeploymentConfig

			// Add driverConfig for kernel plugins only
			if trctype, ok := pluginHandler.DeploymentConfig["trctype"].(string); ok && trctype == "kernelplugin" {
				(serviceConfig)["driverConfig"] = driverConfig
			}

			// Once configurations are retrieved from the plugin, start the flow service if this is it's type.
			if s, ok := pluginToolConfig["trctype"].(string); ok && s == "trcflowpluginservice" {
				go func() {
					pendingPluginHandlers <- pluginHandler
				}()
				pluginConfig["tokenptr"] = driverConfig.CoreConfig.TokenCache.GetToken(*currentTokenNamePtr)
				pluginConfig["pluginName"] = pluginHandler.Name
				if kernelopts.BuildOptions.IsKernel() {
					// Kernel provides it's own templates generally through plugin itself.
					// Add a 'debugging' interface for hardwired only.
					if plugincoreopts.BuildOptions.IsPluginHardwired() {
						pluginConfig["connectionPath"] = append(
							pluginConfig["connectionPath"].([]string),
							"trc_templates/TrcVault/VaultDatabase/config.yml.tmpl", // providing for setup.
						)
					}
				} else {
					// Presently trcshk does not have permissions it needs to access this area of vault.
					pluginConfig["connectionPath"] = []string{
						// No interfaces
						"trc_templates/TrcVault/VaultDatabase/config.yml.tmpl",  // implemented
						"trc_templates/TrcVault/Database/config.yml.tmpl",       // implemented
						"trc_templates/TrcVault/SpiralDatabase/config.yml.tmpl", // implemented
					}
				}

				pluginConfig["kernelId"] = pluginHandler.GetKernelID()

				// Grab app role and secret and addr and env from service config and call auto auth
				// auto auth will return token
				// Create own driver config
				if flowConfigs, ok := serviceConfig[flow.HARBINGER_INTERFACE_CONFIG]; ok {
					driverConfig.CoreConfig.Log.Printf("Found HARBINGER_INTERFACE_CONFIG: %v", ok)

					configMask := 0
					const (
						RATTAN_ROLE_MASK = 1 << 0 // 1
						RATTAN_ENV_MASK  = 1 << 1 // 2
						VAULT_ADDR_MASK  = 1 << 2 // 4
					)

					if flowMachineConfig, ok := flowConfigs.(map[string]any); ok {
						if rattanRole, ok := flowMachineConfig["rattan_role"].(string); ok {
							configMask |= RATTAN_ROLE_MASK

							if rattanEnv, ok := flowMachineConfig["rattan_env"].(string); ok {
								configMask |= RATTAN_ENV_MASK
								if rattanAddress, ok := flowMachineConfig["vault_addr"].(string); ok {
									configMask |= VAULT_ADDR_MASK
									insecure := false
									driverConfig.CoreConfig.Log.Printf("HARBINGER_INTERFACE_CONFIG requirements met.")

									bootDriverConfig = &config.DriverConfig{
										CoreConfig: &coreconfig.CoreConfig{
											ExitOnFailure: true,
											TokenCache:    cache.NewTokenCacheEmpty(&rattanAddress),
											Regions:       driverConfig.CoreConfig.Regions,
											Insecure:      insecure,
											Log:           driverConfig.CoreConfig.Log,
											Env:           rattanEnv,
											EnvBasis:      rattanEnv,
										},
									}
									bootDriverConfig.CoreConfig.TokenCache.AddRoleStr("rattan", &rattanRole)
									tokenPtr := fmt.Sprintf("config_token_plugin%s", rattanEnv)
									currentTokenNamePtr := &tokenPtr
									currentRattanRoleEntity := "rattan"
									rattanToken := new(string)

									autoErr := eUtils.AutoAuth(bootDriverConfig, currentTokenNamePtr, &rattanToken, &rattanEnv, nil, &currentRattanRoleEntity, false)
									if autoErr == nil {
										// Satisfy requirements for flow machine.
										// It expects a token named unrestricted.
										// Although we'll hand it a restricted token for now.
										// This is actually a config_token_plugin<rattanEnv> token, but naming it unrestricted
										// only in the context of bootDriverConfig
										rattanTokenAlias := fmt.Sprintf("config_token_%s_unrestricted", rattanEnv)
										bootDriverConfig.CoreConfig.TokenCache.AddToken(rattanTokenAlias, rattanToken)
										currentTokenNamePtr = &rattanTokenAlias
									}

									bootDriverConfig.CoreConfig.CurrentTokenNamePtr = currentTokenNamePtr
								}
							}
						}
					}
					var missingFields string
					if configMask&RATTAN_ROLE_MASK == 0 {
						missingFields += "rattan_role "
					}
					if configMask&RATTAN_ENV_MASK == 0 {
						missingFields += "rattan_env "
					}
					if configMask&VAULT_ADDR_MASK == 0 {
						missingFields += "vault_addr "
					}
					if configMask != (RATTAN_ROLE_MASK | RATTAN_ENV_MASK | VAULT_ADDR_MASK) {
						driverConfig.CoreConfig.Log.Printf("Missing required fields in HARBINGER_INTERFACE_CONFIG: %s", missingFields)
					}

				} else {
					// We think it went here...
					driverConfig.CoreConfig.Log.Printf("WARNING: Missing HARBINGER_INTERFACE_CONFIG, using default driver config")
					bootDriverConfig = driverConfig
				}

				go func() {
					// Process all handlers that need the boot config
					for handler := range pendingPluginHandlers {
						_, statMod, _, err := eUtils.InitVaultMod(bootDriverConfig)
						if err != nil {
							bootDriverConfig.CoreConfig.Log.Printf("Problem initializing stat mod for %s: %s\n",
								handler.Name, err)
							continue
						}

						var isKernelPlugin bool
						if pluginHandler.DeploymentConfig != nil {
							if trctype, ok := pluginHandler.DeploymentConfig["trctype"].(string); ok {
								isKernelPlugin = (trctype == "kernelplugin")
							}
						}

						go handler.handleDataflowStat(bootDriverConfig, statMod, nil)
						go handler.receiver(bootDriverConfig, isKernelPlugin)
					}
				}()

				if bootDriverConfig == nil && driverConfig.CoreConfig.IsEditor {
					bootDriverConfig = driverConfig
				}
				// Use the plugin dfs channel
				flowMachineInitContext.(*flow.FlowMachineInitContext).DfsChan = pluginHandler.ConfigContext.DfsChan

				// Needs certifyPath and connectionPath
				tfmContext, err := trcflow.BootFlowMachine(flowMachineInitContext.(*flow.FlowMachineInitContext), bootDriverConfig, pluginConfig, pluginHandler.ConfigContext.Log)
				if err != nil || tfmContext == nil {
					driverConfig.CoreConfig.Log.Printf("Error initializing flow machine for %s: %v\n", service, err)
					return
				} else {
					pluginHandler.ServiceResource = tfmContext
				}
				tfmContext.(flow.FlowMachineContext).SetFlowIDs()
				tfmContext.(flow.FlowMachineContext).WaitAllFlowsLoaded()
				// kick off reload process from vault
				go reloadFlows(tfmContext.(flow.FlowMachineContext), mod)
				serviceConfig[tccore.TRCDB_RESOURCE] = tfmContext
			} else {
				// Initialize vault mod for non-flow plugins that need it (e.g., dataflow statistics)
				_, kernelmod, kernelvault, err := eUtils.InitVaultMod(driverConfig)
				if err != nil {
					driverConfig.CoreConfig.Log.Printf("Problem initializing stat mod: %s  Continuing without stats.\n", err)
				} else {
					if kernelvault != nil {
						defer kernelvault.Close()
					}

					go pluginHandler.handleDataflowStat(driverConfig, kernelmod, nil)
				}

				// Determine if this is a kernel plugin BEFORE starting receiver to avoid race
				var isKernelPlugin bool
				if pluginHandler.DeploymentConfig != nil {
					if trctype, ok := pluginHandler.DeploymentConfig["trctype"].(string); ok {
						isKernelPlugin = (trctype == "kernelplugin")
					}
				}

				go pluginHandler.receiver(driverConfig, isKernelPlugin)
				serviceConfig["region"] = driverConfig.CoreConfig.Regions[0]
			}

			pluginHandler.Init(&serviceConfig)

			// Check if plugin refused to initialize
			if refused, ok := serviceConfig["pluginRefused"].(bool); ok && refused {
				driverConfig.CoreConfig.Log.Printf("Plugin %s refused to initialize. Skipping start.", service)
				return
			}

			driverConfig.CoreConfig.Log.Printf("Sending start message to plugin service %s\n", service)
			go safeChannelSend(pluginHandler.ConfigContext.CmdSenderChan, tccore.KernelCmd{
				PluginName: pluginHandler.Name,
				Command:    tccore.PLUGIN_EVENT_START,
			}, fmt.Sprintf("start message to %s", service), driverConfig.CoreConfig.Log)
			driverConfig.CoreConfig.Log.Printf("Successfully sent start message to plugin service %s\n", service)
		}
	}
}

func reloadFlows(tfmContext flow.FlowMachineContext, mod *kv.Modifier) {
	for {
		for _, tfCtx := range tfmContext.GetFlows() {
			// load last modified time from vault
			flowPath := fmt.Sprintf("super-secrets/Index/FlumeDatabase/flowName/%s/%s", tfCtx.GetFlowHeader().TableName(), flow.TierceronControllerFlow.FlowName())
			dataMap, readErr := mod.ReadData(flowPath)
			if readErr == nil && len(dataMap) > 0 && dataMap["lastModified"] != nil {
				if tfCtx.GetLastRefreshedTime() == "" {
					// If RefreshedTime is not set, set it and skip refresh to avoid unnecessary restarts on boot
					tfCtx.SetLastRefreshedTime(dataMap["lastModified"].(string))
					continue
				}
				lastModifiedTime, err := time.Parse("2006-01-02 15:04:05 -0700 MST", dataMap["lastModified"].(string))
				if err != nil {
					tfmContext.Log(fmt.Sprintf("Error parsing last modified time for flow %s", tfCtx.GetFlowHeader().TableName()), err)
					continue
				}
				flowLastModified, err := time.Parse("2006-01-02 15:04:05 -0700 MST", tfCtx.GetLastRefreshedTime())
				if tfCtx.GetLastRefreshedTime() != "" && err != nil {
					tfmContext.Log(fmt.Sprintf("Error parsing existing last modified time for flow %s", tfCtx.GetFlowHeader().TableName()), err)
					continue
				}
				if flowLastModified.Before(lastModifiedTime) {
					tfCtx.SetLastRefreshedTime(dataMap["lastModified"].(string))
					// Need to refresh flow
					tfmContext.LockFlow(tfCtx.GetFlowHeader().FlowNameType())
					tfCtx.NotifyFlowComponentNeedsRestart()
					tfmContext.UnlockFlow(tfCtx.GetFlowHeader().FlowNameType())
				}
			}
		}
		time.Sleep(5 * time.Minute)
	}
}

func (pluginHandler *PluginHandler) receiver(driverConfig *config.DriverConfig, isKernelPlugin bool) {
	for {
		event := <-*pluginHandler.ConfigContext.CmdReceiverChan
		switch {
		case event.Command == tccore.PLUGIN_EVENT_START:
			if isKernelPlugin {
				// Use callback-based start for kernel plugins
				driverConfig.CoreConfig.Log.Printf("Starting kernel plugin %s via callbacks\n", pluginHandler.Name)
				CallPluginStart(pluginHandler.Name)
				// For kernel plugins, wait until they signal ready before setting State
			}
			pluginHandler.State = 1
			pluginsync.SignalPluginReady(pluginHandler.Name)
			if isKernelPlugin {
				driverConfig.CoreConfig.Log.Printf("Kernel plugin %s is ready\n", pluginHandler.Name)
			}
			if globalPluginStatusChan != nil {
				<-globalPluginStatusChan
			}
			driverConfig.CoreConfig.Log.Printf("Kernel finished starting plugin: %s\n", pluginHandler.Name)
		case event.Command == tccore.PLUGIN_EVENT_STOP:
			driverConfig.CoreConfig.Log.Printf("Kernel finished stopping plugin: %s\n", pluginHandler.Name)
			pluginHandler.State = 0
			safeChannelSend(pluginHandler.ConfigContext.ErrorChan,
				errors.New(pluginHandler.Name+" shutting down"),
				pluginHandler.Name+" shutting down", driverConfig.CoreConfig.Log)
			safeChannelSend(pluginHandler.ConfigContext.DfsChan,
				nil,
				pluginHandler.Name+" dfs shutting down", driverConfig.CoreConfig.Log)
			pluginHandler.PluginMod = nil
			if pluginHandler.KernelCtx != nil && pluginHandler.KernelCtx.PluginRestartChan != nil {
				go func(e tccore.KernelCmd) {
					safeChannelSend(pluginHandler.KernelCtx.PluginRestartChan, e, "plugin restart message", driverConfig.CoreConfig.Log)
				}(event)
			}
			return
		case event.Command == tccore.PLUGIN_EVENT_STATUS:
			// TODO
		default:
			// TODO
		}
	}
}

func (pluginHandler *PluginHandler) handleErrors(driverConfig *config.DriverConfig) {
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

func (pluginHandler *PluginHandler) handleDataflowStat(driverConfig *config.DriverConfig, mod *kv.Modifier, _ *system.Vault) {
	// tfmContext := &flowtccore.TrcFlowMachineContext{
	// 	Env:                       driverConfig.CoreConfig.Env,
	// 	GetAdditionalFlowsByState: flowopts.BuildOptions.GetAdditionalFlowsByState,
	// 	FlowMap:                   map[flowtccore.FlowDefinitionType]*flowtccore.TrcFlowContext{},
	// }
	// tfContext := &flowtccore.TrcFlowContext{
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
			trcDfsFlowMachineContext := &flowcore.TrcFlowMachineContext{
				Env:          driverConfig.CoreConfig.Env,
				KernelId:     pluginHandler.KernelId,
				DriverConfig: driverConfig,
			}
			flowcore.DeliverStatistic(trcDfsFlowMachineContext, nil, mod, dfstat, dfstat.Name, tenantIndexPath, tenantDFSIdPath, driverConfig.CoreConfig.Log, true)
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
				fmt.Fprintln(os.Stderr, "Recovered with stack trace of"+string(debug.Stack())+"\n")
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
	safeChannelSend(pluginHandler.ConfigContext.CmdSenderChan, tccore.KernelCmd{
		PluginName: pluginName,
		Command:    tccore.PLUGIN_EVENT_STOP,
	}, fmt.Sprintf("stop message to %s", pluginName), driverConfig.CoreConfig.Log)
	driverConfig.CoreConfig.Log.Printf("Stop message successfully sent to plugin: %s\n", pluginName)
}

func LoadPluginPath(driverConfig *config.DriverConfig, pluginToolConfig map[string]any) string {
	var deployroot string
	var service string
	ext := ".so"
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
	if !plugincoreopts.BuildOptions.IsPluginHardwired() {
		pM, err := plugin.Open(pluginPath)
		if err != nil {
			driverConfig.CoreConfig.Log.Printf("Unable to open plugin module for service: %s\n", pluginPath)
			driverConfig.CoreConfig.Log.Printf("Returned with %v\n", err)
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
	pHID := 0
	pHIDs := strings.Split(pluginHandler.Id, "-")
	if len(pHIDs) > 0 {
		id, err := strconv.Atoi(pHIDs[len(pHIDs)-1])
		if err != nil {
			driverConfig.CoreConfig.Log.Println("Setting default handler id for initial broadcasting.")
		} else {
			pHID = id
		}
	}
	if pHID != 0 {
		driverConfig.CoreConfig.Log.Printf("Initial broadcasting not supported for kernel id: %s\n", pluginHandler.Id)
		return
	}
	for {
		if len(globalPluginStatusChan) == 0 {
			break
		}
		time.Sleep(5 * time.Second)
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
	go safeChannelSend(pluginHandler.ConfigContext.ChatReceiverChan,
		&tccore.ChatMsg{
			Name:        &pluginHandler.Name,
			Query:       &[]string{"trcshtalk"},
			IsBroadcast: true,
			Response:    &response,
		}, "init broadcast sender", driverConfig.CoreConfig.Log)
}

func (pluginHandler *PluginHandler) HandleChat(driverConfig *config.DriverConfig) {
	if pluginHandler == nil || (*pluginHandler).Name != "Kernel" || len(*pluginHandler.Services) == 0 {
		driverConfig.CoreConfig.Log.Printf("Chat handling not supported for plugin: %s\n", pluginHandler.Name)
		return
	}
	if pluginHandler.ConfigContext.ChatReceiverChan == nil {
		msgReceiver := make(chan *tccore.ChatMsg)
		pluginHandler.ConfigContext.ChatReceiverChan = &msgReceiver
		pluginHandler.State = 1
	}

	if !plugincoreopts.BuildOptions.IsPluginHardwired() {
		driverConfig.CoreConfig.Log.Println("All plugins have loaded, sending broadcast message...")
		go pluginHandler.sendInitBroadcast(driverConfig)
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
				if p != nil && p.ConfigContext != nil && p.ConfigContext.ChatSenderChan != nil && *msg.Query != nil && len(*msg.Query) > 0 && (*msg.Query)[0] == p.Name {
					go safeChannelSend(p.ConfigContext.ChatSenderChan, &tccore.ChatMsg{
						Name:     msg.Name,
						KernelId: &pluginHandler.Id,
					}, "SHUTDOWN plugin chat receiver", driverConfig.CoreConfig.Log)
				}
			}
			return
		}

		for _, q := range *msg.Query {
			driverConfig.CoreConfig.Log.Println("Kernel processing chat query.")
			queryPlugin := strings.Split(q, ":")
			if len(queryPlugin) == 0 {
				driverConfig.CoreConfig.Log.Println("No plugin specified in query.")
				continue
			}
			if plugin, ok := (*pluginHandler.Services)[queryPlugin[0]]; ok && plugin.State == 1 {
				driverConfig.CoreConfig.Log.Printf("Sending query to service: %s.\n", plugin.Name)
				newMsg := &tccore.ChatMsg{
					Name:          &q,
					KernelId:      &pluginHandler.Id,
					Query:         &[]string{},
					TrcdbExchange: msg.TrcdbExchange,
					StatisticsDoc: msg.StatisticsDoc,
					HookResponse:  msg.HookResponse,
				}
				if eUtils.RefLength(msg.Name) > 0 {
					*newMsg.Query = append(*newMsg.Query, *msg.Name)
				} else {
					driverConfig.CoreConfig.Log.Printf("Warning, self identification through Name is required for all messages. Dropping query...\n")
					continue
				}
				if eUtils.RefLength(msg.Response) > 0 && eUtils.RefLength((*msg).Response) > 0 {
					newMsg.Response = (*msg).Response
				}
				if eUtils.RefLength(msg.ChatId) > 0 && eUtils.RefLength((*msg).ChatId) > 0 {
					newMsg.ChatId = (*msg).ChatId
				}
				if eUtils.RefLength(msg.RoutingId) > 0 && eUtils.RefLength((*msg).RoutingId) > 0 {
					newMsg.RoutingId = (*msg).RoutingId
				}
				if eUtils.RefLength(msg.RoutingId) == 0 {
					// If only ChatId is provided, use that for routing.
					newMsg.RoutingId = (*msg).ChatId
				}
				var chatSenderChan chan *tccore.ChatMsg
				if (*msg).IsBroadcast {
					if (*plugin.ConfigContext).ChatBroadcastChan != nil {
						newMsg.IsBroadcast = true
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
				go safeChannelSend(&chatSenderChan, newMsg, "chat sender", driverConfig.CoreConfig.Log)
			} else if eUtils.RefLength(msg.Name) > 0 && !msg.IsBroadcast {
				if plugin, ok := (*pluginHandler.Services)[*msg.Name]; ok && plugin != nil && plugin.State != 1 {
					responseError := "Service unavailable"
					if (*pluginHandler.Services)[*msg.Name].State == 0 {
						responseError = "Service initializing"
						driverConfig.CoreConfig.Log.Printf("Service initializing while processessing query from %s\n", *msg.Name)
					} else {
						driverConfig.CoreConfig.Log.Printf("Service unavailable to process query from %s\n", *msg.Name)
					}
					time.Sleep(2 * time.Second) // Give time for the plugin to start
					msg.Response = &responseError
					if plugin.ConfigContext != nil && plugin.ConfigContext.ChatSenderChan != nil {
						go safeChannelSend(plugin.ConfigContext.ChatSenderChan, msg, "unavailable service notification", driverConfig.CoreConfig.Log)
					}
				} else {
					driverConfig.CoreConfig.Log.Printf("Service unavailable to process query from %s\n", *msg.Name)
				}
				continue
			} else {
				driverConfig.CoreConfig.Log.Println("Unable to interpret message.")
			}
		}
	}
}
