package hcore

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/townsendmerino/goinfer/chat"
)

type RoutingPlan struct {
	Intent        string   `json:"intent"`
	Target        string   `json:"target"`
	Transport     string   `json:"transport"`
	LoggingAction string   `json:"logging_action,omitempty"`
	Diagnostic    string   `json:"diagnostic,omitempty"`
	QueryID       string   `json:"query_id,omitempty"`
	Data          []string `json:"data,omitempty"`
	Queries       []string `json:"queries,omitempty"`
	ChatID        string   `json:"chat_id,omitempty"`
	Supported     bool     `json:"supported"`
	Notes         []string `json:"notes,omitempty"`
}

type RoutingTarget struct {
	Target              string   `json:"target"`
	DomainHints         []string `json:"domain_hints,omitempty"`
	Transport           string   `json:"transport"`
	Diagnostic          string   `json:"diagnostic,omitempty"`
	ValidLoggingActions []string `json:"valid_logging_actions,omitempty"`
	ValidData           []string `json:"valid_data,omitempty"`
	ValidQueries        []string `json:"valid_queries,omitempty"`
	ValidChatIDs        []string `json:"valid_chat_ids,omitempty"`
	Constraints         []string `json:"constraints,omitempty"`
}

type RoutingExample struct {
	UserQuery string      `json:"user_query"`
	Output    RoutingPlan `json:"output"`
}

type RoutingCorpus struct {
	SystemPrompt string           `json:"system_prompt"`
	Targets      []RoutingTarget  `json:"targets"`
	Examples     []RoutingExample `json:"examples"`
}

type RoutingTrainingProvider interface {
	LoadCorpus(destination any, validData []string) error
	RoutingInstructions() string
	LoggingInstructions() string
	OutputFields() string
	CanonicalizeFields(fields map[string]json.RawMessage) error
	EncodeFields(fields map[string]json.RawMessage) error
	NormalizePlan(destination any) error
}

type DefaultRoutingTraining struct{}

func (DefaultRoutingTraining) LoadCorpus(destination any, _ []string) error {
	corpus, ok := destination.(*RoutingCorpus)
	if !ok || corpus == nil {
		return fmt.Errorf("invalid routing corpus destination")
	}
	corpus.SystemPrompt = "Route only requests that match a declared target contract. Mark all other requests unsupported."
	return nil
}

func (DefaultRoutingTraining) RoutingInstructions() string { return "" }

func (DefaultRoutingTraining) LoggingInstructions() string { return "" }

func (DefaultRoutingTraining) OutputFields() string {
	return "intent, target, transport, logging_action, diagnostic, query_id, data, queries, chat_id, supported, and notes"
}

func (DefaultRoutingTraining) CanonicalizeFields(map[string]json.RawMessage) error { return nil }

func (DefaultRoutingTraining) EncodeFields(map[string]json.RawMessage) error { return nil }

func (DefaultRoutingTraining) NormalizePlan(destination any) error {
	plan, ok := destination.(*RoutingPlan)
	if !ok || plan == nil {
		return fmt.Errorf("invalid routing plan destination")
	}
	return nil
}

type Router struct {
	provider  RoutingTrainingProvider
	validData []string
	trainOnce sync.Once
	trainErr  error
	corpus    *RoutingCorpus
	prompt    string
	logPrompt string
}

func NewRouter(provider RoutingTrainingProvider, validData []string) *Router {
	if provider == nil {
		provider = DefaultRoutingTraining{}
	}
	return &Router{provider: provider, validData: append([]string(nil), validData...)}
}

func (r *Router) Train() error {
	r.trainOnce.Do(func() {
		r.corpus = &RoutingCorpus{}
		if r.trainErr = r.provider.LoadCorpus(r.corpus, r.validData); r.trainErr != nil {
			return
		}
		contracts, err := json.Marshal(r.corpus.Targets)
		if err != nil {
			r.trainErr = err
			return
		}
		r.prompt = fmt.Sprintf(`You are the vico query router. %s
Return exactly one JSON object and no markdown or explanation.
Use only these output fields: %s.
Never invent field names such as support_status, status, action, plugin, or enabled.
%s
For any plugin logging request, return only intent="plugin_query", the lowercase plugin target, transport="logging_directive", logging_action="start" or "stop", and supported=true. Do not include diagnostic or query fields in logging plans.
Use only these target contracts: %s
Follow the example conversations. If no contract safely matches, return supported=false.`, r.corpus.SystemPrompt, r.provider.OutputFields(), r.provider.RoutingInstructions(), string(contracts))
		r.logPrompt = fmt.Sprintf(`You are the vico logging router. Convert the request into exactly one JSON object with exactly these five fields: intent, target, transport, logging_action, supported.
Set intent to "plugin_query", target to the lowercase plugin name from the request, transport to "logging_directive", logging_action to "start" for on/start/enable or "stop" for off/stop/disable, and supported to true.
%s
Never output support_status, status, action, plugin, enabled, markdown, or explanation.`, r.provider.LoggingInstructions())
	})
	return r.trainErr
}

