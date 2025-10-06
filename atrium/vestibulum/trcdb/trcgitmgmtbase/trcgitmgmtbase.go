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

	// Create the target directory
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Initialize .git directory structure
	gitDir := filepath.Join(targetDir, ".git")
	if err := os.MkdirAll(filepath.Join(gitDir, "objects", "pack"), 0o755); err != nil {
		return fmt.Errorf("failed to create .git directory structure: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(gitDir, "refs", "heads"), 0o755); err != nil {
		return fmt.Errorf("failed to create .git directory structure: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(gitDir, "refs", "tags"), 0o755); err != nil {
		return fmt.Errorf("failed to create .git directory structure: %w", err)
	}

	// Parse repository URL to extract repo info
	repoInfo, err := parseRepoURL(repoURL)
	if err != nil {
		return err
	}

	driverConfig.CoreConfig.Log.Printf("Cloning repository %s into %s", repoURL, targetDir)

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
			return fmt.Errorf("Azure DevOps API clone failed: %w", err)
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

	// Get repository information to find the default branch
	repoArgs := git.GetRepositoryArgs{
		RepositoryId: &repo,
		Project:      &project,
	}

	repository, err := gitClient.GetRepository(ctx, repoArgs)
	if err != nil {
		return fmt.Errorf("failed to get Azure DevOps repository info: %w", err)
	}

	defaultBranch := ""
	if repository.DefaultBranch != nil {
		// Convert from refs/heads/main to just main
		defaultBranch = strings.TrimPrefix(*repository.DefaultBranch, "refs/heads/")
	} else {
		// Fallback to main if default branch is not set
		defaultBranch = "main"
	}

	// Create a basic git config file
	configFile := filepath.Join(targetDir, ".git", "config")
	repoURL := fmt.Sprintf("https://dev.azure.com/%s/%s/_git/%s", organization, project, repo)
	configContent := fmt.Sprintf(`[core]
	repositoryformatversion = 0
	filemode = true
	bare = false
	logallrefupdates = true
[remote "origin"]
	url = %s
	fetch = +refs/heads/*:refs/remotes/origin/*
[branch "%s"]
	remote = origin
	merge = refs/heads/%s
`, repoURL, defaultBranch, defaultBranch)

	if err := os.WriteFile(configFile, []byte(configContent), 0o644); err != nil {
		return fmt.Errorf("failed to create git config file: %w", err)
	}

	// Create HEAD file pointing to the default branch
	headFile := filepath.Join(targetDir, ".git", "HEAD")
	headContent := fmt.Sprintf("ref: refs/heads/%s\n", defaultBranch)
	if err := os.WriteFile(headFile, []byte(headContent), 0o644); err != nil {
		return fmt.Errorf("failed to create HEAD file: %w", err)
	}

	top := 1
	// Get the latest commit for the default branch
	commitArgs := git.GetCommitsArgs{
		RepositoryId: &repo,
		Project:      &project,
		SearchCriteria: &git.GitQueryCommitsCriteria{
			ItemVersion: &git.GitVersionDescriptor{
				Version:     &defaultBranch,
				VersionType: &git.GitVersionTypeValues.Branch,
			},
			Top: &top,
		},
	}

	commits, err := gitClient.GetCommits(ctx, commitArgs)
	if err != nil {
		return fmt.Errorf("failed to get commits: %w", err)
	}

	// Write the SHA to the refs file if we found any commits
	var latestCommitSHA string
	if len(*commits) > 0 {
		latestCommitSHA = *((*commits)[0].CommitId)
		refsFile := filepath.Join(targetDir, ".git", "refs", "heads", defaultBranch)
		if err := os.WriteFile(refsFile, []byte(latestCommitSHA), 0o644); err != nil {
			return fmt.Errorf("failed to create refs file: %w", err)
		}
	} else {
		return fmt.Errorf("no commits found in repository")
	}

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
		// Skip folders, we'll create them when processing files
		if *item.IsFolder {
			continue
		}

		// Get the content path
		relativePath := *item.Path
		if strings.HasPrefix(relativePath, "/") {
			relativePath = relativePath[1:]
		}

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

	// Get the repository details to obtain the default branch
	repository, _, err := client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return fmt.Errorf("failed to get repository info: %w", err)
	}

	defaultBranch := repository.GetDefaultBranch()

	// Create a basic git config file
	configFile := filepath.Join(targetDir, ".git", "config")
	configContent := fmt.Sprintf(`[core]
	repositoryformatversion = 0
	filemode = true
	bare = false
	logallrefupdates = true
[remote "origin"]
	url = %s
	fetch = +refs/heads/*:refs/remotes/origin/*
[branch "%s"]
	remote = origin
	merge = refs/heads/%s
`, repository.GetCloneURL(), defaultBranch, defaultBranch)

	if err := os.WriteFile(configFile, []byte(configContent), 0o644); err != nil {
		return fmt.Errorf("failed to create git config file: %w", err)
	}

	// Create HEAD file pointing to the default branch
	headFile := filepath.Join(targetDir, ".git", "HEAD")
	headContent := fmt.Sprintf("ref: refs/heads/%s\n", defaultBranch)
	if err := os.WriteFile(headFile, []byte(headContent), 0o644); err != nil {
		return fmt.Errorf("failed to create HEAD file: %w", err)
	}

	// Create the refs for the default branch
	refsFile := filepath.Join(targetDir, ".git", "refs", "heads", defaultBranch)
	// We'll add the SHA later once we have it

	// We'll proceed directly to getting the commit SHA and downloading contents
	// No need to get the repository content tree at this point

	// Keep track of the latest commit SHA
	var latestCommitSHA string

	// Get the latest commit SHA
	commits, _, err := client.Repositories.ListCommits(ctx, owner, repo, &github.CommitsListOptions{
		SHA: defaultBranch,
		ListOptions: github.ListOptions{
			PerPage: 1,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to get latest commit: %w", err)
	}

	if len(commits) > 0 {
		latestCommitSHA = *commits[0].SHA
		// Write the SHA to the refs file
		if err := os.WriteFile(refsFile, []byte(latestCommitSHA), 0o644); err != nil {
			return fmt.Errorf("failed to create refs file: %w", err)
		}
	} else {
		return fmt.Errorf("no commits found in repository")
	}

	// Process each file and directory in the repository
	err = downloadRepositoryContents(ctx, client, owner, repo, "", targetDir, defaultBranch, driverConfig)
	if err != nil {
		return fmt.Errorf("failed to download repository contents: %w", err)
	}

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
		switch *content.Type {
		case "file":
			// Download file
			fileContent, _, _, err := client.Repositories.GetContents(ctx, owner, repo, *content.Path, &github.RepositoryContentGetOptions{
				Ref: ref,
			})
			if err != nil {
				return fmt.Errorf("failed to fetch file '%s': %w", *content.Path, err)
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
