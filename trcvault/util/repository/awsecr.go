//go:build awsecr
// +build awsecr

package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
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
		RepositoryName: aws.String(pluginToolConfig["pluginNamePtr"].(string)),
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
		fmt.Println(err.Error())
		os.Exit(1)
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
		RepositoryName: aws.String(pluginToolConfig["pluginNamePtr"].(string)),
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
