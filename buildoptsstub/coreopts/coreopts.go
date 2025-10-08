package coreopts

import (
	bcore "github.com/trimble-oss/tierceron/buildoptsstub/stubby/coreopts"
)

func GetSupportedEndpoints(prod bool) [][]string {
	return bcore.GetSupportedEndpoints(prod)
}
