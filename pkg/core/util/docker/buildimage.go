package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
)

func BuildDockerImage(driverConfig *config.DriverConfig, dockerfilePath, imageName string) error {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}

	// Create a tar archive of the Dockerfile
	cwd, err := os.Getwd()

	if err != nil {
		return err
	}

	dockerfileTar, err := createTarContext(cwd)
	if err != nil {
		return err
	}

	buildOptions := types.ImageBuildOptions{
		Context:    dockerfileTar,
		Dockerfile: dockerfilePath,
		Tags:       []string{imageName},
		Remove:     true,
	}

	response, err := cli.ImageBuild(ctx, dockerfileTar, buildOptions)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	// Print the build response
	if _, err := io.Copy(os.Stdout, response.Body); err != nil {
		return err
	}

	return nil
}

func createTarContext(contextDir string) (io.Reader, error) {
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	defer tw.Close()

	err := filepath.Walk(contextDir, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Create a new header from the file info
		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			return err
		}

		// Update the header name to the relative path
		header.Name, err = filepath.Rel(contextDir, file)
		if err != nil {
			return err
		}

		// Write the header to the tarball
		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		// If the file is not a directory, write its content to the tarball
		if !fi.IsDir() {
			data, err := os.Open(file)
			if err != nil {
				return err
			}
			defer data.Close()

			if _, err := io.Copy(tw, data); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return buf, nil
}
