# trcshtalk

A Tierceron diagnostics and talkback plugin for use with [Tierceron](https://github.com/trimble-oss/tierceron), providing gRPC-based health and plugin diagnostic capabilities.

## Overview

`trcshtalk` is a Go-based plugin that coordinates diagnostics across the Tierceron ecosystem. It exposes gRPC services and talkback flows for health checks and plugin-specific diagnostics, including integrations for components such as `trcdb`, `rainier`, and `ninja`.

## Prerequisites

- Go 1.26+
- Access to a configured Tierceron or `trcshk` environment
- Protobuf and gRPC tooling if you need to regenerate SDK files
- Local configuration and TLS materials for standalone execution

## Building

Build the plugin:

```sh
make trcshtalk
```

If the protobuf contracts change, regenerate the generated files with:

```sh
make protobuf
```

## Usage

This plugin is deployed as part of the Tierceron infrastructure. For local standalone runs, the entrypoint reads `config.yml` plus certificate material from `local_config/` before starting the diagnostics service.

### trcshtalk_mode

The supported public run modes are:

- `trcshtalkback`: starts only the outbound talkback loop to the external trcshtalk system. It does not start the local gRPC server.
- `trcshtalkhubclient`: starts only a client that connects to the local hub. It does not start the remote talkback loop or the local gRPC server.
- `trcshtalkhub`: starts the remote talkback loop and the local gRPC hub service for other clients to connect to. Clients querying a server running in `trcshtalkhub` mode must provide a matching `ttb_token`.

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE) for details.