func (r *Router) CorpusSize() (int, int) {
	if r.corpus == nil {
		return 0, 0
	}
	return len(r.corpus.Targets), len(r.corpus.Examples)
}

var explicitLoggingDirective = regexp.MustCompile(`(?i)^(?:@?trcshtalk\s+)?(?:(turn\s+(?:on|off)|enable|disable|start|stop)\s+(?:the\s+)?(?:logging|logs?)\s+(?:for|on|of)\s+([a-z0-9][a-z0-9_-]*)|(turn\s+(?:on|off)|enable|disable|start|stop)\s+([a-z0-9][a-z0-9_-]*)\s+(?:logging|logs?))[\s.!?]*$`)

func (r *Router) ResolveLoggingPrompt(prompt string) (string, bool) {
	matches := explicitLoggingDirective.FindStringSubmatch(strings.TrimSpace(prompt))
	if len(matches) != 5 {
		return "", false
	}
	action, target := matches[1], matches[2]
	if action == "" {
		action, target = matches[3], matches[4]
	}
	action = strings.ToLower(strings.TrimSpace(action))
	if strings.HasSuffix(action, "on") || action == "enable" || action == "start" {
		action = "start"
	} else {
		action = "stop"
	}
	encoded, err := r.MarshalPlan(RoutingPlan{Intent: "plugin_query", Target: strings.ToLower(target), Transport: "logging_directive", LoggingAction: action, Supported: true})
	return encoded, err == nil
}

func (r *Router) BuildTurns(prompt string) []chat.Turn {
	prompt = normalizeRoutingPrompt(prompt)
	turns := make([]chat.Turn, 0)
	if r.corpus != nil {
		if systemPrompt := r.promptFor(prompt); systemPrompt != "" {
			turns = append(turns, chat.Turn{Role: "system", Content: systemPrompt})
		}
		for _, example := range r.examplesFor(prompt) {
			output, err := r.MarshalPlan(example.Output)
			if err != nil {
				continue
			}
			turns = append(turns, chat.Turn{Role: "user", Content: strings.TrimSpace(example.UserQuery)}, chat.Turn{Role: "assistant", Content: output})
		}
	}
	return append(turns, chat.Turn{Role: "user", Content: prompt})
}

func (r *Router) NormalizeResponse(raw string) (string, error) {
	plan, err := r.ParsePlan(raw)
	if err != nil {
		plan = r.FallbackPlan("model returned non-canonical routing json", raw)
	}
	return r.MarshalPlan(plan)
}

func (r *Router) ParsePlan(raw string) (RoutingPlan, error) {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)
	start, end := strings.Index(cleaned, "{"), strings.LastIndex(cleaned, "}")
	if start == -1 || end == -1 || end < start {
		return RoutingPlan{}, fmt.Errorf("no json object found in model response")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(cleaned[start:end+1]), &fields); err != nil {
		return RoutingPlan{}, err
	}
	if err := r.provider.CanonicalizeFields(fields); err != nil {
		return RoutingPlan{}, err
	}
	if err := canonicalizeLoggingStatus(fields); err != nil {
		return RoutingPlan{}, err
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		return RoutingPlan{}, err
	}
	var plan RoutingPlan
	decoder := json.NewDecoder(strings.NewReader(string(encoded)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&plan); err != nil {
		return RoutingPlan{}, err
	}
	if err := r.normalizePlan(&plan); err != nil {
		return RoutingPlan{}, err
	}
	return plan, nil
}

func (r *Router) FallbackPlan(reason string, raw string) RoutingPlan {
	plan := RoutingPlan{
		Intent:    "plugin_query",
		Transport: "unresolved",
		Supported: false,
		Notes:     []string{reason},
	}
	raw = strings.TrimSpace(raw)
	if raw != "" {
		if len(raw) > 240 {
			raw = raw[:240]
		}
		plan.Notes = append(plan.Notes, fmt.Sprintf("model_response=%s", raw))
	}
	return plan
}

