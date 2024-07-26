package coreopts

var supportedEndpoints = [][]string{
	{
		"tierceron.test:1234",
		"127.0.0.1",
	},
}

var supportedEndpointsProd = [][]string{
	{
		"prodtierceron.test",
		"n/a",
	},
}

func GetSupportedEndpoints(prod bool) [][]string {
	if prod {
		return supportedEndpointsProd
	} else {
		return supportedEndpoints
	}
}
