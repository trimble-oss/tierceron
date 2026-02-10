package shellcmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/trimble-oss/tierceron-core/v2/trcshfs/trcshio"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcdb/trcplgtoolbase"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcsh/kube/native"
	"github.com/trimble-oss/tierceron/pkg/capauth"
	"github.com/trimble-oss/tierceron/pkg/cli/trcconfigbase"
	"github.com/trimble-oss/tierceron/pkg/cli/trcinitbase"
	"github.com/trimble-oss/tierceron/pkg/cli/trcpubbase"
	"github.com/trimble-oss/tierceron/pkg/cli/trcsubbase"
	"github.com/trimble-oss/tierceron/pkg/cli/trcxbase"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
)

// hasUnrestrictedAccess checks if the user has unrestricted write access
// by verifying if trcshunrestricted role credentials are present in TokenCache
func hasUnrestrictedAccess(driverConfig *config.DriverConfig) bool {
	if driverConfig == nil || driverConfig.CoreConfig == nil || driverConfig.CoreConfig.TokenCache == nil {
		return false
	}

	// Check if trcshunrestricted role has credentials
	roleName := "trcshunrestricted"
	appRoleSecret := driverConfig.CoreConfig.TokenCache.GetRoleStr(&roleName)
	if appRoleSecret == nil || len(*appRoleSecret) < 2 {
		return false
	}

	// Verify credentials are non-empty (valid UUID format is 36 chars)
	roleID := (*appRoleSecret)[0]
	secretID := (*appRoleSecret)[1]
	return len(roleID) == 36 && len(secretID) == 36
}

