package capauth

import (
	"strings"
	"testing"

	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcdb/opts/prod"
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

	err := ValidateVhostInverse(host, protocol, true)
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}

	host = "https://prodtierceron.test"
	protocol = "https://"

	err = ValidateVhostInverse(host, protocol, true)
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}

	err = ValidateVhostInverse(host, protocol, false)
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}

}

func TestValidateVhostInverseNonProd(t *testing.T) {
	// Test case 1
	prod.SetProd(false)
	coreopts.NewOptionsBuilder(coreoptsloader.LoadOptions())
	server := "tierceron.test:1234"

	validVhostInverseErr := ValidateVhostInverse(server, "", true)

	if validVhostInverseErr != nil {
		t.Fatalf("Expected nil, got %v", validVhostInverseErr)
	}
}

func TestValidateVhostInverseNonProdDrone(t *testing.T) {
	// Test case 1
	prod.SetProd(false)
	drone := true
	coreopts.NewOptionsBuilder(coreoptsloader.LoadOptions())
	server := "tierceron.test:1234"

	validVhostInverseErr := ValidateVhostInverse(server, "", true, &drone)

	if validVhostInverseErr != nil {
		t.Fatalf("Expected nil, got %v", validVhostInverseErr)
	}
}

func TestValidateVhostPortNonProd(t *testing.T) {
	// Test case 1
	prod.SetProd(false)
	coreopts.NewOptionsBuilder(coreoptsloader.LoadOptions())
	server := "https://tierceron.test:1234"

	validVhostInverseErr := ValidateVhostInverse(server, "https", false)

	if validVhostInverseErr != nil {
		t.Fatalf("Expected nil, got %v", validVhostInverseErr)
	}
}

func TestValidateVhostPortNonProdDrone(t *testing.T) {
	// Test case 1
	prod.SetProd(false)
	coreopts.NewOptionsBuilder(coreoptsloader.LoadOptions())
	server := "https://tierceron.test:1234"
	drone := true
	validVhostInverseErr := ValidateVhostInverse(server, "https", false, &drone)

	if validVhostInverseErr != nil {
		t.Fatalf("Expected nil, got %v", validVhostInverseErr)
	}
}

func TestInvalidValidateVhostPortNonProd(t *testing.T) {
	// Test case 1
	prod.SetProd(false)
	coreopts.NewOptionsBuilder(coreoptsloader.LoadOptions())
	server := "https://tierceron.bar:1234"

	validVhostInverseErr := ValidateVhostInverse(server, "https", false)

	if validVhostInverseErr == nil || !strings.HasPrefix(validVhostInverseErr.Error(), "Bad host") {
		t.Fatal("Expected a bad host error, got nil")
	}
}

func TestInvalidValidateVhostPortNonProdDrone(t *testing.T) {
	// Test case 1
	prod.SetProd(false)
	coreopts.NewOptionsBuilder(coreoptsloader.LoadOptions())
	server := "https://tierceron.bar:1234"
	drone := true

	validVhostInverseErr := ValidateVhostInverse(server, "https", false, &drone)

	if validVhostInverseErr == nil || !strings.HasPrefix(validVhostInverseErr.Error(), "Bad host") {
		t.Fatal("Expected a bad host error, got nil")
	}
}

func TestValidateVhostMissingProtoProd_EmptyAddr(t *testing.T) {
	prod.SetProd(true)
	address := ""
	coreopts.NewOptionsBuilder(coreoptsloader.LoadOptions())
	validateVhostErr := ValidateVhost(address, "https://")
	if validateVhostErr != nil {
		if !strings.HasPrefix(validateVhostErr.Error(), "missing required protocol") {
			t.Fatalf("Expected missing required protocol error, got %v", validateVhostErr)
		}
	} else {
		t.Fatal("Expected error")
	}
}

func TestValidateVhostMissingProtoProd(t *testing.T) {
	prod.SetProd(true)
	address := "http://prodtierceron.test"
	coreopts.NewOptionsBuilder(coreoptsloader.LoadOptions())
	validateVhostErr := ValidateVhost(address, "https://")
	if validateVhostErr != nil {
		if !strings.HasPrefix(validateVhostErr.Error(), "missing required protocol") {
			t.Fatalf("Expected missing required protocol error, got %v", validateVhostErr)
		}
	} else {
		t.Fatal("Expected error")
	}
}

