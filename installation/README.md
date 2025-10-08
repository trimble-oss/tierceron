# Introduction
The folders herein contain templates utilized for installing an active vault instance in the cloud or on a machine (anywhere) capable of storing all your secrets.  You could skip the trccloud step if you want to install vault yourself on a machine somewhere.  This would be trivial to do.

# Prerequisites
You must have 4 basic tools installed either via the Makefile or via the traditional go install commands.  Skip to the [next](#cmdinstall) section if you intend to build everything via traditional build methods.

```
go install github.com/trimble-oss/tierceron/cmd/trcpub@latest
```

```
go install github.com/trimble-oss/tierceron/cmd/trcx@latest
```

```
go install github.com/trimble-oss/tierceron/cmd/trcinit@latest
```

```
go install github.com/trimble-oss/tierceron/cmd/trcconfig@latest
```

## Install command line build support (Optional) (#cmdinstall)
Install build support (Makefile, gcc, etc...):  
```
sudo apt-get install build-essential mingw-w64

sudo apt-get install uuid-runtime

```

Install g3n support libraries (Required for optional tools spiralis and fenestra under atrium):
```
sudo apt-get install xorg-dev libgl1-mesa-dev libopenal1 libopenal-dev libvorbis0a libvorbis-dev libvorbisfile3
```
