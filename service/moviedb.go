package service

import (
	"encoding/json"
	"mapmarker/backend/config"
	"mapmarker/backend/graph/model"
	"mapmarker/backend/helper"
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

func GetUpcoming(country *string) (*MovieResponse, error) {
	url := getRequestLink(UpcomingURL)
	if country != nil {
		url = url + "&region=" + *country
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

func GetNowPlaying(country *string) (*MovieResponse, error) {
	url := getRequestLink(NowPlayingURL)
	if country != nil {
		url = url + "&region=" + *country
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

func GetMovieList(filter model.MovieFilter) ([]*model.MovieOutput, error) {
	var result []*model.MovieOutput
	var data *MovieResponse
	var err error
	if filter.Type == "search" {
		if filter.Query != nil {
			data, err = SearchByName(*filter.Query)
			if err != nil {
				return result, err
			}
		} else {
			return result, &helper.QueryCannotEmptyError{}
		}
	} else if filter.Type == "nowplaying" {
		data, err = GetNowPlaying(filter.Location)
		if err != nil {
			return result, err
		}
	} else if filter.Type == "upcoming" {
		data, err = GetUpcoming(filter.Location)
		if err != nil {
			return result, err
		}
	}

	for _, movieDetails := range data.Results {
		var item model.MovieOutput
		item.Title = movieDetails.OriginTitle
		item.ImageLink = config.Data.MovieDB.ImageLink + movieDetails.PosterPath
		item.ReleaseDate = movieDetails.ReleaseDate
		result = append(result, &item)
	}

	return result, nil
}