func TestValidateVhostMissingProtoNonProd_EmptyAddr(t *testing.T) {
	prod.SetProd(false)
	address := ""
	coreopts.NewOptionsBuilder(coreoptsloader.LoadOptions())
	validateVhostErr := ValidateVhost(address, "https://")
	if validateVhostErr != nil {
		if !strings.HasPrefix(validateVhostErr.Error(), "missing required protocol") {
			t.Fatalf("Expected missing required protocol error, got %v", validateVhostErr)
		}
	} else {
		t.Fatal("Expected error")
	}
}

func TestValidateVhostMissingProtoNonProd_EmptyAddrDrone(t *testing.T) {
	prod.SetProd(false)
	address := ""
	coreopts.NewOptionsBuilder(coreoptsloader.LoadOptions())
	drone := true
	validateVhostErr := ValidateVhost(address, "https://", &drone)
	if validateVhostErr != nil {
		if !strings.HasPrefix(validateVhostErr.Error(), "missing required protocol") {
			t.Fatalf("Expected missing required protocol error, got %v", validateVhostErr)
		}
	} else {
		t.Fatal("Expected error")
	}
}

func TestValidateVhostMissingProtoNonProd(t *testing.T) {
	prod.SetProd(false)
	address := "http://tierceron.test"
	coreopts.NewOptionsBuilder(coreoptsloader.LoadOptions())
	validateVhostErr := ValidateVhost(address, "https://")
	if validateVhostErr != nil {
		if !strings.HasPrefix(validateVhostErr.Error(), "missing required protocol") {
			t.Fatalf("Expected missing required protocol error, got %v", validateVhostErr)
		}
	} else {
		t.Fatal("Expected error")
	}
}

func TestValidateVhostMissingProtoNonProdDrone(t *testing.T) {
	prod.SetProd(false)
	address := "http://tierceron.test"
	coreopts.NewOptionsBuilder(coreoptsloader.LoadOptions())
	drone := true
	validateVhostErr := ValidateVhost(address, "https://", &drone)
	if validateVhostErr != nil {
		if !strings.HasPrefix(validateVhostErr.Error(), "missing required protocol") {
			t.Fatalf("Expected missing required protocol error, got %v", validateVhostErr)
		}
	} else {
		t.Fatal("Expected error")
	}
}

func TestValidateVhostProdBadHost(t *testing.T) {
	prod.SetProd(true)
	coreopts.NewOptionsBuilder(coreoptsloader.LoadOptions())
	address := "https://tierceron.test"
	validateVhostErr := ValidateVhost(address, "https://")
	if validateVhostErr != nil {
		if !strings.HasPrefix(validateVhostErr.Error(), "Bad host:") {
			t.Fatalf("Expected nil, got %v", validateVhostErr)
		}
	} else {
		t.Fatal("Expected error")
	}
}

func TestValidateVhostProd(t *testing.T) {
	prod.SetProd(true)
	coreopts.NewOptionsBuilder(coreoptsloader.LoadOptions())
	address := "https://prodtierceron.test"
	validateVhostErr := ValidateVhost(address, "https://")
	if validateVhostErr != nil {
		t.Fatal("Expected no error")
	}
}

func TestValidateVhostNonProd(t *testing.T) {
	prod.SetProd(false)
	coreopts.NewOptionsBuilder(coreoptsloader.LoadOptions())
	address := "https://tierceron.test"
	validateVhostErr := ValidateVhost(address, "https://")
	if validateVhostErr != nil {
		t.Fatal("Expected no error")
	}
}

func TestValidateVhostNonProdDrone(t *testing.T) {
	prod.SetProd(false)
	coreopts.NewOptionsBuilder(coreoptsloader.LoadOptions())
	address := "https://tierceron.test"
	drone := true
	validateVhostErr := ValidateVhost(address, "https://", &drone)
	if validateVhostErr != nil {
		t.Fatal("Expected no error")
	}
}
