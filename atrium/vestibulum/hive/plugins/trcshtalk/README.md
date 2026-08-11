# trcshtalk

`trcshtalk` is the Tierceron diagnostics and talkback plugin used by `trcshk` and related bootstrap flows. It can expose a local gRPC diagnostics service, run an outbound talkback loop to the external trcshtalk system, or do both depending on `trcshtalk_mode`.

## trcshtalk_mode

The supported public run modes are:

- `trcshtalkback`: starts only the outbound talkback loop to the external trcshtalk system. It does not start the local gRPC server.
- `trcshtalkhubclient`: starts only a client that connects to the local hub. It does not start the remote talkback loop or the local gRPC server.
- `trcshtalkhub`: starts the remote talkback loop and the local gRPC hub service for other clients to connect to. Clients querying a server running in `trcshtalkhub` mode must provide a matching `ttb_token`.

## Build

```sh
make trcshtalk
```

## Test

```sh
go test ./ttcore/common
```