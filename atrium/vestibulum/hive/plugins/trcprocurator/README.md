# trcprocurator

A secure HTTPS reverse proxy plugin (procurator is Latin for "proxy" or "agent acting on behalf of another").

## Features

- Listens on a configurable HTTPS port (default: 8443)
- Forwards all traffic to localhost on a configurable target port (default: 8080)
- Enforces localhost-only access via IP whitelisting
- Uses TLS 1.2+ with strong cipher suites
- Cannot forward traffic off the machine
- Graceful shutdown support

## Configuration

The plugin is configured via `config.yml`:

```yaml
listen_port: 8443  # HTTPS port to listen on
target_port: 8080  # HTTP port to forward to on localhost
```

TLS certificates are provided via the tierceron configuration system.

## Usage

```bash
# Build the plugin
cd atrium/vestibulum/hive/plugins/trcprocurator
go build -o procurator

# Run with config file
./procurator -log=./trcprocurator.log
```

## Security

- All traffic is restricted to localhost sources only (127.0.0.1 and ::1)
- Target can only be localhost, cannot forward off-machine
- TLS 1.2 minimum with strong cipher suites
- Security headers automatically added to responses
