# Hardwired Procurator Plugin

## Overview

The procurator plugin can be compiled directly into trcsh (hardwired) instead of being loaded as a dynamic plugin. This is useful for:
- **Windows deployment** (Go plugins not supported on Windows)
- **Simplified deployment** (no separate .so file needed)
- **Debugging** (easier to debug with integrated code)

## Architecture

The hardwired plugin system uses build tags to conditionally compile plugin code:

- **`tcore/`** - Plugin implementation (used for both dynamic loading and hardwired builds)
- **`buildopts/pluginopts/pluginopts_hardwired.go`** - Registration that imports tcore directly

The procurator plugin uses the same `tcore` package for both dynamic and hardwired builds, keeping the code simple and maintainable.

## Building trcsh with Hardwired Procurator

### 1. Build with hardwired tag

```bash
cd /path/to/tierceron/atrium/vestibulum/hive/trcsh
go build -tags "hardwired" -o trcsh
```

Or for Windows:
```bash
GOOS=windows GOARCH=amd64 go build -tags "hardwired" -o trcsh.exe
```

### 2. Configuration

The procurator plugin is referenced by name in deployment configuration. When trcsh starts with hardwired build:

1. Checks if plugin "procurator" is requested in deployment config
2. Calls `pluginopts.BuildOptions.GetConfigPaths("procurator")` → routes to `hcore.GetConfigPaths()`
3. Calls `pluginopts.BuildOptions.Init("procurator", &properties)` → routes to `hcore.Init()`
4. Plugin starts using the same code as dynamic plugin, but compiled in

### 3. Deployment Configuration

In your vault/configuration, specify procurator as the plugin service:

```yaml
trcplugin: procurator
```

The kernel will detect it's a hardwired build and use the compiled-in code instead of trying to load `procurator.so`.

## How It Works

The key is in `buildopts/pluginopts/pluginopts_hardwired.go`:

```go
//go:build hardwired
// +build hardwired

import (
    pcore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcprocurator/tcore"
)

func GetConfigPaths(pluginName string) []string {
    switch pluginName {
    case "procurator":
        return pcore.GetConfigPaths(pluginName)
    // ... other plugins
    }
    return []string{}
}

func Init(pluginName string, properties *map[string]any) {
    switch pluginName {
    case "procurator":
        pcore.Init(pluginName, properties)
    // ... other plugins
    }
}
```

When built without `-tags hardwired`, these functions return empty/no-op, and plugins are loaded dynamically.

## Code Organization

```
trcprocurator/
├── procurator.go          # Main entry point (for standalone/dynamic plugin)
├── tcore/                 # Plugin implementation (shared for dynamic & hardwired)
│   └── tcore.go
├── trc_templates/         # Configuration templates
├── scripts/               # Deployment scripts
└── Makefile              # Build targets
```

## Benefits

- **Cross-platform**: Works on Windows where Go plugins don't
- **Single binary**: No need to distribute separate .so files
- **Easier deployment**: Just one executable
- **Better debugging**: Standard Go debugging tools work
- **Performance**: Slight startup improvement (no dynamic loading)

## Trade-offs

- **Larger binary**: All plugins compiled in
- **Rebuild required**: Changes to plugins require rebuilding trcsh
- **Build tags**: Must remember to use `-tags hardwired`
