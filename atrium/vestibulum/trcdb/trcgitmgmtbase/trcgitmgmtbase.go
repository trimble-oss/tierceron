// Package trcgitmgmtbase provides functionality for Git operations,
// including repository cloning using the GitHub API.
package trcgitmgmtbase

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-github/v61/github"
	"github.com/microsoft/azure-devops-go-api/azuredevops"
	"github.com/microsoft/azure-devops-go-api/azuredevops/git"
	"github.com/trimble-oss/tierceron-core/v2/buildopts/memonly"
	"github.com/trimble-oss/tierceron-core/v2/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/buildopts/kernelopts"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
	"github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
	"golang.org/x/oauth2"
)

// CloneRepository clones a Git repository using the GitHub API.
// It accepts a repository URL and creates a local copy with the complete
// Git directory structure.
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

	// If no target directory is specified, use "../<repositoryname>"
	if targetDir == "" {
		// Parse the URL to extract repository name
		repoName := ""

		// Try to parse the URL properly
		u, err := url.Parse(repoURL)
		if err == nil {
			// Extract the last part of the path as the repository name
			path := strings.TrimPrefix(u.Path, "/")
			parts := strings.Split(path, "/")
			if len(parts) > 0 {
				repoName = parts[len(parts)-1]
				// Remove .git suffix if present
				repoName = strings.TrimSuffix(repoName, ".git")
			}
		}

		// If we couldn't get a repo name from URL parsing, try simpler approach
		if repoName == "" {
			parts := strings.Split(repoURL, "/")
			if len(parts) > 0 {
				repoName = parts[len(parts)-1]
				// Remove .git suffix if present
				repoName = strings.TrimSuffix(repoName, ".git")
			} else {
				// Fallback if we can't extract a name
				repoName = "repo-clone"
			}
		}

		// Set target directory to "../<repositoryname>"
		targetDir = filepath.Join("..", repoName)
	}

	// Create the entire directory path in one go
	// This ensures all directories in the path are created
	// But doesn't fail if directories already exist
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory structure: %w", err)
	}

	// Check if the target directory already exists, but don't fail - just log it
	if _, err := os.Stat(targetDir); !os.IsNotExist(err) {
		driverConfig.CoreConfig.Log.Printf("Note: Target directory already exists, continuing: %s", targetDir)
	}

	// Parse repository URL to extract repo info
	repoInfo, err := parseRepoURL(repoURL)
	if err != nil {
		return err
	}

	driverConfig.CoreConfig.Log.Printf("Cloning repository %s into %s", repoURL, targetDir)
	tempMap, tokenReadAuthErr := mod.ReadData("super-secrets/Restricted/PluginTool/config")
	authToken := ""
	if tokenReadAuthErr == nil && len(tempMap) > 0 {
		tokenKey := "githubToken"
		if repoInfo.IsAzureDevOps {
			tokenKey = "azureToken"
		}
		// If credentials exist, try to use them
		if token, ok := tempMap[tokenKey]; ok {
			authToken = fmt.Sprintf("%v", token)
		} else {
			return fmt.Errorf("missing required %s auth token", tokenKey)
		}
	}

	// Different handling based on the repository type
	if repoInfo.IsGitHub {
		// Clone GitHub repository using the GitHub API
		err = cloneGitHubRepo(authToken, repoInfo.Owner, repoInfo.RepoName, targetDir, driverConfig)
		if err != nil {
			return fmt.Errorf("GitHub API clone failed: %w", err)
		}
	} else if repoInfo.IsAzureDevOps {
		// Clone Azure DevOps repository using the Azure DevOps API
		err = cloneAzureDevOpsRepo(authToken, repoInfo.Owner, repoInfo.Project, repoInfo.RepoName, targetDir, driverConfig)
		if err != nil {
			return fmt.Errorf("azure DevOps API clone failed: %w", err)
		}
	} else {
		return fmt.Errorf("unsupported repository type: %s", repoURL)
	}

	driverConfig.CoreConfig.Log.Printf("Repository %s cloned successfully to %s", repoURL, targetDir)
	return nil
}

