package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"fmt"
	"mapmarker/backend/graph/generated"
	"mapmarker/backend/graph/model"
)

func (r *mutationResolver) CreateUser(ctx context.Context, input model.NewUser) (string, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) UpdateRelation(ctx context.Context, input model.UpdateRelation) (string, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) UpdatePreferredPin(ctx context.Context, input model.UpdatePreferredPin) (*model.UserPreference, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) CreateMarker(ctx context.Context, input model.NewMarker) (*model.Marker, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) EditMarker(ctx context.Context, input model.UpdateMarker) (*model.Marker, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) RemoveMarker(ctx context.Context, input model.RemoveModel) (string, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) UpdateMarkerFav(ctx context.Context, input model.UpdateMarkerFavourite) (*model.Marker, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) CreateMarkerType(ctx context.Context, input model.NewMarkerType) (*model.MarkerType, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) EditMarkerType(ctx context.Context, input model.UpdatedMarkerType) (*model.MarkerType, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) RemoveMarkerType(ctx context.Context, input model.RemoveModel) (string, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) CreatePin(ctx context.Context, input model.NewPin) (*model.Pin, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) EditPin(ctx context.Context, input model.UpdatedPin) (*model.Pin, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) PreviewPin(ctx context.Context, input model.PreviewPinInput) (string, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) RemovePin(ctx context.Context, input model.RemoveModel) (string, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) UpdateDefault(ctx context.Context, input model.UpdatedDefault) (string, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) CreateSchedule(ctx context.Context, input model.NewSchedule) (*model.Schedule, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) CreateMovieSchedule(ctx context.Context, input model.NewMovieSchedule) (*model.Schedule, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) EditSchedule(ctx context.Context, input model.UpdateSchedule) (*model.Schedule, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) UpdateScheduleStatus(ctx context.Context, input model.ScheduleStatusList) ([]*model.Schedule, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) RemoveSchedule(ctx context.Context, input model.RemoveModel) (*model.Marker, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) RevokeMarker(ctx context.Context, input model.UpdateModel) (*model.Marker, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) WebsiteScrap(ctx context.Context, input model.WebsiteScrapInput) (*model.WebsiteScrapResult, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) UpdateStation(ctx context.Context, input model.UpdateStation) (*model.Station, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) CreateFavouriteMovie(ctx context.Context, input model.NewFavouriteMovie) (*model.Movie, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) RemoveFavouriteMovie(ctx context.Context, input model.RemoveModel) (string, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) CreateCountryPoint(ctx context.Context, input model.NewCountryPoint) (*model.CountryPoint, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) CreateCountryLocation(ctx context.Context, input model.NewCountryLocation) (*model.CountryLocation, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) Login(ctx context.Context, input model.Login) (*model.LoginResult, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) Logout(ctx context.Context, input model.Logout) (string, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) Users(ctx context.Context, filter *model.UserFilter) ([]*model.User, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) Usersearch(ctx context.Context, filter model.UserSearch) (*model.User, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) Preference(ctx context.Context) (*model.UserPreference, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) Markers(ctx context.Context) ([]*model.Marker, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) Markertypes(ctx context.Context) ([]*model.MarkerType, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) Eventtypes(ctx context.Context) ([]*model.EventType, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) Pins(ctx context.Context) ([]*model.Pin, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) Defaultpins(ctx context.Context) ([]*model.DefaultPin, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) Mappins(ctx context.Context) ([]*model.MapPin, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) Schedules(ctx context.Context, params model.CurrentTime) ([]*model.Schedule, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) Movies(ctx context.Context) ([]*model.Movie, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) Today(ctx context.Context, params model.CurrentTime) (*model.TodayEvent, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) Previousmarkers(ctx context.Context) ([]*model.Marker, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) Expiredmarkers(ctx context.Context) ([]*model.Marker, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) Markerschedules(ctx context.Context, params model.IDModel) ([]*model.Schedule, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) Scrapimage(ctx context.Context, params model.WebLink) (*model.MetaDataOutput, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) Moviefetch(ctx context.Context, filter model.MovieFilter) ([]*model.MovieOutput, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) Latestreleasenote(ctx context.Context) (*model.ReleaseNote, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) Specificreleasenote(ctx context.Context, filter model.ReleaseNoteFilter) (*model.ReleaseNote, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) Releasenotes(ctx context.Context) ([]*model.ReleaseNote, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) Stations(ctx context.Context) ([]*model.Station, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) Countrycodemap(ctx context.Context) ([]*model.CountryCodeMap, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) Watchedmovies(ctx context.Context) ([]*model.Schedule, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) Countrypoints(ctx context.Context) ([]*model.CountryPoint, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) Countrylocations(ctx context.Context) ([]*model.CountryLocation, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) Me(ctx context.Context) (string, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) Viewportmarkers(ctx context.Context, params model.MarkerViewportQuery) (*model.MarkerPage, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) Pagedschedules(ctx context.Context, params model.PagedScheduleQuery) (*model.SchedulePage, error) {
	panic(fmt.Errorf("not implemented"))
}

// Mutation returns generated.MutationResolver implementation.
func (r *Resolver) Mutation() generated.MutationResolver { return &mutationResolver{r} }

// Query returns generated.QueryResolver implementation.
func (r *Resolver) Query() generated.QueryResolver { return &queryResolver{r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