// ExecuteShellCommand executes a shell command based on the command type string from ChatMsg.Response
// Returns the MemoryFileSystem where command output is written
func ExecuteShellCommand(cmdType string, args []string, driverConfig *config.DriverConfig) trcshio.MemoryFileSystem {
	if driverConfig == nil {
		return nil
	}

	if driverConfig.CoreConfig != nil && driverConfig.CoreConfig.Log != nil {
		driverConfig.CoreConfig.Log.Printf("ExecuteShellCommand: cmdType=%s, args=%v, IsShellCommand before=%v\n", cmdType, args, driverConfig.IsShellCommand)
	}

	// Clear io/STDIO before each command to avoid accumulating output
	if driverConfig.MemFs != nil {
		if _, statErr := driverConfig.MemFs.Stat("io"); statErr == nil {
			// io directory exists, truncate the STDIO file
			if stdioFile, err := driverConfig.MemFs.OpenFile("io/STDIO", os.O_RDWR|os.O_TRUNC|os.O_CREATE, 0o644); err == nil {
				stdioFile.Close()
			}
		}
	}

	// Mark that this command is running from trcshcmd
	driverConfig.IsShellCommand = true

	var err error

	// Pull common values from DriverConfig like trcsh.go does
	envDefaultPtr := driverConfig.CoreConfig.EnvBasis
	tokenName := "config_token_" + driverConfig.CoreConfig.EnvBasis
	envCtx := driverConfig.CoreConfig.EnvBasis // Use same as envDefaultPtr
	region := ""
	if len(driverConfig.CoreConfig.Regions) > 0 {
		region = driverConfig.CoreConfig.Regions[0]
	}

	// Prepend command name to args as argLines[0]
	argLines := append([]string{cmdType}, args...)

	switch cmdType {
	case CmdTrcConfig:
		err = trcconfigbase.CommonMain(&envDefaultPtr, &envCtx, &tokenName, &region, nil, argLines, driverConfig)

	case CmdTrcPub:
		// Require elevated access for trcpub (write operations)
		if !hasUnrestrictedAccess(driverConfig) {
			err = errors.New("AUTHORIZATION ERROR: 'tpub' command requires elevated access. Run 'su' to obtain unrestricted credentials.")
			break
		}
		pubTokenName := fmt.Sprintf("vault_pub_token_%s", driverConfig.CoreConfig.EnvBasis)
		pubEnv := driverConfig.CoreConfig.Env
		trcpubbase.CommonMain(&pubEnv, &envCtx, &pubTokenName, nil, argLines, driverConfig)

	case CmdTrcSub:
		originalEndDir := driverConfig.EndDir
		err = trcsubbase.CommonMain(&envDefaultPtr, &envCtx, &tokenName, nil, argLines, driverConfig)
		driverConfig.EndDir = originalEndDir

	case CmdTrcX:
		// Require elevated access for trcx (write operations)
		if !hasUnrestrictedAccess(driverConfig) {
			err = errors.New("AUTHORIZATION ERROR: 'tx' command requires elevated access. Run 'su' to obtain unrestricted credentials.")
			break
		}
		trcxbase.CommonMain(nil, nil, &envDefaultPtr, nil, &envCtx, nil, nil, argLines, driverConfig)

	case CmdTrcInit:
		// Require elevated access for trcinit (write operations)
		if !hasUnrestrictedAccess(driverConfig) {
			err = errors.New("AUTHORIZATION ERROR: 'tinit' command requires elevated access. Run 'su' to obtain unrestricted credentials.")
			break
		}
		pubTokenName := fmt.Sprintf("vault_pub_token_%s", driverConfig.CoreConfig.EnvBasis)
		pubEnv := driverConfig.CoreConfig.Env
		uploadCert := driverConfig.CoreConfig.WantCerts
		trcinitbase.CommonMain(&pubEnv, &envCtx, &pubTokenName, &uploadCert, nil, args, driverConfig)

	case CmdTrcPlgtool:
		env := driverConfig.CoreConfig.Env
		plgTokenName := "config_token_pluginany"
		// Create TrcshDriverConfig wrapper
		trcshDriverConfig := &capauth.TrcshDriverConfig{
			DriverConfig: driverConfig,
		}
		err = trcplgtoolbase.CommonMain(&env, &envCtx, &plgTokenName, &region, nil, args, trcshDriverConfig)

	case CmdKubectl:
		// Initialize kubectl configuration
		trcKubeConfig, kubeErr := native.InitTrcKubeConfig(nil, driverConfig.CoreConfig)
		if kubeErr != nil {
			err = kubeErr
		} else {
			// Execute kubectl command
			err = native.KubeCtl(trcKubeConfig, driverConfig)
		}

	case CmdTrcBoot:
		// Simply return the memFs from driverConfig without executing any commands
		// This is used for initializing plugins that need access to the shared memFs
		if driverConfig.MemFs != nil {
			return driverConfig.MemFs
		}
		return nil

	case CmdRm:
		err = ExecuteRm(args, driverConfig)

	case CmdCp:
		err = ExecuteCp(args, driverConfig)

	case CmdMv:
		err = ExecuteMv(args, driverConfig)

	case CmdCat:
		err = ExecuteCat(args, driverConfig)

	case CmdSu:
		// Perform OAuth authentication for unrestricted write access
		err = ExecuteSu(driverConfig)

	default:
		// Unknown command type
		return nil
	}

	if err != nil {
		// Error occurred, but MemFs may still have partial output
		if driverConfig.CoreConfig != nil && driverConfig.CoreConfig.Log != nil {
			driverConfig.CoreConfig.Log.Printf("ExecuteShellCommand: command execution error: %v\n", err)
		}

		// Write error message to io/STDIO so shell can display it
		// This is especially important for authorization errors
		if driverConfig.MemFs != nil {
			errMsg := fmt.Sprintf("%v\n", err)
			outputData := []byte(errMsg)

			// Check if io directory exists
			if _, statErr := driverConfig.MemFs.Stat("io"); statErr == nil {
				// Directory exists, open file for append
				if stdioFile, writeErr := driverConfig.MemFs.OpenFile("io/STDIO", os.O_RDWR|os.O_APPEND|os.O_CREATE, 0o644); writeErr == nil {
					stdioFile.Write(outputData)
					stdioFile.Close()
				} else {
					driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &outputData, "io/STDIO")
				}
			} else {
				// Directory doesn't exist, use WriteToMemFile to create it
				driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &outputData, "io/STDIO")
			}
		}
	}

	// Return the MemFs where command wrote its output
	if driverConfig.CoreConfig != nil && driverConfig.CoreConfig.Log != nil {
		driverConfig.CoreConfig.Log.Printf("ExecuteShellCommand: returning MemFs (nil=%v)\n", driverConfig.MemFs == nil)
	}
	return driverConfig.MemFs
}