// cloneAzureDevOpsRepo clones an Azure DevOps repository using the Azure DevOps API
func cloneAzureDevOpsRepo(authToken, organization, project, repo, targetDir string, driverConfig *config.DriverConfig) error {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Set up Azure DevOps client with authentication
	connection := azuredevops.NewPatConnection(fmt.Sprintf("https://dev.azure.com/%s", organization), authToken)

	// Create a Git client
	gitClient, err := git.NewClient(ctx, connection)
	if err != nil {
		return fmt.Errorf("failed to create Azure DevOps git client: %w", err)
	}

	// No need to get repository information since we're just downloading files
	// Target directory already created in main function

	driverConfig.CoreConfig.Log.Printf("Downloading files from %s/%s/%s using Azure DevOps API...", organization, project, repo)

	// Get the repository items (files and folders)
	itemsArgs := git.GetItemsArgs{
		RepositoryId:   &repo,
		Project:        &project,
		RecursionLevel: &git.VersionControlRecursionTypeValues.Full,
	}

	items, err := gitClient.GetItems(ctx, itemsArgs)
	if err != nil {
		return fmt.Errorf("failed to get repository items: %w", err)
	}

	// Process all items (files and directories)
	for _, item := range *items {
		// Skip folders and ensure IsFolder is not nil
		if item.IsFolder != nil && *item.IsFolder {
			continue
		}

		// Get the content path, ensuring Path is not nil
		if item.Path == nil {
			continue // Skip items without a path
		}
		relativePath := *item.Path
		relativePath = strings.TrimPrefix(relativePath, "/")

		// Create the file path
		filePath := filepath.Join(targetDir, relativePath)

		// Ensure the directory exists
		if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
			return fmt.Errorf("failed to create directory for file '%s': %w", filePath, err)
		}

		// Get the file content
		contentArgs := git.GetItemContentArgs{
			RepositoryId: &repo,
			Project:      &project,
			Path:         &relativePath,
		}

		content, err := gitClient.GetItemContent(ctx, contentArgs)
		if err != nil {
			return fmt.Errorf("failed to get content for file '%s': %w", relativePath, err)
		}

		// Write the file
		file, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("failed to create file '%s': %w", filePath, err)
		}

		_, err = io.Copy(file, content)
		file.Close()

		if err != nil {
			return fmt.Errorf("failed to write file '%s': %w", filePath, err)
		}

		driverConfig.CoreConfig.Log.Printf("Downloaded file: %s", relativePath)
	}

	driverConfig.CoreConfig.Log.Printf("Downloaded all files from %s/%s/%s", organization, project, repo)

	return nil
}

// RepoInfo holds information about a parsed repository URL
type RepoInfo struct {
	Owner         string // GitHub owner or Azure DevOps organization
	RepoName      string // Repository name
	Project       string // Azure DevOps project name (empty for GitHub)
	IsGitHub      bool   // Whether this is a GitHub repository
	IsAzureDevOps bool   // Whether this is an Azure DevOps repository
}

// parseRepoURL extracts repository information from a Git repository URL
func parseRepoURL(repoURL string) (RepoInfo, error) {
	info := RepoInfo{}

	if !strings.HasPrefix(repoURL, "https://") {
		repoURL = "https://" + repoURL
	}

	// Parse the URL
	u, err := url.Parse(repoURL)
	if err != nil {
		return info, fmt.Errorf("failed to parse URL: %w", err)
	}

	// Check if it's a GitHub URL
	if strings.Contains(u.Host, "github.com") {
		// Parse GitHub URL format: github.com/owner/repo[.git]
		path := strings.TrimPrefix(u.Path, "/")
		parts := strings.Split(path, "/")

		if len(parts) < 2 {
			return info, fmt.Errorf("invalid GitHub URL format: %s", repoURL)
		}

		info.Owner = parts[0]
		info.RepoName = parts[1]

		// Remove .git suffix if present
		info.RepoName = strings.TrimSuffix(info.RepoName, ".git")
		info.IsGitHub = true

		return info, nil
	}

	// Check if it's an Azure DevOps URL
	if strings.Contains(u.Host, "dev.azure.com") {
		// Parse Azure DevOps URL format: dev.azure.com/organization/project/_git/repository
		path := strings.TrimPrefix(u.Path, "/")
		parts := strings.Split(path, "/")

		// Need at least organization/project/_git/repository
		if len(parts) < 4 || parts[2] != "_git" {
			return info, fmt.Errorf("invalid Azure DevOps URL format: %s", repoURL)
		}

		info.Owner = parts[0]    // Organization
		info.Project = parts[1]  // Project
		info.RepoName = parts[3] // Repository name
		info.IsAzureDevOps = true

		return info, nil
	}

	// Not a supported URL
	return info, fmt.Errorf("unsupported repository URL: %s", repoURL)
}

