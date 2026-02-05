package hive

// PluginInitFunc is the signature for a plugin's Init function
type PluginInitFunc func(string, *map[string]any)

// PluginStartFunc is the signature for a plugin's Start function
type PluginStartFunc func()

// Plugin initialization callbacks registry to avoid import cycles
var (
	pluginInitCallbacks  = make(map[string]PluginInitFunc)
	pluginStartCallbacks = make(map[string]PluginStartFunc)
)

// RegisterPluginCallbacks allows plugins to register their Init and Start functions
// without creating import cycles
func RegisterPluginCallbacks(pluginName string, initFunc PluginInitFunc, startFunc PluginStartFunc) {
	if initFunc != nil {
		pluginInitCallbacks[pluginName] = initFunc
	}
	if startFunc != nil {
		pluginStartCallbacks[pluginName] = startFunc
	}
}

// CallPluginInit calls a registered plugin's Init function if it exists
func CallPluginInit(pluginName string, name string, properties *map[string]any) {
	if initFunc, ok := pluginInitCallbacks[pluginName]; ok {
		initFunc(name, properties)
	}
}

// CallPluginStart calls a registered plugin's Start function if it exists
func CallPluginStart(pluginName string) {
	if startFunc, ok := pluginStartCallbacks[pluginName]; ok {
		startFunc()
	}
}
