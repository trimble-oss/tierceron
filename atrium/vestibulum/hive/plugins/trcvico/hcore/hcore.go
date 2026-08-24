package hcore

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/trimble-oss/tierceron-core/v2/buildopts/plugincoreopts"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	"gopkg.in/yaml.v2"
)

var (
	configContext *tccore.ConfigContext
	sender        chan error
	dfstat        *tccore.TTDINode
	llmState      = &llmRuntime{}
)

const (
	COMMON_PATH     = "./config.yml"
	llmStartAction  = "LLM_START"
	llmStopAction   = "LLM_STOP"
	llmStatusAction = "LLM_STATUS"
	llmPullAction   = "LLM_PULL"
	llmPromptAction = "LLM_PROMPT"
)

type llmRuntime struct {
	mu  sync.Mutex
	cmd *exec.Cmd
}

type llmConfig struct {
	Enabled          bool
	Provider         string
	Command          string
	APIURL           string
	ListenAddress    string
	Model            string
	ModelsPath       string
	AutoStart        bool
	PullModelOnStart bool
	StartupTimeout   time.Duration
	HealthEndpoint   string
}

type llmPromptRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	System string `json:"system,omitempty"`
	Stream bool   `json:"stream"`
}

type llmPromptResponse struct {
	Model    string `json:"model"`
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

func receiver(receive_chan chan tccore.KernelCmd) {
	for {
		event := <-receive_chan
		switch {
		case event.Command == tccore.PLUGIN_EVENT_START:
			go start(event.PluginName)
		case event.Command == tccore.PLUGIN_EVENT_STOP:
			go stop(event.PluginName)
			sender <- errors.New("vico shutting down")
			return
		case event.Command == tccore.PLUGIN_EVENT_STATUS:
			// TODO
		default:
			// TODO
		}
	}
}

func init() {
	if plugincoreopts.BuildOptions.IsPluginHardwired() {
		return
	}

	peerExe, err := os.Open("plugins/vico.so")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Vico unable to sha256 plugin")
		return
	}

	defer peerExe.Close()

	h := sha256.New()
	if _, err := io.Copy(h, peerExe); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to copy file for sha256 of plugin: %s\n", err)
		return
	}
	sha := hex.EncodeToString(h.Sum(nil))
	fmt.Fprintf(os.Stderr, "vico Version: %s\n", sha)
}

func send_dfstat() {
	if configContext == nil || configContext.DfsChan == nil || dfstat == nil {
		fmt.Fprintln(os.Stderr, "Dataflow Statistic channel not initialized properly for vico.")
		return
	}
	dfsctx, _, err := dfstat.GetDeliverStatCtx()
	if err != nil {
		configContext.Log.Println("Failed to get dataflow statistic context: ", err)
		send_err(err)
		return
	}
	tccore.SendDfStat(configContext, dfsctx, dfstat)
}

func send_err(err error) {
	if configContext == nil || configContext.ErrorChan == nil || err == nil {
		fmt.Fprintln(os.Stderr, "Failure to send error message, error channel not initialized properly for vico.")
		return
	}
	if dfstat != nil {
		dfsctx, _, err := dfstat.GetDeliverStatCtx()
		if err != nil {
			configContext.Log.Println("Failed to get dataflow statistic context: ", err)
			return
		}
		dfstat.UpdateDataFlowStatistic(dfsctx.FlowGroup,
			dfsctx.FlowName,
			dfsctx.StateName,
			dfsctx.StateCode,
			2,
			func(msg string, err error) {
				configContext.Log.Println(msg, err)
			})
		tccore.SendDfStat(configContext, dfsctx, dfstat)
	}
	*configContext.ErrorChan <- err
}

func chat_receiver(chat_receive_chan chan *tccore.ChatMsg) {
	for {
		event := <-chat_receive_chan
		switch {
		case event == nil:
			return
		case event.Name != nil && *event.Name == "SHUTDOWN":
			configContext.Log.Println("vico shutting down message receiver")
			return
		case event.Response != nil && *event.Response == "Service unavailable":
			configContext.Log.Println("Vico unable to access chat service.")
			return
		case event.ChatId != nil && *event.ChatId == "PROGRESS":
			configContext.Log.Println("Sending progress results back to kernel.")
			progressResp := "Running Vico Diagnostics..."
			event.Response = &progressResp
			*configContext.ChatSenderChan <- event
		case event.ChatId != nil:
			configContext.Log.Println("vico request")
			response := handleLLMAction(strings.ToUpper(strings.TrimSpace(*event.ChatId)), event)
			event.Response = &response
			*configContext.ChatSenderChan <- event
		default:
			configContext.Log.Println("vico received chat message")
		}
	}
}

