package utils

import (
	"fmt"
	"os"

	"github.com/trimble-oss/tierceron/pkg/utils/config"
)

// GetUnrestrictedAccess performs OAuth authentication for unrestricted write access
// This can be called from trcsh at any time to obtain write-access credentials
// Example usage in trcsh: utils.GetUnrestrictedAccess(driverConfig)
func GetUnrestrictedAccess(driverConfig *config.DriverConfig) error {
	fmt.Fprintf(os.Stderr, "\n=== Obtaining Unrestricted Write Access ===\n")
	fmt.Fprintf(os.Stderr, "This will authenticate you for write access to configuration tokens.\n")
	fmt.Fprintf(os.Stderr, "You must be authorized in the trcshunrestricted Vault JWT role.\n\n")
	fmt.Fprintf(os.Stderr, "GetUnrestrictedAccess: received driverConfig=%p\n", driverConfig)

	err := KernelZOAuthForRole(driverConfig, "trcshunrestricted")
	if err != nil {
		return fmt.Errorf("failed to obtain unrestricted access: %w", err)
	}

	fmt.Fprintf(os.Stderr, "\n=== Unrestricted Access Granted ===\n")
	fmt.Fprintf(os.Stderr, "You now have write access to configuration tokens.\n\n")

	return nil
}

// GetReadAccess performs OAuth authentication for read-only access
// This is the default mode for trcsh and is typically called automatically at startup
// Example usage in trcsh: utils.GetReadAccess(driverConfig)
func GetReadAccess(driverConfig *config.DriverConfig) error {
	fmt.Fprintf(os.Stderr, "\n=== Obtaining Read Access ===\n")
	fmt.Fprintf(os.Stderr, "This will authenticate you for read access to configuration tokens.\n\n")

	err := KernelZOAuthForRole(driverConfig, "trcshhivez")
	if err != nil {
		return fmt.Errorf("failed to obtain read access: %w", err)
	}

	fmt.Fprintf(os.Stderr, "\n=== Read Access Granted ===\n")
	fmt.Fprintf(os.Stderr, "You now have read access to configuration tokens.\n\n")

	return nil
}