// ExecuteRm removes files or directories from memfs
// Supports -r flag for recursive directory deletion
func ExecuteRm(args []string, driverConfig *config.DriverConfig) error {
	if driverConfig == nil || driverConfig.MemFs == nil {
		return errors.New("driver config or memfs is nil")
	}

	if len(args) == 0 {
		errMsg := "rm: missing operand"
		outputBytes := []byte(errMsg)
		driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &outputBytes, "io/STDIO")
		return errors.New(errMsg)
	}

	recursive := false
	var paths []string

	// Parse arguments
	for _, arg := range args {
		if arg == "-r" || arg == "-R" || arg == "--recursive" {
			recursive = true
		} else if !strings.HasPrefix(arg, "-") {
			paths = append(paths, arg)
		}
	}

	if len(paths) == 0 {
		errMsg := "rm: missing file operand"
		outputBytes := []byte(errMsg)
		driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &outputBytes, "io/STDIO")
		return errors.New(errMsg)
	}

	var output strings.Builder
	var hasError bool

	// Process each path
	for _, path := range paths {
		if err := removePath(driverConfig, path, recursive); err != nil {
			if driverConfig.CoreConfig != nil && driverConfig.CoreConfig.Log != nil {
				driverConfig.CoreConfig.Log.Printf("rm: %v\n", err)
			}
			output.WriteString(fmt.Sprintf("rm: %v\n", err))
			hasError = true
		}
	}

	// Write output to io/STDIO
	var outputData []byte
	if output.Len() > 0 {
		outputData = []byte(output.String())
	} else {
		outputData = []byte("Files removed successfully\n")
	}

	// Check if io directory exists
	if _, statErr := driverConfig.MemFs.Stat("io"); statErr == nil {
		// Directory exists, open file for append
		if stdioFile, err := driverConfig.MemFs.OpenFile("io/STDIO", os.O_RDWR|os.O_APPEND|os.O_CREATE, 0o644); err == nil {
			stdioFile.Write(outputData)
			stdioFile.Close()
		} else {
			driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &outputData, "io/STDIO")
		}
	} else {
		// Directory doesn't exist, use WriteToMemFile to create it
		driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &outputData, "io/STDIO")
	}

	if hasError {
		return errors.New("rm encountered errors")
	}
	return nil
}

// removePath removes a single file or directory
func removePath(driverConfig *config.DriverConfig, path string, recursive bool) error {
	// Clean the path
	if strings.HasPrefix(path, "./") {
		path = strings.TrimPrefix(path, "./")
	}

	// Check if path exists
	info, err := driverConfig.MemFs.Stat(path)
	if err != nil {
		return fmt.Errorf("cannot remove '%s': %v", path, err)
	}

	// If it's a directory, handle recursively if flag is set
	if info.IsDir() {
		if !recursive {
			return fmt.Errorf("cannot remove '%s': Is a directory (use -r to remove directories)", path)
		}

		// Remove directory recursively using Walk
		var pathsToRemove []string
		err := driverConfig.MemFs.Walk(path, func(walkPath string, isDir bool) error {
			pathsToRemove = append(pathsToRemove, walkPath)
			return nil
		})
		if err != nil {
			return fmt.Errorf("cannot remove '%s': %v", path, err)
		}

		// Remove in reverse order (files first, then directories)
		for i := len(pathsToRemove) - 1; i >= 0; i-- {
			if err := driverConfig.MemFs.Remove(pathsToRemove[i]); err != nil {
				if driverConfig.CoreConfig != nil && driverConfig.CoreConfig.Log != nil {
					driverConfig.CoreConfig.Log.Printf("warning: could not remove '%s': %v\n", pathsToRemove[i], err)
				}
			}
		}

		// Finally remove the directory itself
		return driverConfig.MemFs.Remove(path)
	}

	// It's a file, remove it directly
	return driverConfig.MemFs.Remove(path)
}