func start(pluginName string) {
	if configContext == nil {
		fmt.Fprintln(os.Stderr, "no config context initialized for vico")
		return
	}
	config := getCommonConfig()

	if config != nil {
		dfstat = tccore.InitDataFlow(nil, configContext.ArgosId, false)
		dfstat.UpdateDataFlowStatistic("System",
			pluginName,
			"Start up",
			"1",
			1,
			func(msg string, err error) {
				configContext.Log.Println(msg, err)
			})
		send_dfstat()
		llmCfg := parseLLMConfig(config)
		if llmCfg.Enabled && llmCfg.AutoStart {
			if _, err := startLLM(llmCfg); err != nil {
				configContext.Log.Println("Failed to start configured LLM:", err)
				send_err(err)
			}
		}
	} else {
		configContext.Log.Println("Missing common configs")
		send_err(errors.New("missing common configs"))
		return
	}
}

func stop(pluginName string) {
	if configContext != nil {
		configContext.Log.Println("vico received shutdown message from kernel.")
		config := getCommonConfig()
		if config != nil {
			if msg, err := stopLLM(parseLLMConfig(config)); err != nil {
				configContext.Log.Println("Failed to stop managed LLM:", err)
			} else if msg != "" {
				configContext.Log.Println(msg)
			}
		}
	}
	if configContext != nil {
		configContext.Log.Println("Stopped server for vico.")
		dfstat.UpdateDataFlowStatistic("System",
			pluginName,
			"Shutdown",
			"0",
			1, func(msg string, err error) {
				if err != nil {
					configContext.Log.Println(tccore.SanitizeForLogging(err.Error()))
				} else {
					configContext.Log.Println(tccore.SanitizeForLogging(msg))
				}
			})
		send_dfstat()
		*configContext.CmdSenderChan <- tccore.KernelCmd{PluginName: pluginName, Command: tccore.PLUGIN_EVENT_STOP}
	}
	dfstat = nil
}

func GetConfigContext(pluginName string) *tccore.ConfigContext { return configContext }

func GetConfigPaths(pluginName string) []string {
	return []string{
		COMMON_PATH,
	}
}

func PostInit(configContext *tccore.ConfigContext) {
	configContext.Start = start
	sender = *configContext.ErrorChan
	go receiver(*configContext.CmdReceiverChan)
}

func Init(pluginName string, properties *map[string]any) {
	var err error

	configContext, err = tccore.Init(
		properties,
		tccore.TRCSHHIVEK_CERT,
		tccore.TRCSHHIVEK_KEY,
		COMMON_PATH,
		"vico",
		start,
		receiver,
		chat_receiver,
	)
	if err != nil {
		(*properties)["log"].(*log.Logger).Printf("Initialization error: %v", err)
		return
	}
	if _, ok := (*properties)[COMMON_PATH]; !ok {
		fmt.Fprintln(os.Stderr, "Missing common config components")
		return
	}
}

func GetPluginMessages(pluginName string) []string {
	return []string{}
}

func getCommonConfig() map[string]any {
	if configContext == nil || configContext.Config == nil {
		return nil
	}
	if config, ok := (*configContext.Config)[COMMON_PATH].(map[string]any); ok {
		return config
	}
	configBytes, ok := (*configContext.Config)[COMMON_PATH].([]byte)
	if !ok {
		return nil
	}
	var config map[string]any
	if err := yaml.Unmarshal(configBytes, &config); err != nil {
		return nil
	}
	(*configContext.Config)[COMMON_PATH] = config
	return config
}

