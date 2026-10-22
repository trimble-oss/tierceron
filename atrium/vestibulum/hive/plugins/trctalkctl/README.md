# Trctalkctl

TrcshZ command plugin registered as `talkctl`. Include `trcshtalk` and `talkctl` in
the `.trcshrc` deployments line. After `su`, run
`@talkctl <command...>` (or `talkctl <command...>`). Replies, including multiline
results and transport errors, return to the initiating shell command.

The plugin refuses non-TrcshZ and Kubernetes startup. It accepts only the shell
command envelope, not broadcasts or replies. This is an operational shell gate,
not a separate cryptographic identity for elevated users or protection against
a malicious plugin in the same process.

## Transport

The local TrcshTalk plugin must run in `trcshtalkhubclient` mode with its normal
hub name/port, talkback token, service-client root CA, kernel ID, and nonempty
`deployments` registration. Talkctl uses the client registered by that running
plugin; it does not load its own secrets or certificates.

The tenant-specific extension is named `pendentive-talkctl`.

The hub command RPC uses the existing DiagnosticRequest/DiagnosticResponse
messages. The hub invokes Vico's routing executor with the incoming request
context, retaining the client registration metadata used by authorization.
For a standalone TrcshZ kernel, its kernel ID is also its registration group.

Commands have a 90-second client deadline. The shell waits up to two minutes.

## Build

Build with the same Go toolchain, shared dependency versions, and build tags as
the target TrcshZ host and its TrcshTalk plugin. The local TrcshTalk module replace
is intentional: these plugins require the new shared command transport API.
Rebuild the shell, TrcshTalk, and Vico hub with the corresponding source changes.

From this directory, with `TRCSH_BUILD_TAGS` set to the host's build tags:

```sh
mkdir -p bin
CGO_ENABLED=1 go build -tags="$TRCSH_BUILD_TAGS" -buildmode=plugin -o bin/talkctl.so .
```

CGO is enabled for Go plugin linking; trctalkctl itself adds no native dependency.
DockerfilePlugin packages the resulting artifact using this directory as its
build context.

## Management

- `scripts/servicedefine.sh`: service definitions for dev and QA, using the
  existing VAULT_ADDR/VAULT_TOKEN environment variables. Run with `sh`.
- `deploy/deploy.trc`: bootstrap entry point.
- `trc_templates/Hive/PluginTalkctl/deploy/deploy.trc.tmpl`: stop, deploy, start.
- `trc_templates/Hive/PluginTalkctlBuild/deploy/deploy.trc.tmpl`: build/push image.
- `trc_templates/Hive/PluginTalkctlBuild/deploy/deploytag.trc.tmpl`: release alias.

The plugin is `talkctl`, service `Hive/PluginTalkctl`, type `trcshcmdtoolplugin`, with
bundle `talkctl.so` deployed under `/usr/local/trcshk/plugins`. The deploy template's
`env` value defaults to `dev`; set it for each target environment. Build templates
use the usual TAG and RELEASE values. No application config template is needed:
trctalkctl uses the existing TrcshTalk configuration. Registration is operator-run.

## Checks

Focused tests live in `hcore`, the sibling `trctrcsh/shell` package, and
`trcshtalk/ttcore/common`. The parent plugin dependency-boundary tests discover
this module automatically. Build and test commands are operator-run.