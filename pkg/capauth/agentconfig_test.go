package capauth

import (
	"strings"
	"testing"

	prod "github.com/trimble-oss/tierceron-core/v2/prod"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	coreoptsloader "github.com/trimble-oss/tierceron/buildoptsstub/coreopts"
)

type ValidateVhostInverseFunc func(prod bool) []string

func TestValidateVhostInverseProd(t *testing.T) {
	// Test case 1
	prod.SetProd(true)
	coreopts.NewOptionsBuilder(coreoptsloader.LoadOptions())
	host := "prodtierceron.test"
	protocol := ""

	err := ValidateVhostInverse(host, protocol, true, false)
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}

	err = ValidateVhostInverse(host, protocol, false, false)
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}

	host = "https://prodtierceron.test"
	protocol = "https://"

	err = ValidateVhostInverse(host, protocol, true, false)
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}

	err = ValidateVhostInverse(host, protocol, false, false)
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}

}

func TestValidateVhostInverseNonProd(t *testing.T) {
	// Test case - Not prod, empty protocol, inverse, valid endpoint + ip
	prod.SetProd(false)
	coreopts.NewOptionsBuilder(coreoptsloader.LoadOptions())
	server := "tierceron.test:1234"

	validVhostInverseErr := ValidateVhostInverse(server, "", true, false)

	if validVhostInverseErr != nil {
		t.Fatalf("Expected nil, got %v", validVhostInverseErr)
	}
}

func TestValidateVhostInverseNonProd_DiffPort(t *testing.T) {
	// Test case - Not prod, empty protocol, inverse, valid ip, different port
	prod.SetProd(false)
	coreopts.NewOptionsBuilder(coreoptsloader.LoadOptions())
	server := "tierceron.test:5678"

	validVhostInverseErr := ValidateVhostInverse(server, "", true, false)
	if validVhostInverseErr == nil {
		t.Fatalf("Expected Bad host error, got %v", validVhostInverseErr)
	}
}

func TestValidateVhostNotInverseNonProd_DiffPort(t *testing.T) {
	// Test case - Not prod, empty protocol, not inverse, valid ip, different port
	prod.SetProd(false)
	coreopts.NewOptionsBuilder(coreoptsloader.LoadOptions())
	server := "tierceron.test:5678"

	validVhostInverseErr := ValidateVhostInverse(server, "", false, false)
	if validVhostInverseErr == nil {
		t.Fatalf("Expected Bad host error, got %v", validVhostInverseErr)
	}
}

func TestValidateVhostInverseNonProd_NonPort(t *testing.T) {
	// Test case - Not prod, empty protocol, inverse, valid ip, different port, not port check
	prod.SetProd(false)
	coreopts.NewOptionsBuilder(coreoptsloader.LoadOptions())
	server := "tierceron.test:5678"
	validVhostInverseErr := ValidateVhostInverse(server, "", true, true)
	if validVhostInverseErr != nil {
		t.Fatalf("Expected nil, got %v", validVhostInverseErr)
	}
}

func TestValidateVhostNotInverseNonProd_NonPort(t *testing.T) {
	// Test case - Not prod, empty protocol, not inverse, valid ip, different port, not port check
	prod.SetProd(false)
	coreopts.NewOptionsBuilder(coreoptsloader.LoadOptions())
	server := "tierceron.test:5678"
	validVhostInverseErr := ValidateVhost(server, "", true)
	if validVhostInverseErr != nil {
		t.Fatalf("Expected nil, got %v", validVhostInverseErr)
	}
}

func TestValidateVhostInverseNonProd_NonEmptyProto(t *testing.T) {
	// Test case - Not prod, https as protocol, inverse
	prod.SetProd(false)
	coreopts.NewOptionsBuilder(coreoptsloader.LoadOptions())
	server := "https://tierceron.test:1234"

	validVhostInverseErr := ValidateVhostInverse(server, "https", true, false)

	if validVhostInverseErr != nil {
		t.Fatalf("Expected nil, got %v", validVhostInverseErr)
	}
}

