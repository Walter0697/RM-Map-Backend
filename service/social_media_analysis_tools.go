package service

import (
	"fmt"
	"strings"
)

var socialMediaGeocodeAddressFn = GeocodeStreetAddress

type LocationResolution struct {
	LocationCandidate
	Geocoded bool `json:"geocoded"`
}

func ResolveLocationCandidate(candidate LocationCandidate) (LocationResolution, error) {
	if candidate.Latitude != 0 || candidate.Longitude != 0 {
		return LocationResolution{LocationCandidate: candidate}, nil
	}
	address := strings.TrimSpace(candidate.Address)
	if address == "" {
		return LocationResolution{}, fmt.Errorf("location address is required")
	}
	parts := strings.Split(address, ",")
	if len(parts) < 2 {
		return LocationResolution{}, fmt.Errorf("location address must include street and country")
	}
	street := strings.TrimSpace(parts[0])
	country := strings.TrimSpace(parts[len(parts)-1])
	fields := strings.Fields(street)
	if len(fields) < 2 || strings.TrimSpace(country) == "" {
		return LocationResolution{}, fmt.Errorf("location address must include street number, street name, and country")
	}
	latitude, longitude, err := socialMediaGeocodeAddressFn(fields[0], strings.Join(fields[1:], " "), country)
	if err != nil {
		return LocationResolution{}, fmt.Errorf("geocode location: %w", err)
	}
	candidate.Latitude = latitude
	candidate.Longitude = longitude
	return LocationResolution{LocationCandidate: candidate, Geocoded: true}, nil
}
