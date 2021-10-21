package utils

import (
	"crypto/tls"
	"io"
	"net/http"
	"os"
)

func SaveImageFromURL(path string, url string) error {
	// WARNING: possibly a dangerous solution but this function will only be used by authorized user
	// I can remove this feature or find another way to work with this
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	response, e := client.Get(url)
	if e != nil {
		return e
	}
	defer response.Body.Close()

	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, response.Body)
	if err != nil {
		return err
	}

	return nil
}

func UploadImage(path string, file io.Reader) error {
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, file)
	if err != nil {
		return err
	}
	return nil
}
