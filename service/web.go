package service

import (
	"io/ioutil"
	"net/http"
)

func GetRequest(url string) ([]byte, error) {
	var body []byte

	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return body, err
	}

	client := &http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		return body, err
	}

	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return body, err
	}

	defer resp.Body.Close()

	return body, nil
}
