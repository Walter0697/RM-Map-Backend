package service

import (
	"encoding/json"
	"mapmarker/backend/config"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/graph/model"
	"mapmarker/backend/helper"
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

func getRequestLink(suffix string) string {
	return config.Data.MovieDB.ApiLink + suffix + "?api_key=" + config.Data.MovieDB.ApiKey
}

func getByIdRequest(id int64) string {
	id_str := strconv.FormatInt(id, 10)
	return config.Data.MovieDB.ApiLink + GetByIdURL + id_str + "?api_key=" + config.Data.MovieDB.ApiKey
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
		item.RefID = movieDetails.ID
		if movieDetails.PosterPath != "" {
			item.ImageLink = config.Data.MovieDB.ImageLink + movieDetails.PosterPath
		} else if movieDetails.BackdropPath != "" {
			item.ImageLink = config.Data.MovieDB.ImageLink + movieDetails.BackdropPath
		} else {
			item.ImageLink = ""
		}

		item.ReleaseDate = movieDetails.ReleaseDate
		result = append(result, &item)
	}

	return result, nil
}

func GetMovieByRid(movie_rid int64) (*dbmodel.Movie, error) {
	url := getByIdRequest(movie_rid)

	body, err := GetRequest(url)
	if err != nil {
		return nil, err
	}

	var movieDetail MovieDetail
	err = json.Unmarshal(body, &movieDetail)
	if err != nil {
		return nil, err
	}

	var output dbmodel.Movie
	output.RefId = movieDetail.ID
	output.Label = movieDetail.OriginTitle
	output.ReleaseDate = &movieDetail.ReleaseDate
	output.ImageLink = movieDetail.PosterPath // it won't be the one to be saved inside db but act as temporary object

	return &output, nil
}