func handleLLMAction(action string, event *tccore.ChatMsg) string {
	config := getCommonConfig()
	if config == nil {
		return "Vico configuration unavailable"
	}
	llmCfg := parseLLMConfig(config)
	if !llmCfg.Enabled {
		return "LLM support is disabled in Vico config"
	}

	switch action {
	case llmStartAction:
		msg, err := startLLM(llmCfg)
		if err != nil {
			return fmt.Sprintf("Failed to start %s LLM: %s", llmCfg.Provider, tccore.SanitizeForLogging(err.Error()))
		}
		return msg
	case llmStopAction:
		msg, err := stopLLM(llmCfg)
		if err != nil {
			return fmt.Sprintf("Failed to stop %s LLM: %s", llmCfg.Provider, tccore.SanitizeForLogging(err.Error()))
		}
		return msg
	case llmPullAction:
		msg, err := pullLLMModel(llmCfg)
		if err != nil {
			return fmt.Sprintf("Failed to pull model for %s: %s", llmCfg.Provider, tccore.SanitizeForLogging(err.Error()))
		}
		return msg
	case llmStatusAction:
		return llmStatus(llmCfg)
	case llmPromptAction:
		msg, err := promptLLM(llmCfg, event)
		if err != nil {
			return fmt.Sprintf("Failed to prompt %s: %s", llmCfg.Provider, tccore.SanitizeForLogging(err.Error()))
		}
		return msg
	default:
		return "Vico supports PROGRESS, LLM_START, LLM_STOP, LLM_PULL, LLM_STATUS, and LLM_PROMPT"
	}
}

func parseLLMConfig(config map[string]any) llmConfig {
	llmCfg := llmConfig{
		Enabled:          boolValue(config, "llm_enabled", false),
		Provider:         strings.ToLower(stringValue(config, "llm_provider", "ollama")),
		Command:          stringValue(config, "llm_command", "/usr/bin/ollama"),
		APIURL:           strings.TrimRight(stringValue(config, "llm_api_url", "http://127.0.0.1:11434"), "/"),
		ListenAddress:    stringValue(config, "llm_listen_address", "127.0.0.1:11434"),
		Model:            stringValue(config, "llm_model", ""),
		ModelsPath:       stringValue(config, "llm_models_path", ""),
		AutoStart:        boolValue(config, "llm_autostart", false),
		PullModelOnStart: boolValue(config, "llm_pull_model_on_start", false),
		StartupTimeout:   time.Duration(intValue(config, "llm_start_timeout_seconds", 30)) * time.Second,
		HealthEndpoint:   stringValue(config, "llm_health_endpoint", "/api/tags"),
	}
	if llmCfg.StartupTimeout <= 0 {
		llmCfg.StartupTimeout = 30 * time.Second
	}
	if !strings.HasPrefix(llmCfg.HealthEndpoint, "/") {
		llmCfg.HealthEndpoint = "/" + llmCfg.HealthEndpoint
	}
	return llmCfg
}