// ExecuteCp copies files or directories from source to destination
// Supports -r flag for recursive directory copying
func ExecuteCp(args []string, driverConfig *config.DriverConfig) error {
	if driverConfig == nil || driverConfig.MemFs == nil {
		errMsg := "driver config or memfs is nil"
		outputBytes := []byte(errMsg)
		driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &outputBytes, "io/STDIO")
		return errors.New(errMsg)
	}

	if len(args) < 2 {
		errMsg := "cp: missing file operand"
		outputBytes := []byte(errMsg)
		driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &outputBytes, "io/STDIO")
		return errors.New(errMsg)
	}

	recursive := false
	var paths []string

	// Parse arguments
	for _, arg := range args {
		if arg == "-r" || arg == "-R" || arg == "--recursive" {
			recursive = true
		} else if !strings.HasPrefix(arg, "-") {
			paths = append(paths, arg)
		}
	}

	if len(paths) < 2 {
		errMsg := "cp: missing destination file operand"
		outputData := []byte(errMsg)
		// Check if io directory exists
		if _, statErr := driverConfig.MemFs.Stat("io"); statErr == nil {
			// Directory exists, open file for append
			if stdioFile, err := driverConfig.MemFs.OpenFile("io/STDIO", os.O_RDWR|os.O_APPEND|os.O_CREATE, 0o644); err == nil {
				stdioFile.Write(outputData)
				stdioFile.Close()
			} else {
				driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &outputData, "io/STDIO")
			}
		} else {
			// Directory doesn't exist, use WriteToMemFile to create it
			driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &outputData, "io/STDIO")
		}
		return errors.New(errMsg)
	}

	source := paths[0]
	dest := paths[1]

	// Clean paths
	if strings.HasPrefix(source, "./") {
		source = strings.TrimPrefix(source, "./")
	}
	if strings.HasPrefix(dest, "./") {
		dest = strings.TrimPrefix(dest, "./")
	}

	// Check if source exists
	sourceInfo, err := driverConfig.MemFs.Stat(source)
	if err != nil {
		errMsg := fmt.Sprintf("cannot stat '%s': %v", source, err)
		outputData := []byte(errMsg)
		// Check if io directory exists
		if _, statErr := driverConfig.MemFs.Stat("io"); statErr == nil {
			// Directory exists, open file for append
			if stdioFile, err := driverConfig.MemFs.OpenFile("io/STDIO", os.O_RDWR|os.O_APPEND|os.O_CREATE, 0o644); err == nil {
				stdioFile.Write(outputData)
				stdioFile.Close()
			} else {
				driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &outputData, "io/STDIO")
			}
		} else {
			// Directory doesn't exist, use WriteToMemFile to create it
			driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &outputData, "io/STDIO")
		}
		return errors.New(errMsg)
	}

	// If source is a directory, require -r flag
	if sourceInfo.IsDir() {
		if !recursive {
			errMsg := fmt.Sprintf("cp: -r not specified; omitting directory '%s'", source)
			outputData := []byte(errMsg)
			// Check if io directory exists
			if _, statErr := driverConfig.MemFs.Stat("io"); statErr == nil {
				// Directory exists, open file for append
				if stdioFile, err := driverConfig.MemFs.OpenFile("io/STDIO", os.O_RDWR|os.O_APPEND|os.O_CREATE, 0o644); err == nil {
					stdioFile.Write(outputData)
					stdioFile.Close()
				} else {
					driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &outputData, "io/STDIO")
				}
			} else {
				// Directory doesn't exist, use WriteToMemFile to create it
				driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &outputData, "io/STDIO")
			}
			return errors.New(errMsg)
		}
		err = copyDirectory(driverConfig, source, dest)
	} else {
		// Copy single file
		err = copyFile(driverConfig, source, dest)
	}

	if err != nil {
		outputData := []byte(err.Error())
		// Check if io directory exists
		if _, statErr := driverConfig.MemFs.Stat("io"); statErr == nil {
			// Directory exists, open file for append
			if stdioFile, err := driverConfig.MemFs.OpenFile("io/STDIO", os.O_RDWR|os.O_APPEND|os.O_CREATE, 0o644); err == nil {
				stdioFile.Write(outputData)
				stdioFile.Close()
			} else {
				driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &outputData, "io/STDIO")
			}
		} else {
			// Directory doesn't exist, use WriteToMemFile to create it
			driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &outputData, "io/STDIO")
		}
		return err
	}

	// Success message
	outputData := []byte("Files copied successfully\n")
	// Check if io directory exists
	if _, statErr := driverConfig.MemFs.Stat("io"); statErr == nil {
		// Directory exists, open file for append
		if stdioFile, err := driverConfig.MemFs.OpenFile("io/STDIO", os.O_RDWR|os.O_APPEND|os.O_CREATE, 0o644); err == nil {
			stdioFile.Write(outputData)
			stdioFile.Close()
		} else {
			driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &outputData, "io/STDIO")
		}
	} else {
		// Directory doesn't exist, use WriteToMemFile to create it
		driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &outputData, "io/STDIO")
	}
	return nil
}

