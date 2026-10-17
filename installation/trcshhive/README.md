# Introduction 
You have found the installation folder for trcsh hive installation.  The scripts folder contains a script that will create a set of plugins for use in trcsh hive technology.

# Prerequisites
This assumes the existence of a vault with tokens.  You'll need a root or unrestricted token to initialize data from here on out.  If you're a security purist, you'll already have deleted the root token at this point and will just operate with the unrestricted dev token for the steps below.
Agent *requires* trcsh to be set up prior to use.  Trcsh running on the server coordinates all activities within the agent.  In order to function, the additional feathering configurations must also be set up for each environment in which the agent operates.

# Next steps
To install the curator to support cursor management, see [trcsh-curator/README.md](trcsh-curator/README.md)
To install the cursors to support trcsh deployment workers and the special trcsh kernel, see  [trcsh-cursor/README.md](trcsh-cursor/README.md)
To install the trcsh kernel, see  [trcshk/README.md](trcshk/README.md)

# Plugin dependency boundary

Separately built plugins may import packages under
`github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/`, and shared
modules such as `tierceron-core/v2`. They must not import other packages from
the Tierceron root or atrium modules, directly or transitively. Module
requirements alone do not indicate which packages are compiled into a plugin.

Mutabilis reads restricted mappings through the shared
`tierceron-core/v2/buildopts/coreopts` contract, which the host initializes.
It must not import the host's build-tag-specific options implementation.

Fenestra still has pre-existing dependencies on the host's
`atrium/speculatio/fenestra`, `atrium/buildopts/argosyopts`, and root utility
packages. Its proposed host-capability refactor was rolled back to preserve
existing authentication and hardwired/debug behavior without adding new
Fenestra dependencies to generic kernel builds. Isolating its data loading
requires a separate approved design. The boundary checks below will report
these existing violations; Fenestra is not yet compliant.

Publish the updated Mutabilis plugin module and update consuming hosts to
select it. For local builds, use a local replacement or Go workspace;
dependency-module replacements are not inherited by consumers.

Boundary regression checks inspect all source build variants and each plugin
module's transitive package graph under default and kernel-plugin tags:

```sh
go -C atrium test ./vestibulum/hive/plugins -run TestPlugin
go -C atrium/vestibulum/hive/plugins/trcmutabilis test ./hcore -run TestGetConfigPathsUsesSharedHostOptions
```

Run platform-specific or additional production tag combinations with
`go list -deps` as well when changing their plugin builds.