// cloneGitHubRepo clones a GitHub repository using the GitHub API
func cloneGitHubRepo(authToken, owner, repo, targetDir string, driverConfig *config.DriverConfig) error {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Set up GitHub client with authentication if token is available
	var client *github.Client
	if authToken != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: authToken},
		)
		tc := oauth2.NewClient(ctx, ts)
		client = github.NewClient(tc)
	} else {
		client = github.NewClient(nil)
	}

	// Target directory already created in main function
	driverConfig.CoreConfig.Log.Printf("Downloading files from %s/%s using GitHub API...", owner, repo)

	// Use default branch (main/master) for downloading files
	defaultBranch := "main"

	// Download repository contents
	err := downloadRepositoryContents(ctx, client, owner, repo, "", targetDir, defaultBranch, driverConfig)
	if err != nil {
		return fmt.Errorf("failed to download repository contents: %w", err)
	}

	driverConfig.CoreConfig.Log.Printf("Downloaded all files from %s/%s", owner, repo)

	return nil
}

// downloadRepositoryContents recursively downloads all files in the repository
func downloadRepositoryContents(ctx context.Context, client *github.Client, owner, repo, path, targetDir, ref string, driverConfig *config.DriverConfig) error {
	_, directoryContent, _, err := client.Repositories.GetContents(ctx, owner, repo, path, &github.RepositoryContentGetOptions{
		Ref: ref,
	})
	if err != nil {
		return fmt.Errorf("failed to fetch directory contents for path '%s': %w", path, err)
	}

	for _, content := range directoryContent {
		// Skip if content or Type is nil
		if content == nil || content.Type == nil {
			continue
		}
		switch *content.Type {
		case "file":
			// Download file
			fileContent, _, _, err := client.Repositories.GetContents(ctx, owner, repo, *content.Path, &github.RepositoryContentGetOptions{
				Ref: ref,
			})
			if err != nil {
				return fmt.Errorf("failed to fetch file '%s': %w", *content.Path, err)
			}

			// Skip if Path is nil
			if content.Path == nil {
				continue
			}

			// Create the file path
			filePath := filepath.Join(targetDir, *content.Path)

			// Ensure the directory exists
			if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
				return fmt.Errorf("failed to create directory for file '%s': %w", filePath, err)
			}

			// Decode and write the file content
			decodedContent, err := fileContent.GetContent()
			if err != nil {
				return fmt.Errorf("failed to decode content for file '%s': %w", *content.Path, err)
			}

			if err := os.WriteFile(filePath, []byte(decodedContent), 0o644); err != nil {
				return fmt.Errorf("failed to write file '%s': %w", filePath, err)
			}

			driverConfig.CoreConfig.Log.Printf("Downloaded file: %s", *content.Path)

		case "dir":
			// Create the directory
			dirPath := filepath.Join(targetDir, *content.Path)
			if err := os.MkdirAll(dirPath, 0o755); err != nil {
				return fmt.Errorf("failed to create directory '%s': %w", dirPath, err)
			}

			// Recursively download the directory contents
			if err := downloadRepositoryContents(ctx, client, owner, repo, *content.Path, targetDir, ref, driverConfig); err != nil {
				return err
			}
		}
	}

	return nil
}