// copyFile copies a single file from source to destination
func copyFile(driverConfig *config.DriverConfig, source, dest string) error {
	// Open source file
	srcFile, err := driverConfig.MemFs.Open(source)
	if err != nil {
		return fmt.Errorf("cannot open '%s': %v", source, err)
	}
	defer srcFile.Close()

	// Check if destination is a directory
	if destInfo, err := driverConfig.MemFs.Stat(dest); err == nil {
		if destInfo.IsDir() {
			// Destination is a directory, append source filename
			dest = path.Join(dest, path.Base(source))
		}
	}

	// Create destination file
	destFile, err := driverConfig.MemFs.Create(dest)
	if err != nil {
		return fmt.Errorf("cannot create '%s': %v", dest, err)
	}
	defer destFile.Close()

	// Copy contents
	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return fmt.Errorf("error copying '%s' to '%s': %v", source, dest, err)
	}

	return nil
}

// copyDirectory recursively copies a directory from source to destination
func copyDirectory(driverConfig *config.DriverConfig, source, dest string) error {
	// Create destination directory
	destFile, err := driverConfig.MemFs.Create(dest)
	if err != nil {
		return fmt.Errorf("cannot create directory '%s': %v", dest, err)
	}
	destFile.Close()

	// Walk through source directory
	err = driverConfig.MemFs.Walk(source, func(walkPath string, isDir bool) error {
		if walkPath == source {
			return nil // Skip the source directory itself
		}

		// Calculate relative path and destination path
		relPath := strings.TrimPrefix(walkPath, source)
		relPath = strings.TrimPrefix(relPath, "/")
		destPath := path.Join(dest, relPath)

		if isDir {
			// Create directory
			dirFile, err := driverConfig.MemFs.Create(destPath)
			if err != nil {
				return fmt.Errorf("cannot create directory '%s': %v", destPath, err)
			}
			dirFile.Close()
		} else {
			// Copy file
			if err := copyFile(driverConfig, walkPath, destPath); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("error copying directory '%s': %v", source, err)
	}

	return nil
}

// ExecuteMv moves (renames) files or directories from source to destination
func ExecuteMv(args []string, driverConfig *config.DriverConfig) error {
	if driverConfig == nil || driverConfig.MemFs == nil {
		errMsg := "driver config or memfs is nil"
		outputData := []byte(errMsg)
		// Check if io directory exists
		if _, statErr := driverConfig.MemFs.Stat("io"); statErr == nil {
			// Directory exists, open file for append
			if stdioFile, err := driverConfig.MemFs.OpenFile("io/STDIO", os.O_RDWR|os.O_APPEND|os.O_CREATE, 0o644); err == nil {
				stdioFile.Write(outputData)
				stdioFile.Close()
			} else {
				driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &outputData, "io/STDIO")
			}
		} else {
			// Directory doesn't exist, use WriteToMemFile to create it
			driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &outputData, "io/STDIO")
		}
		return errors.New(errMsg)
	}

	if len(args) < 2 {
		errMsg := "mv: missing file operand"
		outputData := []byte(errMsg)
		// Check if io directory exists
		if _, statErr := driverConfig.MemFs.Stat("io"); statErr == nil {
			// Directory exists, open file for append
			if stdioFile, err := driverConfig.MemFs.OpenFile("io/STDIO", os.O_RDWR|os.O_APPEND|os.O_CREATE, 0o644); err == nil {
				stdioFile.Write(outputData)
				stdioFile.Close()
			} else {
				driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &outputData, "io/STDIO")
			}
		} else {
			// Directory doesn't exist, use WriteToMemFile to create it
			driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &outputData, "io/STDIO")
		}
		return errors.New(errMsg)
	}

	source := args[0]
	dest := args[1]

	// Clean paths
	if strings.HasPrefix(source, "./") {
		source = strings.TrimPrefix(source, "./")
	}
	if strings.HasPrefix(dest, "./") {
		dest = strings.TrimPrefix(dest, "./")
	}

	// Check if source exists
	sourceInfo, err := driverConfig.MemFs.Stat(source)
	if err != nil {
		errMsg := fmt.Sprintf("cannot stat '%s': %v", source, err)
		outputData := []byte(errMsg)
		// Check if io directory exists
		if _, statErr := driverConfig.MemFs.Stat("io"); statErr == nil {
			// Directory exists, open file for append
			if stdioFile, err := driverConfig.MemFs.OpenFile("io/STDIO", os.O_RDWR|os.O_APPEND|os.O_CREATE, 0o644); err == nil {
				stdioFile.Write(outputData)
				stdioFile.Close()
			} else {
				driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &outputData, "io/STDIO")
			}
		} else {
			// Directory doesn't exist, use WriteToMemFile to create it
			driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &outputData, "io/STDIO")
		}
		return errors.New(errMsg)
	}

	// Check if destination is a directory
	if destInfo, err := driverConfig.MemFs.Stat(dest); err == nil {
		if destInfo.IsDir() {
			// Destination is a directory, append source filename
			dest = path.Join(dest, path.Base(source))
		}
	}

	// Copy source to destination
	if sourceInfo.IsDir() {
		// For directories, use recursive copy
		if err := copyDirectory(driverConfig, source, dest); err != nil {
			outputBytes := []byte(err.Error())
			driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &outputBytes, "io/STDIO")
			return err
		}
	} else {
		// For files, use simple copy
		if err := copyFile(driverConfig, source, dest); err != nil {
			outputBytes := []byte(err.Error())
			driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &outputBytes, "io/STDIO")
			return err
		}
	}

	// Remove source after successful copy
	var removeErr error
	if sourceInfo.IsDir() {
		removeErr = removePath(driverConfig, source, true)
	} else {
		removeErr = driverConfig.MemFs.Remove(source)
	}

	if removeErr != nil {
		outputData := []byte(removeErr.Error())
		// Check if io directory exists
		if _, statErr := driverConfig.MemFs.Stat("io"); statErr == nil {
			// Directory exists, open file for append
			if stdioFile, err := driverConfig.MemFs.OpenFile("io/STDIO", os.O_RDWR|os.O_APPEND|os.O_CREATE, 0o644); err == nil {
				stdioFile.Write(outputData)
				stdioFile.Close()
			} else {
				driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &outputData, "io/STDIO")
			}
		} else {
			// Directory doesn't exist, use WriteToMemFile to create it
			driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &outputData, "io/STDIO")
		}
		return removeErr
	}

	// Success message
	outputData := []byte("Files moved successfully\n")
	// Check if io directory exists
	if _, statErr := driverConfig.MemFs.Stat("io"); statErr == nil {
		// Directory exists, open file for append
		if stdioFile, err := driverConfig.MemFs.OpenFile("io/STDIO", os.O_RDWR|os.O_APPEND|os.O_CREATE, 0o644); err == nil {
			stdioFile.Write(outputData)
			stdioFile.Close()
		} else {
			driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &outputData, "io/STDIO")
		}
	} else {
		// Directory doesn't exist, use WriteToMemFile to create it
		driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &outputData, "io/STDIO")
	}
	return nil
}

