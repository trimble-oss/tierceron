package coreopts

var supportedEndpoints = []string{
	"tierceron.test:1234",
}

var supportedEndpointsProd = []string{
	"prodtierceron.test",
}

func GetSupportedEndpoints(prod bool) []string {
	if prod {
		return supportedEndpointsProd
	} else {
		return supportedEndpoints
	}
}
