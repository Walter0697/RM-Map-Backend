package service

import (
	"encoding/json"
	"mapmarker/backend/config"
)

type MovieDetail struct {
	Adult       bool   `json:"adult"`
	OriginTitle string `json:"original_title"`
	Overview    string `json:"overview"`
	PosterPath  string `json:"poster_path"`
	ReleaseDate string `json:"release_date"`
}

type MovieResponse struct {
	Page    uint          `json:"page"`
	Results []MovieDetail `json:"results"`
}

const (
	NowPlayingURL string = "/3/movie/now_playing"
	UpcomingURL   string = "/3/movie/upcoming"
	SearchURL     string = "/3/search/movie"
)

func getRequestLink(suffix string) string {
	return config.Data.MovieDB.ApiLink + suffix + "?api_key=" + config.Data.MovieDB.ApiKey
}

func GetUpcoming(country string) (*MovieResponse, error) {
	url := getRequestLink(UpcomingURL)
	if country != "" {
		url = url + "&region=" + country
	}
	body, err := GetRequest(url)
	if err != nil {
		return nil, err
	}

	var movieResp MovieResponse

	err = json.Unmarshal(body, &movieResp)
	if err != nil {
		return nil, err
	}

	return &movieResp, nil
}

func GetNowPlaying(country string) (*MovieResponse, error) {
	url := getRequestLink(NowPlayingURL)
	if country != "" {
		url = url + "&region=" + country
	}

	body, err := GetRequest(url)
	if err != nil {
		return nil, err
	}

	var movieResp MovieResponse

	err = json.Unmarshal(body, &movieResp)
	if err != nil {
		return nil, err
	}

	return &movieResp, nil
}

func SearchByName(query string) (*MovieResponse, error) {
	url := getRequestLink(SearchURL) + "&query=" + query

	body, err := GetRequest(url)
	if err != nil {
		return nil, err
	}

	var movieResp MovieResponse

	err = json.Unmarshal(body, &movieResp)
	if err != nil {
		return nil, err
	}

	return &movieResp, nil
}