// ExecuteCat displays the contents of files
func ExecuteCat(args []string, driverConfig *config.DriverConfig) error {
	if driverConfig == nil || driverConfig.MemFs == nil {
		return errors.New("driver config or memfs is nil")
	}

	if len(args) == 0 {
		return errors.New("cat: missing file operand")
	}

	var output strings.Builder

	// Process each file
	for _, filePath := range args {
		// Clean path
		if strings.HasPrefix(filePath, "./") {
			filePath = strings.TrimPrefix(filePath, "./")
		}

		// Check if file exists
		info, err := driverConfig.MemFs.Stat(filePath)
		if err != nil {
			errMsg := fmt.Sprintf("cat: %s: %v\n", filePath, err)
			if driverConfig.CoreConfig != nil && driverConfig.CoreConfig.Log != nil {
				driverConfig.CoreConfig.Log.Print(errMsg)
			}
			output.WriteString(errMsg)
			continue
		}

		// Check if it's a directory
		if info.IsDir() {
			errMsg := fmt.Sprintf("cat: %s: Is a directory\n", filePath)
			if driverConfig.CoreConfig != nil && driverConfig.CoreConfig.Log != nil {
				driverConfig.CoreConfig.Log.Print(errMsg)
			}
			output.WriteString(errMsg)
			continue
		}

		// Open and read file
		file, err := driverConfig.MemFs.Open(filePath)
		if err != nil {
			errMsg := fmt.Sprintf("cat: cannot open %s: %v\n", filePath, err)
			if driverConfig.CoreConfig != nil && driverConfig.CoreConfig.Log != nil {
				driverConfig.CoreConfig.Log.Print(errMsg)
			}
			output.WriteString(errMsg)
			continue
		}

		// Read contents
		buf := new(bytes.Buffer)
		_, err = io.Copy(buf, file)
		file.Close()
		if err != nil {
			errMsg := fmt.Sprintf("cat: error reading %s: %v\n", filePath, err)
			if driverConfig.CoreConfig != nil && driverConfig.CoreConfig.Log != nil {
				driverConfig.CoreConfig.Log.Print(errMsg)
			}
			output.WriteString(errMsg)
			continue
		}

		// Add file contents to output
		output.WriteString(buf.String())
	}

	// Write output to io/STDIO so it can be read as response
	if output.Len() > 0 {
		outputData := []byte(output.String())
		// Check if io directory exists
		if _, statErr := driverConfig.MemFs.Stat("io"); statErr == nil {
			// Directory exists, open file for append
			if stdioFile, err := driverConfig.MemFs.OpenFile("io/STDIO", os.O_RDWR|os.O_APPEND|os.O_CREATE, 0o644); err == nil {
				stdioFile.Write(outputData)
				stdioFile.Close()
			} else {
				driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &outputData, "io/STDIO")
			}
		} else {
			// Directory doesn't exist, use WriteToMemFile to create it
			driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &outputData, "io/STDIO")
		}
	}

	return nil
}

