// Package trcgitmgmtbase provides functionality for Git operations,
// including repository cloning using the Git command line.
package trcgitmgmtbase

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/trimble-oss/tierceron-core/v2/buildopts/memonly"
	"github.com/trimble-oss/tierceron-core/v2/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/buildopts/kernelopts"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
	"github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)

// CloneRepository is a simple function that uses the git command line to clone
// any repository URL. It accepts the direct repository URL rather than trying to
// parse or construct URLs for different repository types.
func CloneRepository(
	repoURL string,
	targetDir string,
	envPtr *string,
	tokenNamePtr *string,
	driverConfig *config.DriverConfig,
	mod *kv.Modifier,
) error {
	// If we're running inside the hive, disable this function
	if kernelopts.BuildOptions.IsKernel() {
		return errors.New("git cloning is not available when running within the hive")
	}

	// Initialize memory protection if needed
	if memonly.IsMemonly() {
		memprotectopts.MemProtectInit(nil)
	}

	// Get authentication token from the vault's Restricted section if available
	authToken := ""
	tempMap, readErr := mod.ReadData("super-secrets/Restricted/GitHubConfig/config")
	if readErr == nil && len(tempMap) > 0 {
		// If credentials exist, try to use them
		if token, ok := tempMap["github_token"]; ok {
			authToken = fmt.Sprintf("%v", token)
		}
	}

	// If no target directory is specified, extract a default one from the repo URL
	if targetDir == "" {
		// Parse the repository URL to get a sensible default directory name
		parts := strings.Split(repoURL, "/")
		if len(parts) > 0 {
			// Get the last part of the URL (usually the repo name)
			repoName := parts[len(parts)-1]
			// Remove .git suffix if present
			repoName = strings.TrimSuffix(repoName, ".git")
			targetDir = filepath.Join(".", repoName)
		} else {
			// Fallback to a generic name if we can't parse the URL
			targetDir = filepath.Join(".", "repo-clone")
		}
	}

	// Check if the target directory already exists
	if _, err := os.Stat(targetDir); !os.IsNotExist(err) {
		return fmt.Errorf("target directory already exists: %s", targetDir)
	}

	// Create the parent directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(targetDir), 0o755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Prepare the git command
	args := []string{"clone", "--depth", "1"}

	// If we have a token and the URL contains github.com or dev.azure.com,
	// inject the token into the URL for authentication
	cloneURL := repoURL
	if authToken != "" {
		if strings.Contains(repoURL, "github.com") {
			// Format: https://TOKEN@github.com/owner/repo.git
			cloneURL = strings.Replace(repoURL, "https://", fmt.Sprintf("https://%s@", authToken), 1)
		} else if strings.Contains(repoURL, "dev.azure.com") {
			// Format: https://TOKEN@dev.azure.com/org/project/_git/repo
			cloneURL = strings.Replace(repoURL, "https://", fmt.Sprintf("https://%s@", authToken), 1)
		}
	}

	// Add the clone URL and target directory
	args = append(args, cloneURL, targetDir)

	driverConfig.CoreConfig.Log.Printf("Cloning repository %s into %s", repoURL, targetDir)

	// Execute the git clone command
	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()

	// Sanitize the output to remove any tokens that might be in the output
	sanitizedOutput := string(output)
	if authToken != "" {
		sanitizedOutput = strings.ReplaceAll(sanitizedOutput, authToken, "***")
	}

	if err != nil {
		return fmt.Errorf("git clone failed: %s: %w", sanitizedOutput, err)
	}

	driverConfig.CoreConfig.Log.Printf("Repository %s cloned successfully to %s", repoURL, targetDir)
	return nil
}
