//go:build awsecr
// +build awsecr

package repository

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
)

func getImageSHA(svc *ecr.ECR, pluginToolConfig map[string]interface{}) error {
	imageInput := &ecr.BatchGetImageInput{
		ImageIds: []*ecr.ImageIdentifier{
			{
				ImageTag: aws.String("latest"),
			},
		},
		RepositoryName: aws.String(pluginToolConfig["trcplugin"].(string)),
		RegistryId:     aws.String(strings.Split(pluginToolConfig["ecrrepository"].(string), ".")[0]),
	}

	batchImages, err := svc.BatchGetImage(imageInput)
	if err != nil {
		var errorString string
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ecr.ErrCodeServerException:
				errorString = aerr.Error()
			case ecr.ErrCodeInvalidParameterException:
				errorString = aerr.Error()
			case ecr.ErrCodeRepositoryNotFoundException:
				errorString = aerr.Error()
			default:
				errorString = aerr.Error()
			}
		} else {
			if err != nil {
				return err
			}
		}
		return errors.New(errorString)
	}

	var layerDigest string
	var data map[string]interface{}
	err = json.Unmarshal([]byte(*batchImages.Images[0].ImageManifest), &data)
	if err != nil {
		return errors.New(err.Error())
	}

	layers := data["layers"].([]interface{})
	for _, layerMetadata := range layers {
		mapLayerMetdata := layerMetadata.(map[string]interface{})
		layerDigest = mapLayerMetdata["digest"].(string)
	}

	pluginToolConfig["layerDigest"] = layerDigest
	return nil
}

// Return url to the image to be used for download.
func GetImageDownloadUrl(pluginToolConfig map[string]interface{}) (string, error) {
	svc := ecr.New(session.New(&aws.Config{
		Region:      aws.String("us-west-2"),
		Credentials: credentials.NewStaticCredentials(pluginToolConfig["awspassword"].(string), pluginToolConfig["awsaccesskey"].(string), ""),
	}))

	err := getImageSHA(svc, pluginToolConfig)
	if err != nil {
		return "", err
	}
	downloadInput := &ecr.GetDownloadUrlForLayerInput{
		LayerDigest:    aws.String(pluginToolConfig["layerDigest"].(string)),
		RegistryId:     aws.String(strings.Split(pluginToolConfig["ecrrepository"].(string), ".")[0]),
		RepositoryName: aws.String(pluginToolConfig["trcplugin"].(string)),
	}

	downloadOutput, err := svc.GetDownloadUrlForLayer(downloadInput)
	if err != nil {
		var errorString string
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ecr.ErrCodeServerException:
				errorString = aerr.Error()
			case ecr.ErrCodeInvalidParameterException:
				errorString = aerr.Error()
			case ecr.ErrCodeRepositoryNotFoundException:
				errorString = aerr.Error()
			default:
				errorString = aerr.Error()
			}
		} else {
			if err != nil {
				return "", err
			}
		}
		return "", errors.New(errorString)
	}

	return *downloadOutput.DownloadUrl, nil
}

func gUnZipData(data []byte) ([]byte, error) {
	var unCompressedBytes []byte
	newB := bytes.NewBuffer(unCompressedBytes)
	b := bytes.NewBuffer(data)
	zr, err := gzip.NewReader(b)
	if err != nil {
		return nil, err
	}

	if _, err := io.Copy(newB, zr); err != nil {
		return nil, err
	}

	return newB.Bytes(), nil
}

func untarData(data []byte) ([]byte, error) {
	var b bytes.Buffer
	writer := io.Writer(&b)
	tarReader := tar.NewReader(bytes.NewReader(data))
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

func getImage(downloadUrl string) ([]byte, error) {
	response, err := http.Get(downloadUrl)
	if err != nil {
		return nil, err
	}
	if response.Body != nil {
		defer response.Body.Close()
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func GetImageAndShaFromDownload(pluginToolConfig map[string]interface{}) error {
	downloadUrl, downloadURlError := GetImageDownloadUrl(pluginToolConfig)
	if downloadURlError != nil {
		return errors.New("Failed to get download url.")
	}
	pluginImageDataCompressed, downloadError := getImage(downloadUrl)
	if downloadError != nil {
		return errors.New("Failed to get download from url.")
	}
	pluginTarredData, gUnZipError := gUnZipData(pluginImageDataCompressed)
	if gUnZipError != nil {
		return errors.New("Gunzip failed.")
	}
	pluginImage, gUnTarError := untarData(pluginTarredData)
	if gUnTarError != nil {
		return errors.New("Untarring failed.")
	}
	pluginSha := sha256.Sum256(pluginImage)
	pluginToolConfig["rawImageFile"] = pluginImage
	pluginToolConfig["imagesha256"] = fmt.Sprintf("%x", pluginSha)
	return nil
}
