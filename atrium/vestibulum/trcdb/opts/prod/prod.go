package prod

var isProd bool = false

func SetProd(prod bool) {
	isProd = prod
}

func IsProd() bool {
	return isProd
}
