# Introduction 
The folders herein contain stubbed implementations for select buildopts functions.  These are used to provide

fake data so that unit tests can be built for functions that use buildopts functions.  This is also how

you would extend tierceron in another project to make your own customizations of tierceron functions.

# Extending tierceron
Your structure in an extended github repository would replicate the structure of buildoptsstub (naming it buildopts instead)

# Further extending your extension of tierceron
You can further by envisioning that stubby is in yet another github repository.

# Running tests
You can run tests in the extended repository via go test on various packages that support it:

GOOS=linux GOARCH=amd64 GOCACHE=$GOCACHE go test -v github.com/trimble-oss/tierceron/pkg/capauth