func TestValidateVhostInverseNonProd_NonEmptyProto_DiffPort(t *testing.T) {
	// Test case - Not prod, https as protocol, inverse, invalid port
	prod.SetProd(false)
	coreopts.NewOptionsBuilder(coreoptsloader.LoadOptions())
	server := "https://tierceron.test:5678"

	validVhostInverseErr := ValidateVhostInverse(server, "https", true, false)

	if validVhostInverseErr == nil {
		t.Fatalf("Expected Bad host error, got %v", validVhostInverseErr)
	}
}

func TestValidateVhostInverseNonProd_NonEmptyProto_NonPort(t *testing.T) {
	// Test case - Not prod, https as protocol, inverse, invalid port but not port check
	prod.SetProd(false)
	coreopts.NewOptionsBuilder(coreoptsloader.LoadOptions())
	server := "https://tierceron.test:5678"

	validVhostInverseErr := ValidateVhostInverse(server, "https", true, true)

	if validVhostInverseErr != nil {
		t.Fatalf("Expected nil, got %v", validVhostInverseErr)
	}
}

func TestValidateVhostPortNonProd(t *testing.T) {
	// Test case - Not prod, protocol is https, not inverse, valid endpt
	prod.SetProd(false)
	coreopts.NewOptionsBuilder(coreoptsloader.LoadOptions())
	server := "https://tierceron.test:1234"

	validVhostInverseErr := ValidateVhostInverse(server, "https", false, false)

	if validVhostInverseErr != nil {
		t.Fatalf("Expected nil, got %v", validVhostInverseErr)
	}
}

func TestInvalidValidateVhostPortNonProd(t *testing.T) {
	// Test case - Not prod, Protocol is https, invalid endpoint, not inverse
	prod.SetProd(false)
	coreopts.NewOptionsBuilder(coreoptsloader.LoadOptions())
	server := "https://tierceron.bar:1234"

	validVhostInverseErr := ValidateVhostInverse(server, "https", false, false)

	if validVhostInverseErr == nil || !strings.HasPrefix(validVhostInverseErr.Error(), "Bad host") {
		t.Fatal("Expected a bad host error, got nil")
	}
}

func TestValidateVhostMissingProtoProd_EmptyAddr(t *testing.T) {
	// Test case - prod, empty host, protocol is https://, not inverse
	prod.SetProd(true)
	address := ""
	coreopts.NewOptionsBuilder(coreoptsloader.LoadOptions())
	validateVhostErr := ValidateVhost(address, "https://", false)
	if validateVhostErr != nil {
		if !strings.HasPrefix(validateVhostErr.Error(), "missing required protocol") {
			t.Fatalf("Expected missing required protocol error, got %v", validateVhostErr)
		}
	} else {
		t.Fatal("Expected error")
	}
}

func TestValidateVhostMissingProtoProd(t *testing.T) {
	// Test case - prod, protocol is not prefix
	prod.SetProd(true)
	address := "http://prodtierceron.test"
	coreopts.NewOptionsBuilder(coreoptsloader.LoadOptions())
	validateVhostErr := ValidateVhost(address, "https://", false)
	if validateVhostErr != nil {
		if !strings.HasPrefix(validateVhostErr.Error(), "missing required protocol") {
			t.Fatalf("Expected missing required protocol error, got %v", validateVhostErr)
		}
	} else {
		t.Fatal("Expected error")
	}
}

func TestValidateVhostMissingProtoNonProd_EmptyAddr(t *testing.T) {
	// Test case - Not prod, protocol is not prefix
	prod.SetProd(false)
	address := ""
	coreopts.NewOptionsBuilder(coreoptsloader.LoadOptions())
	validateVhostErr := ValidateVhost(address, "https://", false)
	if validateVhostErr != nil {
		if !strings.HasPrefix(validateVhostErr.Error(), "missing required protocol") {
			t.Fatalf("Expected missing required protocol error, got %v", validateVhostErr)
		}
	} else {
		t.Fatal("Expected error")
	}
}

