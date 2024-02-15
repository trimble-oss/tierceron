package coreopts

var supportedEndpoints = []string{
	"tierceron.com:1234",
	"tierceron.com:5678",
}

var supportedEndpointsProd = []string{
	"prodtierceron.com",
}

func GetSupportedEndpoints(prod bool) []string {
	if prod {
		return supportedEndpointsProd
	} else {
		return supportedEndpoints
	}
}
