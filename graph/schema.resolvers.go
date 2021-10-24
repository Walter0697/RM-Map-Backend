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
	// ADMIN
	// create user if ldap is not enabled

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
	// USER
	// Create marker by user
	// TODO: add relation

	user := middleware.ForContext(ctx)
	if user == nil {
		return "", &helper.PermissionDeniedError{}
	}

	if err := helper.IsAuthorize(*user, helper.User); err != nil {
		return "", err
	}

	_, err := service.CreateMarker(input, *user)
	if err != nil {
		return "", err
	}
	return "ok", nil
}

func (r *mutationResolver) UpdateRelation(ctx context.Context, input model.UpdateRelation) (string, error) {
	// USER
	// update your current preference

	user := middleware.ForContext(ctx)
	if user == nil {
		return "", &helper.PermissionDeniedError{}
	}

	if err := helper.IsAuthorize(*user, helper.User); err != nil {
		return "", err
	}

	if user.Username == input.Username {
		return "", &helper.RelationWithYourselfError{}
	}

	if _, err := service.UpdateRelation(input, *user); err != nil {
		return "", err
	}

	return "ok", nil
}

func (r *mutationResolver) Login(ctx context.Context, input model.Login) (*model.LoginResult, error) {
	// Login function

	token, err := service.Login(input.Username, input.Password)
	if err != nil {
		return nil, err
	}

	return &model.LoginResult{
		Jwt:      token,
		Username: input.Username,
	}, nil
}

func (r *queryResolver) Users(ctx context.Context, filter *model.UserFilter) ([]*model.User, error) {
	// ADMIN
	// get all users for management

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

func (r *queryResolver) Usersearch(ctx context.Context, filter model.UserSearch) (*model.User, error) {
	// USER
	// search user by username, must be exact for a bit security

	user := middleware.ForContext(ctx)
	if user == nil {
		return nil, &helper.PermissionDeniedError{}
	}

	if err := helper.IsAuthorize(*user, helper.User); err != nil {
		return nil, err
	}

	returnUser, err := service.GetUserByExactUsername(filter)
	if err != nil {
		return nil, err
	}

	if returnUser != nil {
		return &model.User{
			ID:       int(returnUser.ID),
			Username: returnUser.Username,
		}, nil
	}
	return nil, nil
}

func (r *queryResolver) Preference(ctx context.Context) (*model.UserPreference, error) {
	// USER
	// get your current preference

	user := middleware.ForContext(ctx)
	if user == nil {
		return nil, &helper.PermissionDeniedError{}
	}

	if err := helper.IsAuthorize(*user, helper.User); err != nil {
		return nil, err
	}

	var preference dbmodel.UserPreference

	preference.UserId = user.ID

	preferencePtr := &preference

	if err := preferencePtr.GetPreferenceByUserId(); err != nil {
		if !utils.RecordNotFound(err) {
			return nil, err
		}
	}

	var result model.UserPreference
	if preferencePtr != nil {
		result.ID = int(preference.ID)
		if preference.RelationId != nil {
			var relation dbmodel.UserRelation
			relation.ID = *preference.RelationId
			if err := relation.GetRelationById(); err == nil {
				user1 := helper.ConvertUser(relation.UserOne)
				user2 := helper.ConvertUser(relation.UserTwo)
				if user.Username == relation.UserOne.Username {
					result.User = &user1
					result.Relation = &user2
				} else {
					result.User = &user2
					result.Relation = &user1
				}
			}
		}
		return &result, nil
	}

	return nil, nil
}

func (r *queryResolver) Markers(ctx context.Context) ([]*model.Marker, error) {
	// USER
	// get markers
	// TODO: make it by relation

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
