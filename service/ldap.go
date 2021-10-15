package service

import (
	"encoding/base64"
	"mapmarker/backend/config"
	"net/http"
)

func LDAP(username string, password string) error {
	r, err := http.NewRequest("GET", "", nil)
	if err != nil {
		return err
	}
	r.Header.Set("Authorization", "Basic "+basicAuth(username, password))
	_, err = config.Strategy.Authenticate(r.Context(), r)
	if err != nil {
		return err
	}
	return nil
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}
