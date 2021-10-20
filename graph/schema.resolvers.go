package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"mapmarker/backend/config"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/graph/generated"
	"mapmarker/backend/graph/model"
	"mapmarker/backend/helper"
	"mapmarker/backend/middleware"
	"mapmarker/backend/service"
	"mapmarker/backend/utils"
)

func (r *mutationResolver) CreateUser(ctx context.Context, input model.NewUser) (string, error) {
	if config.Data.LDAP.Enable {
		return "", &helper.LDAPLoginEnabledError{}
	}

	user := middleware.ForContext(ctx)
	if user == nil {
		return "", &helper.PermissionDeniedError{}
	}

	if err := helper.IsAuthorize(*user, helper.Admin); err != nil {
		return "", err
	}

	if !helper.IsValidRoles(input.Role) {
		return "", &helper.RoleInvalidError{}
	}

	var newuser dbmodel.User
	newuser.Username = input.Username
	if newuser.CheckUsernameExist() {
		return "", &helper.SameUserNameExistError{}
	}

	_, err := service.CreateUser(input)
	if err != nil {
		return "", err
	}
	return "ok", nil
}

func (r *mutationResolver) CreateMarker(ctx context.Context, input model.NewMarker) (string, error) {
	user := middleware.ForContext(ctx)
	if user == nil {
		return "", &helper.PermissionDeniedError{}
	}

	if err := helper.IsAuthorize(*user, helper.User); err != nil {
		return "", err
	}

	// var user dbmodel.User
	// user.ID = uint(input.UserID)
	// if err := user.GetUserById(); err != nil {
	// 	return "", helper.GetDatabaseError(err)
	// }

	_, err := service.CreateMarker(input, *user)
	if err != nil {
		return "", err
	}
	return "ok", nil
}

func (r *mutationResolver) Login(ctx context.Context, input model.Login) (string, error) {
	token, err := service.Login(input.Username, input.Password)
	if err != nil {
		return "", err
	}
	return token, nil
}

func (r *queryResolver) Users(ctx context.Context, filter *model.UserFilter) ([]*model.User, error) {
	var result []*model.User

	user := middleware.ForContext(ctx)
	if user == nil {
		return result, &helper.PermissionDeniedError{}
	}

	if err := helper.IsAuthorize(*user, helper.Admin); err != nil {
		return result, err
	}

	users, err := service.GetAllUser(filter)
	if err != nil {
		return result, err
	}

	for _, user := range users {
		item := helper.ConvertUser(user)
		result = append(result, &item)
	}

	return result, nil
}

func (r *queryResolver) Markers(ctx context.Context) ([]*model.Marker, error) {
	var result []*model.Marker
	user := middleware.ForContext(ctx)
	if user == nil {
		return result, &helper.PermissionDeniedError{}
	}

	if err := helper.IsAuthorize(*user, helper.User); err != nil {
		return result, err
	}

	requested_field := utils.GetTopPreloads(ctx)

	markers, err := service.GetAllActiveMarker(requested_field)
	if err != nil {
		return result, err
	}

	for _, marker := range markers {
		item := helper.ConvertMarker(marker)
		result = append(result, &item)
	}

	return result, nil
}

// Mutation returns generated.MutationResolver implementation.
func (r *Resolver) Mutation() generated.MutationResolver { return &mutationResolver{r} }

// Query returns generated.QueryResolver implementation.
func (r *Resolver) Query() generated.QueryResolver { return &queryResolver{r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