// ExecuteSu performs OAuth authentication for unrestricted write access
func ExecuteSu(driverConfig *config.DriverConfig) error {
	if driverConfig == nil || driverConfig.MemFs == nil {
		return errors.New("driver config or memfs is nil")
	}

	fmt.Printf("ExecuteSu: received driverConfig\n")

	var output strings.Builder

	// Perform OAuth authentication for unrestricted access
	err := eUtils.GetUnrestrictedAccess(driverConfig)
	if err != nil {
		errMsg := fmt.Sprintf("su: authentication failed: %v\n", err)
		if driverConfig.CoreConfig != nil && driverConfig.CoreConfig.Log != nil {
			driverConfig.CoreConfig.Log.Print(errMsg)
		}
		output.WriteString(errMsg)
	} else {
		output.WriteString("success: Elevated access granted.\n")
		output.WriteString("You now have write access to configuration tokens.\n")
	}

	// Write output to io/STDIO so it can be read as response
	if output.Len() > 0 {
		outputData := []byte(output.String())
		// Check if io directory exists
		if _, statErr := driverConfig.MemFs.Stat("io"); statErr == nil {
			// Directory exists, open file for append
			if stdioFile, err := driverConfig.MemFs.OpenFile("io/STDIO", os.O_RDWR|os.O_APPEND|os.O_CREATE, 0o644); err == nil {
				stdioFile.Write(outputData)
				stdioFile.Close()
			} else {
				driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &outputData, "io/STDIO")
			}
		} else {
			// Directory doesn't exist, use WriteToMemFile to create it
			driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &outputData, "io/STDIO")
		}
	}

	return err
}