func TestValidateVhostMissingProtoNonProd(t *testing.T) {
	// Test case - not prod, protocol is not prefix of host
	prod.SetProd(false)
	address := "http://tierceron.test"
	coreopts.NewOptionsBuilder(coreoptsloader.LoadOptions())
	validateVhostErr := ValidateVhost(address, "https://", false)
	if validateVhostErr != nil {
		if !strings.HasPrefix(validateVhostErr.Error(), "missing required protocol") {
			t.Fatalf("Expected missing required protocol error, got %v", validateVhostErr)
		}
	} else {
		t.Fatal("Expected error")
	}
}

func TestValidateVhostProdBadHost(t *testing.T) {
	// Test case - prod, not supported endpoint, protocol is prefix
	prod.SetProd(true)
	coreopts.NewOptionsBuilder(coreoptsloader.LoadOptions())
	address := "https://tierceron.test"
	validateVhostErr := ValidateVhost(address, "https://", false)
	if validateVhostErr != nil {
		if !strings.HasPrefix(validateVhostErr.Error(), "Bad host:") {
			t.Fatalf("Expected Bad host error, got %v", validateVhostErr)
		}
	} else {
		t.Fatal("Expected error")
	}
}

func TestValidateVhostProd(t *testing.T) {
	// Test case - prod, valid endpoint, protocol is prefix
	prod.SetProd(true)
	coreopts.NewOptionsBuilder(coreoptsloader.LoadOptions())
	address := "https://prodtierceron.test"
	validateVhostErr := ValidateVhost(address, "https://", false)
	if validateVhostErr != nil {
		t.Fatal("Expected no error")
	}
}

func TestValidateVhostProd_InsecureCheck(t *testing.T) {
	// Test case - prod, valid endpoint, protocol is prefix, attempting http instead of https
	prod.SetProd(true)
	coreopts.NewOptionsBuilder(coreoptsloader.LoadOptions())
	address := "http://prodtierceron.test"
	validateVhostErr := ValidateVhost(address, "http://", false)
	if validateVhostErr == nil {
		t.Fatal("Expected error")
	}
}

func TestValidateVhostProd_InsecureCheck_Inverse(t *testing.T) {
	// Test case - prod, inverse, valid endpoint, protocol is prefix, attempting http instead of https
	prod.SetProd(true)
	coreopts.NewOptionsBuilder(coreoptsloader.LoadOptions())
	address := "http://prodtierceron.test"
	validateVhostErr := ValidateVhostInverse(address, "http://", true, false)
	if validateVhostErr == nil {
		t.Fatal("Expected error")
	}
}

func TestValidateVhostNonProd(t *testing.T) {
	prod.SetProd(false)
	coreopts.NewOptionsBuilder(coreoptsloader.LoadOptions())
	address := "https://tierceron.test"
	validateVhostErr := ValidateVhost(address, "https://", false)
	if validateVhostErr != nil {
		t.Fatal("Expected no error")
	}
}

func TestValidateVhostNonProd_Insecure(t *testing.T) {
	// Test case - not prod, valid endpoint, protocol is prefix, http not https
	prod.SetProd(false)
	coreopts.NewOptionsBuilder(coreoptsloader.LoadOptions())
	address := "http://tierceron.test"
	validateVhostErr := ValidateVhost(address, "http://", false)
	if validateVhostErr == nil {
		t.Fatal("Expected an error")
	}
}

func TestValidateVhostNonProd_Insecure_Inverse(t *testing.T) {
	// Test case - not prod, inverse, valid endpoint, protocol is prefix, http not https
	prod.SetProd(false)
	coreopts.NewOptionsBuilder(coreoptsloader.LoadOptions())
	address := "http://tierceron.test"
	validateVhostErr := ValidateVhostInverse(address, "http://", true, false)
	if validateVhostErr == nil {
		t.Fatal("Expected an error")
	}
}