func canonicalizeLoggingStatus(fields map[string]json.RawMessage) error {
	statusValue, hasStatus := fields["support_status"]
	if !hasStatus {
		return nil
	}
	if _, hasAction := fields["logging_action"]; hasAction {
		return fmt.Errorf("routing plan contains both logging_action and support_status")
	}
	var status string
	if err := json.Unmarshal(statusValue, &status); err != nil {
		return fmt.Errorf("invalid support_status: %w", err)
	}
	var action string
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "on", "start", "enabled", "enable":
		action = "start"
	case "off", "stop", "disabled", "disable":
		action = "stop"
	default:
		return fmt.Errorf("unsupported support_status %q", status)
	}
	fields["logging_action"], _ = json.Marshal(action)
	fields["transport"], _ = json.Marshal("logging_directive")
	fields["supported"] = json.RawMessage("true")
	delete(fields, "support_status")
	delete(fields, "diagnostic")
	delete(fields, "query_id")
	delete(fields, "data")
	delete(fields, "queries")
	delete(fields, "chat_id")
	return nil
}

func (r *Router) MarshalPlan(plan RoutingPlan) (string, error) {
	if err := r.normalizePlan(&plan); err != nil {
		return "", err
	}
	encoded, err := json.Marshal(plan)
	if err != nil {
		return "", err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		return "", err
	}
	if err := r.provider.EncodeFields(fields); err != nil {
		return "", err
	}
	encoded, err = json.Marshal(fields)
	return string(encoded), err
}

func (r *Router) normalizePlan(plan *RoutingPlan) error {
	plan.Intent = strings.TrimSpace(plan.Intent)
	if plan.Intent == "" {
		plan.Intent = "plugin_query"
	}
	plan.Target = strings.ToLower(strings.TrimSpace(plan.Target))
	plan.Transport = strings.ToLower(strings.TrimSpace(plan.Transport))
	plan.LoggingAction = strings.ToLower(strings.TrimSpace(plan.LoggingAction))
	plan.Diagnostic = strings.ToUpper(strings.TrimSpace(plan.Diagnostic))
	plan.QueryID = strings.TrimSpace(plan.QueryID)
	plan.ChatID = strings.TrimSpace(plan.ChatID)
	plan.Data = compactRoutingValues(plan.Data)
	plan.Queries = compactRoutingValues(plan.Queries)
	plan.Notes = compactRoutingValues(plan.Notes)
	if plan.LoggingAction != "" {
		plan.Transport = "logging_directive"
	}
	if err := r.provider.NormalizePlan(plan); err != nil {
		return err
	}
	if plan.Target == "" {
		plan.Supported = false
	}
	if r.corpus != nil && plan.LoggingAction == "" && plan.Target != "" && !r.hasTarget(plan.Target) {
		plan.Supported = false
	}
	if plan.Transport == "" && !plan.Supported {
		plan.Transport = "unresolved"
	}
	plan.Notes = compactRoutingValues(plan.Notes)
	return nil
}

func (r *Router) hasTarget(target string) bool {
	if r.corpus == nil {
		return false
	}
	for _, contract := range r.corpus.Targets {
		if strings.EqualFold(strings.TrimSpace(contract.Target), target) {
			return true
		}
	}
	return false
}

func (r *Router) examplesFor(prompt string) []RoutingExample {
	if r.corpus == nil {
		return nil
	}
	normalized := strings.ToLower(strings.TrimSpace(prompt))
	ordered := make([]RoutingExample, 0, len(r.corpus.Examples))
	matched := make([]RoutingExample, 0, 1)
	for _, example := range r.corpus.Examples {
		if isRoutingLoggingPrompt(normalized) != (example.Output.LoggingAction != "") {
			continue
		}
		if example.Output.Target != "" && strings.Contains(normalized, example.Output.Target) && strings.Contains(strings.ToLower(example.UserQuery), example.Output.Target) {
			matched = append(matched, example)
			continue
		}
		ordered = append(ordered, example)
	}
	return append(ordered, matched...)
}

func (r *Router) promptFor(prompt string) string {
	if isRoutingLoggingPrompt(prompt) {
		return strings.TrimSpace(r.logPrompt)
	}
	return strings.TrimSpace(r.prompt)
}

func normalizeRoutingPrompt(prompt string) string {
	fields := strings.Fields(strings.TrimSpace(prompt))
	if len(fields) > 0 && strings.HasPrefix(fields[0], "@") {
		fields = fields[1:]
	}
	return strings.Join(fields, " ")
}

func isRoutingLoggingPrompt(prompt string) bool {
	for _, field := range strings.Fields(strings.ToLower(prompt)) {
		field = strings.Trim(field, " ,.!?:;()[]{}")
		if field == "log" || field == "logs" || field == "logging" {
			return true
		}
	}
	return false
}

func compactRoutingValues(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
