
## License

# Tierceron

## What is it? 🤔
Tierceron is a [encrypted configuration management system](https://en.wikipedia.org/wiki/Microservices) created for managing configurations and secrets used in microservices in Vault (by Hashicorp).  It is written in [Go](https://go.dev/), using Apache [Dolthub](https://github.com/dolthub/go-mysql-server) (Tierceron Flume: provides integrated flows), [G3n](https://github.com/g3n/engine) (integrated visualization), [Kubernetes](https://github.com/kubernetes) (Tierceron Shell: integrated cloud agent secure shell), and Hashicorp [Vault](https://github.com/hashicorp/vault) (data and secrets encryption).

This suite of tools provides functionality for creating, reading, and updating configurations over multiple environments (presently dev, QA, RQA, and staging).  If you have a Vault token with the right permissions for the right environment, you can read configurations for that environment.  Presently, only the root token can be used to actually create and update changes to the stored configurations (this should probably be changed).  Support has also been recently prototyped (2019 hackathon) to provide in memory configurations via a supporting shared library, dll, or dynamic library.

## Why❓
* Because Configuration Management is a pain.  I wanted to be able to switch between development and QA and any other environment with a single call for all my microservices.  With these tools, I can now do that.
* We wanted a system that worked transparently from dev -> QA -> staging -> production.
* Wanted a fun project for our interns to work on over the summer.
* Since Tierceron is written all in go, the services involved are very stable and tiny.  All our configurations are managed on an EC2 tiny up in AWS backed by an encrypted and backed up database.
* Coding in go is a dream.  If I could code an entire system in go, I would do it in a snap.

## Key Features 🔑

- This project follows a [GitFlow](https://www.atlassian.com/git/tutorials/comparing-workflows/gitflow-workflow) model for development and release.
- Encrypted configurations store in Vault backed by encrypted mysql.
- Highly stable Vault service running on t2 micro in AWS.
- Tools: 
    * trcconfig -- for reading configurations
    * trcinit -- for initializing a configuration set over multiple projects.
    * trcx -- for extracting seed data that can be managed locally separate from the configuration templates.
    * trcpub -- for publishing template changes.
    * nc.so -- for dynamically loading configurations securely in memory.
            - this has been used successfully for a java microservice to pull in configuration files and public certificates all referenced in memory.  This means there is no configuration footprint on the filesystem.
            -- switching from dev to QA in this setup simply means using a different token.

## Trusted Committers 💻
- [Joel Rieke](mailto:joel_rieke@trimble.com)
- [David Mkrtychyan](mailto:david_mkrtychyan@trimble.com)
- [Karnveer Gill](mailto:karnveer_gill@trimble.com)

## Getting started 🚀
If you are a contributor, please have a look on the [getting started](GETTING_STARTED.MD) file. Here you can check the information required and other things before providing a useful contribution.

## Contributing 🎗️ 

Contributions are always welcome, no matter how large or small. Before contributing, please read the [code of conduct](CODE_OF_CONDUCT.MD).

See [Contributing](CONTRIBUTING.MD).

## Code review 📝
Check the [code review](CODE_REVIEW.MD) information to find out how a **Pull Request** is evaluated for this project and what other coding standards you should consider when you want to contribute.

## Current effort
Create indexing pathing in vault for easy indexing of vault data by desired variable. 
