package service

import (
	"math"
	"regexp"
	"strings"
)

type LocationVerificationStatus string

const (
	LocationMatch     LocationVerificationStatus = "match"
	LocationAmbiguous LocationVerificationStatus = "ambiguous"
	LocationNoMatch   LocationVerificationStatus = "no_match"
)

type LocationVerification struct {
	Status     LocationVerificationStatus
	Candidate  *GooglePlaceCandidate
	Candidates []GooglePlaceCandidate
	Reason     string
}

var locationTokenPattern = regexp.MustCompile(`[^[:alnum:]]+`)

func VerifyLocation(agent LocationCandidate, places []GooglePlaceCandidate) LocationVerification {
	if len(places) == 0 {
		return LocationVerification{Status: LocationNoMatch, Reason: "Google Places returned no candidates"}
	}
	accepted := make([]GooglePlaceCandidate, 0, len(places))
	for _, place := range places {
		if locationCandidateMatches(agent, place) {
			accepted = append(accepted, place)
		}
	}
	if len(accepted) == 1 {
		return LocationVerification{Status: LocationMatch, Candidate: &accepted[0], Candidates: accepted, Reason: "one Google Places candidate matched"}
	}
	if len(accepted) > 1 {
		return LocationVerification{Status: LocationAmbiguous, Candidates: accepted, Reason: "multiple Google Places candidates matched"}
	}
	return LocationVerification{Status: LocationNoMatch, Candidates: places, Reason: "no Google Places candidate matched"}
}

func locationCandidateMatches(agent LocationCandidate, place GooglePlaceCandidate) bool {
	nameScore := locationTokenOverlap(agent.Name, place.Name)
	addressScore := locationTokenOverlap(agent.Address, place.Address)
	hasAgentCoordinates := validLocationCoordinates(agent.Latitude, agent.Longitude)
	hasPlaceCoordinates := validLocationCoordinates(place.Latitude, place.Longitude)
	if hasAgentCoordinates && hasPlaceCoordinates && haversineKilometers(agent.Latitude, agent.Longitude, place.Latitude, place.Longitude) > 25 {
		return false
	}
	if nameScore < 0.5 {
		return false
	}
	return addressScore > 0 || (hasAgentCoordinates && hasPlaceCoordinates && haversineKilometers(agent.Latitude, agent.Longitude, place.Latitude, place.Longitude) <= 5)
}

func locationTokenOverlap(left, right string) float64 {
	leftTokens := locationTokens(left)
	rightTokens := locationTokens(right)
	if len(leftTokens) == 0 || len(rightTokens) == 0 {
		return 0
	}
	matched := 0
	for token := range leftTokens {
		if _, ok := rightTokens[token]; ok {
			matched++
		}
	}
	minimum := len(leftTokens)
	if len(rightTokens) < minimum {
		minimum = len(rightTokens)
	}
	return float64(matched) / float64(minimum)
}

func locationTokens(value string) map[string]struct{} {
	value = strings.ToLower(strings.TrimSpace(value))
	value = locationTokenPattern.ReplaceAllString(value, " ")
	result := make(map[string]struct{})
	for _, token := range strings.Fields(value) {
		if len(token) > 1 {
			result[token] = struct{}{}
		}
	}
	return result
}

func validLocationCoordinates(latitude, longitude float64) bool {
	return latitude >= -90 && latitude <= 90 && longitude >= -180 && longitude <= 180 && (latitude != 0 || longitude != 0)
}

func haversineKilometers(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusKm = 6371.0
	latDelta := (lat2 - lat1) * math.Pi / 180
	lonDelta := (lon2 - lon1) * math.Pi / 180
	lat1Radians := lat1 * math.Pi / 180
	lat2Radians := lat2 * math.Pi / 180
	a := math.Sin(latDelta/2)*math.Sin(latDelta/2) + math.Cos(lat1Radians)*math.Cos(lat2Radians)*math.Sin(lonDelta/2)*math.Sin(lonDelta/2)
	return earthRadiusKm * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
