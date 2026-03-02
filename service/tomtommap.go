package service

import (
	"encoding/json"
	"fmt"
	"mapmarker/backend/config"
	"mapmarker/backend/constant"
)

type AddressInfo struct {
	Country     string `json:"country"`
	CountryCode string `json:"countryCode"`
	LocalName   string `json:"localName"`
}

type AddressWrapper struct {
	Address AddressInfo `json:"address"`
}

type TomTomResponse struct {
	Addresses []AddressWrapper `json:"addresses"`
}

const (
	ReverseGeocodeURL string = "/2/reverseGeocode"
)

func reverseGeocodeRequest(lat, lon float64) string {
	latstr := fmt.Sprintf("%f", lat)
	lonstr := fmt.Sprintf("%f", lon)
	return constant.TomtomMapAPI + ReverseGeocodeURL + "/" + latstr + "," + lonstr + ".json?key=" + config.Data.APIKEY.TomTomMap
}

func GetReverseGeocode(lat, lon float64) (*TomTomResponse, error) {
	url := reverseGeocodeRequest(lat, lon)

	var tomtomResp TomTomResponse
	_, err := GetRequestWithExternalAPIAudit(ExternalAPIProviderTomTomMap, "reverse_geocode", url, func(body []byte) error {
		if unmarshalErr := json.Unmarshal(body, &tomtomResp); unmarshalErr != nil {
			return fmt.Errorf("decode response: %w", unmarshalErr)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &tomtomResp, nil
}
