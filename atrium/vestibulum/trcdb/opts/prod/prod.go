package prod

var isProd bool = false

func SetProd(prod bool) {
	isProd = prod
}

func IsProd() bool {
	return isProd
}

func IsStagingProd(env string) bool {
	if env == "staging" || env == "prod" {
		return true
	}
	return false
}
