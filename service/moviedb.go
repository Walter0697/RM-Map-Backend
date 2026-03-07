package service

import (
	"encoding/json"
	"fmt"
	"mapmarker/backend/config"
	"mapmarker/backend/constant"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/graph/model"
	"mapmarker/backend/helper"
	"net/url"
	"strconv"
)

type MovieDetail struct {
	ID           int    `json:"id"`
	Adult        bool   `json:"adult"`
	OriginTitle  string `json:"original_title"`
	Overview     string `json:"overview"`
	PosterPath   string `json:"poster_path"`
	BackdropPath string `json:"backdrop_path"`
	ReleaseDate  string `json:"release_date"`
}

type MovieResponse struct {
	Page    uint          `json:"page"`
	Results []MovieDetail `json:"results"`
}

const (
	NowPlayingURL string = "/3/movie/now_playing"
	UpcomingURL   string = "/3/movie/upcoming"
	SearchURL     string = "/3/search/movie"
	GetByIdURL    string = "/3/movie/"
)

var movieDBGetRequestWithAuditFn = GetRequestWithExternalAPIAudit

func getRequestLink(suffix string) string {
	return constant.MovieDBAPI + suffix + "?api_key=" + config.Data.APIKEY.MovieDB
}

func getByIdRequest(id int64) string {
	id_str := strconv.FormatInt(id, 10)
	return constant.MovieDBAPI + GetByIdURL + id_str + "?api_key=" + config.Data.APIKEY.MovieDB
}

func GetUpcoming(country *string) (*MovieResponse, error) {
	url := getRequestLink(UpcomingURL)
	if country != nil {
		url = url + "&region=" + *country
	}

	var movieResp MovieResponse
	_, err := movieDBGetRequestWithAuditFn(ExternalAPIProviderMovieDB, "get_upcoming", url, func(body []byte) error {
		if unmarshalErr := json.Unmarshal(body, &movieResp); unmarshalErr != nil {
			return fmt.Errorf("decode response: %w", unmarshalErr)
		}
		return nil
	})
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

	var movieResp MovieResponse
	_, err := movieDBGetRequestWithAuditFn(ExternalAPIProviderMovieDB, "get_now_playing", url, func(body []byte) error {
		if unmarshalErr := json.Unmarshal(body, &movieResp); unmarshalErr != nil {
			return fmt.Errorf("decode response: %w", unmarshalErr)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &movieResp, nil
}

func SearchByName(query string) (*MovieResponse, error) {
	url := getRequestLink(SearchURL) + "&query=" + url.QueryEscape(query)

	var movieResp MovieResponse
	_, err := movieDBGetRequestWithAuditFn(ExternalAPIProviderMovieDB, "search_movie", url, func(body []byte) error {
		if unmarshalErr := json.Unmarshal(body, &movieResp); unmarshalErr != nil {
			return fmt.Errorf("decode response: %w", unmarshalErr)
		}
		return nil
	})
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
		item.RefID = movieDetails.ID
		if movieDetails.PosterPath != "" {
			item.ImageLink = constant.MovieDBAPIImage + movieDetails.PosterPath
		} else if movieDetails.BackdropPath != "" {
			item.ImageLink = constant.MovieDBAPIImage + movieDetails.BackdropPath
		} else {
			item.ImageLink = ""
		}

		item.ReleaseDate = movieDetails.ReleaseDate
		result = append(result, &item)
	}

	return result, nil
}

func FetchMovieByRid(movie_rid int64) (*dbmodel.Movie, error) {
	url := getByIdRequest(movie_rid)

	var movieDetail MovieDetail
	_, err := movieDBGetRequestWithAuditFn(ExternalAPIProviderMovieDB, "get_movie_by_id", url, func(body []byte) error {
		if unmarshalErr := json.Unmarshal(body, &movieDetail); unmarshalErr != nil {
			return fmt.Errorf("decode response: %w", unmarshalErr)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	var output dbmodel.Movie
	output.RefId = movieDetail.ID
	output.Label = movieDetail.OriginTitle
	output.ReleaseDate = &movieDetail.ReleaseDate
	if movieDetail.PosterPath != "" {
		output.ImageLink = constant.MovieDBAPIImage + movieDetail.PosterPath
	} else if movieDetail.BackdropPath != "" {
		output.ImageLink = constant.MovieDBAPIImage + movieDetail.BackdropPath
	} else {
		output.ImageLink = ""
	}

	return &output, nil
}
