package repository

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	trcvutils "github.com/trimble-oss/tierceron/pkg/core/util"
)

func gUnZipData(data *[]byte) ([]byte, error) {
	var unCompressedBytes []byte
	newB := bytes.NewBuffer(unCompressedBytes)
	b := bytes.NewBuffer(*data)
	zr, err := gzip.NewReader(b)
	if err != nil {
		return nil, err
	}

	if _, err := io.Copy(newB, zr); err != nil {
		return nil, err
	}

	return newB.Bytes(), nil
}

func untarData(data *[]byte) ([]byte, error) {
	var b bytes.Buffer
	writer := io.Writer(&b)
	tarReader := tar.NewReader(bytes.NewReader(*data))
	for {
		_, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		_, err = io.Copy(writer, tarReader)
		if err != nil {
			return nil, err
		}
	}
	return b.Bytes(), nil
}

func getImage(downloadUrl string) (*[]byte, error) {
	response, err := http.Get(downloadUrl)
	if err != nil {
		return nil, err
	}
	if response.Body != nil {
		defer response.Body.Close()
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	return &body, nil
}

func deployImage(reader *tar.Reader, pluginToolConfig map[string]interface{}) error {
	// Write the image to the destination...
	var deployPath string
	var deployRoot string
	if deploySubPath, ok := pluginToolConfig["trcdeploysubpath"]; ok {
		deployRoot = filepath.Join(pluginToolConfig["trcdeployroot"].(string), deploySubPath.(string))
	} else {
		deployRoot = pluginToolConfig["trcdeployroot"].(string)
	}

	//check if there is a place holder, if there is replace it
	if strings.Contains(deployRoot, "{{.trcpathparam}}") {
		if pathParam, ok := pluginToolConfig["trcpathparam"].(string); ok && pathParam != "" {
			r, _ := regexp.Compile("^[a-zA-Z0-9_]*$")
			if !r.MatchString(pathParam) {
				fmt.Println("trcpathparam can only contain alphanumberic characters or underscores")
				return errors.New("trcpathparam can only contain alphanumberic characters or underscores")
			}
			deployRoot = strings.Replace(deployRoot, "{{.trcpathparam}}", pathParam, -1)
		} else {
			return errors.New("Unable to replace path placeholder with pathParam.")
		}
	}
	deployPath = filepath.Join(deployRoot, pluginToolConfig["trccodebundle"].(string))

	fmt.Printf("Deploying image to: %s\n", deployPath)

	if _, err := os.Stat(deployRoot); err != nil {
		err = os.MkdirAll(deployRoot, 0644)
		if err != nil {
			fmt.Println(err.Error())
			fmt.Println("Could not prepare needed directory for deployment.")
			return err
		}
	}

	// Create a file from deployPath and make copy the reader directly into file pointing at deployPath...
	// err := os.WriteFile(deployPath, pluginToolConfig["rawImageFile"].([]byte), 0644)
	f, err := os.OpenFile(deployPath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		fmt.Println("Could not create file for deployment.")
		return err
	}
	if err := f.Close(); err != nil {
		fmt.Println("Could not create file for deployment.")
		return err
	}
	for {
		_, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println(err.Error())
			fmt.Println("Image write failure.")
			return err
		}

		_, err = io.Copy(f, reader)
		if err != nil {
			fmt.Println(err.Error())
			fmt.Println("Image write failure.")
			return err
		}
	}

	if expandTarget, ok := pluginToolConfig["trcexpandtarget"].(string); ok && expandTarget == "true" {
		// TODO: provide archival of existing directory.
		if ok, errList := trcvutils.UncompressZipFile(deployPath); !ok {
			fmt.Printf("Uncompressing zip file in place failed. %v\n", errList)
			return errList[0]
		} else {
			os.Remove(deployPath)
		}
	} else {
		if strings.HasSuffix(deployPath, ".war") {
			explodedWarPath := strings.TrimSuffix(deployPath, ".war")
			fmt.Printf("Checking exploded war path: %s\n", explodedWarPath)
			if _, err := os.Stat(explodedWarPath); err == nil {
				if deploySubPath, ok := pluginToolConfig["trcdeploysubpath"]; ok {
					archiveDirPath := filepath.Join(deployRoot, "archive")
					fmt.Printf("Verifying archive directory: %s\n", archiveDirPath)
					err := os.MkdirAll(archiveDirPath, 0700)
					if err == nil {
						currentTime := time.Now()
						formattedTime := fmt.Sprintf("%d-%02d-%02d_%02d-%02d-%02d", currentTime.Year(), currentTime.Month(), currentTime.Day(), currentTime.Hour(), currentTime.Minute(), currentTime.Second())
						archiveRoot := filepath.Join(pluginToolConfig["trcdeployroot"].(string), deploySubPath.(string), "archive", formattedTime)
						fmt.Printf("Verifying archive backup directory: %s\n", archiveRoot)
						err := os.MkdirAll(archiveRoot, 0700)
						if err == nil {
							archivePath := filepath.Join(archiveRoot, pluginToolConfig["trccodebundle"].(string))
							archivePath = strings.TrimSuffix(archivePath, ".war")
							fmt.Printf("Archiving: %s to %s\n", explodedWarPath, archivePath)
							os.Rename(explodedWarPath, archivePath)
						}
					}
				}
			}
		}
	}

	fmt.Printf("Image deployed to: %s\n", deployPath)

	return nil
}