func startLLM(llmCfg llmConfig) (string, error) {
	if llmCfg.Provider != "ollama" {
		return "", fmt.Errorf("unsupported llm provider: %s", llmCfg.Provider)
	}
	if err := checkLLMHealth(llmCfg); err == nil {
		if llmCfg.PullModelOnStart && llmCfg.Model != "" {
			if _, err := pullLLMModel(llmCfg); err != nil {
				return "", err
			}
		}
		return llmStatus(llmCfg), nil
	}

	llmState.mu.Lock()
	if llmState.cmd == nil {
		cmd := exec.Command(llmCfg.Command, "serve")
		cmd.Env = append(os.Environ(), "OLLAMA_HOST="+llmCfg.ListenAddress)
		if llmCfg.ModelsPath != "" {
			cmd.Env = append(cmd.Env, "OLLAMA_MODELS="+llmCfg.ModelsPath)
		}
		if configContext != nil && configContext.Log != nil {
			logWriter := configContext.Log.Writer()
			cmd.Stdout = logWriter
			cmd.Stderr = logWriter
		}
		if err := cmd.Start(); err != nil {
			llmState.mu.Unlock()
			return "", err
		}
		llmState.cmd = cmd
		go waitForManagedLLM(cmd)
	}
	llmState.mu.Unlock()

	deadline := time.Now().Add(llmCfg.StartupTimeout)
	for time.Now().Before(deadline) {
		if err := checkLLMHealth(llmCfg); err == nil {
			if llmCfg.PullModelOnStart && llmCfg.Model != "" {
				if _, err := pullLLMModel(llmCfg); err != nil {
					return "", err
				}
			}
			return llmStatus(llmCfg), nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return "", fmt.Errorf("timed out waiting for %s at %s", llmCfg.Provider, llmCfg.APIURL)
}

func waitForManagedLLM(cmd *exec.Cmd) {
	err := cmd.Wait()
	llmState.mu.Lock()
	if llmState.cmd == cmd {
		llmState.cmd = nil
	}
	llmState.mu.Unlock()
	if configContext != nil && configContext.Log != nil {
		if err != nil {
			configContext.Log.Println("Managed LLM process exited:", err)
		} else {
			configContext.Log.Println("Managed LLM process exited")
		}
	}
}

func stopLLM(llmCfg llmConfig) (string, error) {
	if llmCfg.Provider != "ollama" {
		return "", fmt.Errorf("unsupported llm provider: %s", llmCfg.Provider)
	}

	llmState.mu.Lock()
	cmd := llmState.cmd
	llmState.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		if err := checkLLMHealth(llmCfg); err == nil {
			return fmt.Sprintf("%s is running but is not managed by Vico", llmCfg.Provider), nil
		}
		return fmt.Sprintf("%s is not running", llmCfg.Provider), nil
	}

	if err := cmd.Process.Kill(); err != nil {
		return "", err
	}
	return fmt.Sprintf("Stopped managed %s service", llmCfg.Provider), nil
}

func pullLLMModel(llmCfg llmConfig) (string, error) {
	if llmCfg.Provider != "ollama" {
		return "", fmt.Errorf("unsupported llm provider: %s", llmCfg.Provider)
	}
	if llmCfg.Model == "" {
		return "", errors.New("no llm_model configured")
	}

	cmd := exec.Command(llmCfg.Command, "pull", llmCfg.Model)
	cmd.Env = append(os.Environ(), "OLLAMA_HOST="+llmCfg.ListenAddress)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return fmt.Sprintf("Model %s is available", llmCfg.Model), nil
}

func llmStatus(llmCfg llmConfig) string {
	status := "down"
	if err := checkLLMHealth(llmCfg); err == nil {
		status = "ready"
	}
	managed := "unmanaged"
	llmState.mu.Lock()
	if llmState.cmd != nil {
		managed = "managed"
	}
	llmState.mu.Unlock()
	if llmCfg.Model != "" {
		return fmt.Sprintf("LLM provider=%s status=%s mode=%s api=%s model=%s", llmCfg.Provider, status, managed, llmCfg.APIURL, llmCfg.Model)
	}
	return fmt.Sprintf("LLM provider=%s status=%s mode=%s api=%s", llmCfg.Provider, status, managed, llmCfg.APIURL)
}

func promptLLM(llmCfg llmConfig, event *tccore.ChatMsg) (string, error) {
	if llmCfg.Provider != "ollama" {
		return "", fmt.Errorf("unsupported llm provider: %s", llmCfg.Provider)
	}
	if _, err := startLLM(llmCfg); err != nil {
		return "", err
	}

	req, err := buildPromptRequest(llmCfg, event)
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequest(http.MethodPost, llmCfg.APIURL+"/api/generate", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: llmCfg.StartupTimeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("generate returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var promptResp llmPromptResponse
	if err := json.NewDecoder(resp.Body).Decode(&promptResp); err != nil {
		return "", err
	}
	return promptResp.Response, nil
}

func buildPromptRequest(llmCfg llmConfig, event *tccore.ChatMsg) (*llmPromptRequest, error) {
	req := &llmPromptRequest{Model: llmCfg.Model, Stream: false}
	if event == nil {
		return nil, errors.New("missing chat message")
	}
	switch payload := event.HookResponse.(type) {
	case string:
		req.Prompt = payload
	case map[string]any:
		req.Prompt = stringValue(payload, "prompt", "")
		req.System = stringValue(payload, "system", "")
		overrideModel := stringValue(payload, "model", "")
		if overrideModel != "" {
			req.Model = overrideModel
		}
	default:
		if event.Query != nil && len(*event.Query) > 0 {
			req.Prompt = strings.Join(*event.Query, " ")
		}
	}
	if req.Model == "" {
		return nil, errors.New("no llm model configured")
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, errors.New("no prompt supplied")
	}
	return req, nil
}

func checkLLMHealth(llmCfg llmConfig) error {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(llmCfg.APIURL + llmCfg.HealthEndpoint)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("health check returned %d", resp.StatusCode)
	}
	return nil
}

func stringValue(config map[string]any, key string, fallback string) string {
	value, ok := config[key]
	if !ok || value == nil {
		return fallback
	}
	if typed, ok := value.(string); ok {
		if typed == "" {
			return fallback
		}
		return typed
	}
	return fmt.Sprint(value)
}

func boolValue(config map[string]any, key string, fallback bool) bool {
	value, ok := config[key]
	if !ok || value == nil {
		return fallback
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}
	return fallback
}

func intValue(config map[string]any, key string, fallback int) int {
	value, ok := config[key]
	if !ok || value == nil {
		return fallback
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil {
			return parsed
		}
	}
	return fallback
}
