package hcore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/townsendmerino/goinfer/chat"
	"github.com/townsendmerino/goinfer/decoder"
	"github.com/townsendmerino/goinfer/tokenizer"
	"github.com/trimble-oss/tierceron-core/v2/buildopts/plugincoreopts"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	"gopkg.in/yaml.v2"
)

var (
	configContext *tccore.ConfigContext
	sender        chan error
	dfstat        *tccore.TTDINode
	localModel    *localModelRuntime
)

const (
	COMMON_PATH = "./config.yml"
)

type pluginConfig struct {
	LocalModelPath        string  `yaml:"local_model_path"`
	LocalModelBackend     string  `yaml:"local_model_backend"`
	LocalModelMaxTokens   int     `yaml:"local_model_max_tokens"`
	LocalModelTemperature float64 `yaml:"local_model_temperature"`
	LocalModelTopK        int     `yaml:"local_model_top_k"`
	LocalModelTopP        float64 `yaml:"local_model_top_p"`
}

type localModelRuntime struct {
	maxTokens int
	sampling  decoder.SamplingParams
	model     *decoder.Model
	tokenizer *tokenizer.Tokenizer
	template  *chat.Template
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

func loadPluginConfig() (*pluginConfig, error) {
	if configContext == nil || configContext.Config == nil {
		return nil, errors.New("missing config context")
	}
	rawConfig, ok := (*configContext.Config)[COMMON_PATH]
	if !ok || rawConfig == nil {
		return nil, errors.New("missing common configs")
	}

	var cfg pluginConfig
	switch typedConfig := rawConfig.(type) {
	case map[string]any:
		configBytes, err := yaml.Marshal(typedConfig)
		if err != nil {
			return nil, err
		}
		if err := yaml.Unmarshal(configBytes, &cfg); err != nil {
			return nil, err
		}
	case *map[string]any:
		if typedConfig == nil {
			return nil, errors.New("missing common configs")
		}
		configBytes, err := yaml.Marshal(*typedConfig)
		if err != nil {
			return nil, err
		}
		if err := yaml.Unmarshal(configBytes, &cfg); err != nil {
			return nil, err
		}
	case []byte:
		if err := yaml.Unmarshal(typedConfig, &cfg); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported common config type %T", rawConfig)
	}

	return &cfg, nil
}

func loadTokenizer(modelPath string) (*tokenizer.Tokenizer, error) {
	if strings.HasSuffix(strings.ToLower(strings.TrimSpace(modelPath)), ".gguf") {
		return tokenizer.LoadGGUF(modelPath)
	}
	return tokenizer.Load(modelPath)
}

func initLocalModel(cfg *pluginConfig) error {
	closeLocalModel()
	if cfg == nil {
		return errors.New("missing local model config")
	}
	if strings.TrimSpace(cfg.LocalModelPath) == "" {
		return errors.New("local_model_path is empty")
	}

	modelOptions := decoder.Options{Backend: strings.TrimSpace(cfg.LocalModelBackend)}
	if err := modelOptions.Validate(); err != nil {
		return err
	}

	model, err := decoder.Load(cfg.LocalModelPath, modelOptions)
	if err != nil {
		return err
	}

	tok, err := loadTokenizer(cfg.LocalModelPath)
	if err != nil {
		model.Close()
		return err
	}

	var template *chat.Template
	if detectedTemplate, err := chat.Detect(chat.Meta{ChatTemplate: tok.ChatTemplate(), HasToken: tok.Has}); err == nil {
		template = detectedTemplate
	} else if !errors.Is(err, chat.ErrUnknownTemplate) {
		configContext.Log.Printf("vico local model chat template detection failed: %s\n", tccore.SanitizeForLogging(err.Error()))
	}

	maxTokens := cfg.LocalModelMaxTokens
	if maxTokens <= 0 {
		maxTokens = 256
	}

	localModel = &localModelRuntime{
		maxTokens: maxTokens,
		sampling: decoder.SamplingParams{
			Temperature: cfg.LocalModelTemperature,
			TopK:        cfg.LocalModelTopK,
			TopP:        cfg.LocalModelTopP,
		},
		model:     model,
		tokenizer: tok,
		template:  template,
	}

	return nil
}

func closeLocalModel() {
	if localModel != nil && localModel.model != nil {
		if err := localModel.model.Close(); err != nil && configContext != nil && configContext.Log != nil {
			configContext.Log.Printf("vico local model shutdown failed: %s\n", tccore.SanitizeForLogging(err.Error()))
		}
	}
	localModel = nil
}

func extractPrompt(event *tccore.ChatMsg) string {
	if event == nil {
		return ""
	}
	if event.Response != nil && strings.TrimSpace(*event.Response) != "" && *event.Response != "Service unavailable" {
		return strings.TrimSpace(*event.Response)
	}
	switch promptValue := event.HookResponse.(type) {
	case string:
		return strings.TrimSpace(promptValue)
	case []string:
		return strings.TrimSpace(strings.Join(promptValue, "\n"))
	case map[string]any:
		if prompt, ok := promptValue["prompt"].(string); ok {
			return strings.TrimSpace(prompt)
		}
	}
	return ""
}

func encodePrompt(runtime *localModelRuntime, prompt string) ([]int, error) {
	if runtime == nil || runtime.tokenizer == nil {
		return nil, errors.New("vico local model is not initialized")
	}
	if runtime.template != nil {
		return runtime.tokenizer.EncodeSegments(runtime.template.RenderSegments("", []chat.Turn{{Role: "user", Content: prompt}}), false)
	}
	return runtime.tokenizer.Encode(prompt, true)
}

func queryLocalModel(prompt string) (string, error) {
	if localModel == nil || localModel.model == nil || localModel.tokenizer == nil {
		return "", errors.New("vico local model is not configured")
	}
	encodedPrompt, err := encodePrompt(localModel, prompt)
	if err != nil {
		return "", err
	}
	responseChan, generation := localModel.model.Generate(context.Background(), encodedPrompt, localModel.maxTokens, localModel.sampling)
	responseTokens := make([]int, 0, localModel.maxTokens)
	for tokenID := range responseChan {
		responseTokens = append(responseTokens, tokenID)
	}
	if generation != nil && generation.Err() != nil {
		return "", generation.Err()
	}
	response, err := localModel.tokenizer.DecodeContinuation(responseTokens)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(response), nil
}

func chat_receiver(chat_receive_chan chan *tccore.ChatMsg) {
	for {
		event := <-chat_receive_chan
		switch {
		case event == nil:
			continue
		case event.Name != nil && *event.Name == "SHUTDOWN":
			configContext.Log.Println("vico shutting down message receiver")
			return
		case event.Response != nil && *(*event).Response == "Service unavailable":
			configContext.Log.Println("Vico unable to access chat service.")
			return
		case event.ChatId != nil && (*event).ChatId != nil && *event.ChatId == "PROGRESS":
			configContext.Log.Println("Sending progress results back to kernel.")
			progressResp := "Running Vico Diagnostics..."
			(*event).Response = &progressResp
			*configContext.ChatSenderChan <- event
		case extractPrompt(event) != "":
			prompt := extractPrompt(event)
			configContext.Log.Println("vico local model request")
			response, err := queryLocalModel(prompt)
			if err != nil {
				configContext.Log.Printf("vico local model request failed: %s\n", tccore.SanitizeForLogging(err.Error()))
				response = "Vico local model unavailable."
			}
			(*event).Response = &response
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
	cfg, err := loadPluginConfig()
	if err != nil {
		configContext.Log.Println("Missing common configs")
		send_err(err)
		return
	}

	if cfg != nil {
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
		if err := initLocalModel(cfg); err != nil {
			configContext.Log.Printf("vico local model startup failed: %s\n", tccore.SanitizeForLogging(err.Error()))
			send_err(err)
		} else if localModel != nil {
			configContext.Log.Println("vico local model ready")
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
	}
	closeLocalModel()
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
