package repository

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"net/http"
)

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
