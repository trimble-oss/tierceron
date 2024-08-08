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
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
)

func BuildDockerImage(driverConfig *eUtils.DriverConfig, dockerfilePath, imageName string) error {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}

	// Create a tar archive of the Dockerfile
	dockerfileTar, err := createTar(dockerfilePath)
	if err != nil {
		return err
	}

	buildOptions := types.ImageBuildOptions{
		Context:    dockerfileTar,
		Dockerfile: filepath.Base(dockerfilePath),
		Tags:       []string{imageName},
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

func createTar(dockerfilePath string) (io.Reader, error) {
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	defer tw.Close()

	dockerfile, err := os.Open(dockerfilePath)
	if err != nil {
		return nil, err
	}
	defer dockerfile.Close()

	stat, err := dockerfile.Stat()
	if err != nil {
		return nil, err
	}

	header := &tar.Header{
		Name: filepath.Base(dockerfilePath),
		Size: stat.Size(),
	}

	if err := tw.WriteHeader(header); err != nil {
		return nil, err
	}

	if _, err := io.Copy(tw, dockerfile); err != nil {
		return nil, err
	}

	return buf, nil
}
