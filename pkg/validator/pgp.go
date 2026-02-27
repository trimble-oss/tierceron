package validator

import (
	"bytes"
	"fmt"
	"os"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
)

func ValidateASCKeyFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	// 1) Check ASCII armor envelope
	block, err := armor.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("not valid ASCII-armored OpenPGP data: %w", err)
	}

	// Accept public or private key blocks (tighten if you only want public keys)
	switch block.Type {
	case "PGP PUBLIC KEY BLOCK", "PGP PRIVATE KEY BLOCK":
	default:
		return fmt.Errorf("unexpected armor block type: %s", block.Type)
	}

	// 2) Parse key material
	entities, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("invalid OpenPGP key data: %w", err)
	}

	// 3) Minimal structural checks
	for i, e := range entities {
		if e == nil || e.PrimaryKey == nil {
			return fmt.Errorf("entity %d missing primary key", i)
		}
	}

	return nil
}
