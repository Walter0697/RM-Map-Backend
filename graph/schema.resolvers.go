package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"mapmarker/backend/config"
	"mapmarker/backend/constant"
	"mapmarker/backend/database"
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
	if newuser.CheckUsernameExist(database.Connection) {
		return "", &helper.SameUserNameExistError{}
	}

	_, err := service.CreateUser(input)
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

func (r *mutationResolver) UpdatePreferredPin(ctx context.Context, input model.UpdatePreferredPin) (*model.UserPreference, error) {
	// USER
	// update your current pin preference

	user := middleware.ForContext(ctx)
	if user == nil {
		return nil, &helper.PermissionDeniedError{}
	}

	if err := helper.IsAuthorize(*user, helper.User); err != nil {
		return nil, err
	}

	preference, err := service.UpdatePreferredPin(input, *user)
	if err != nil {
		return nil, err
	}

	if err := preference.GetByUserId(database.Connection); err != nil {
		if !utils.RecordNotFound(err) {
			return nil, err
		}
	}

	defaultPins, err := service.GetAllDefaultPins()
	if err != nil {
		return nil, err
	}

	output := helper.UserPreferencePin(preference, defaultPins)

	return &output, nil
}

func (r *mutationResolver) CreateMarker(ctx context.Context, input model.NewMarker) (*model.Marker, error) {
	// USER
	// Create marker by user

	user := middleware.ForContext(ctx)
	if user == nil {
		return nil, &helper.PermissionDeniedError{}
	}

	if err := helper.IsAuthorize(*user, helper.User); err != nil {
		return nil, err
	}

	relation, err := service.GetCurrentRelation(*user)
	if relation == nil {
		if err == nil {
			return nil, &helper.RelationNotFoundError{}
		}
		return nil, helper.CheckDatabaseError(err, &helper.RelationNotFoundError{})
	}

	marker, err := service.CreateMarker(input, *user, *relation)
	if err != nil {
		return nil, err
	}

	output := helper.ConvertMarker(*marker)
	return &output, nil
}

func (r *mutationResolver) UpdateMarkerFav(ctx context.Context, input model.UpdateMarkerFavourite) (*model.Marker, error) {
	// UESR
	// update marker favourite

	user := middleware.ForContext(ctx)
	if user == nil {
		return nil, &helper.PermissionDeniedError{}
	}

	if err := helper.IsAuthorize(*user, helper.User); err != nil {
		return nil, err
	}

	marker, err := service.UpdateMarkerFavourite(input, *user)
	if err != nil {
		return nil, err
	}

	output := helper.ConvertMarker(*marker)

	return &output, nil
}

func (r *mutationResolver) CreateMarkerType(ctx context.Context, input model.NewMarkerType) (*model.MarkerType, error) {
	// ADMIN
	// Create marker type

	user := middleware.ForContext(ctx)
	if user == nil {
		return nil, &helper.PermissionDeniedError{}
	}

	if err := helper.IsAuthorize(*user, helper.Admin); err != nil {
		return nil, err
	}

	markertype, err := service.CreateMarkerType(input, *user)
	if err != nil {
		return nil, err
	}

	output := helper.ConvertMarkerType(*markertype)

	return &output, nil
}

func (r *mutationResolver) EditMarkerType(ctx context.Context, input model.UpdatedMarkerType) (*model.MarkerType, error) {
	// ADMIN
	// Edit marker type

	user := middleware.ForContext(ctx)
	if user == nil {
		return nil, &helper.PermissionDeniedError{}
	}

	if err := helper.IsAuthorize(*user, helper.Admin); err != nil {
		return nil, err
	}

	markertype, err := service.EditMarkerType(input, *user)
	if err != nil {
		return nil, err
	}

	output := helper.ConvertMarkerType(*markertype)

	return &output, nil
}

func (r *mutationResolver) RemoveMarkerType(ctx context.Context, input model.RemoveModel) (string, error) {
	// ADMIN
	// Remove marker type

	user := middleware.ForContext(ctx)
	if user == nil {
		return "", &helper.PermissionDeniedError{}
	}

	if err := helper.IsAuthorize(*user, helper.Admin); err != nil {
		return "", err
	}

	err := service.RemoveMarkerType(input)
	if err != nil {
		return "", err
	}

	return "ok", nil
}

func (r *mutationResolver) CreatePin(ctx context.Context, input model.NewPin) (*model.Pin, error) {
	// ADMIN
	// Create pin

	user := middleware.ForContext(ctx)
	if user == nil {
		return nil, &helper.PermissionDeniedError{}
	}

	if err := helper.IsAuthorize(*user, helper.Admin); err != nil {
		return nil, err
	}

	pin, err := service.CreatePin(input, *user)
	if err != nil {
		return nil, err
	}

	output := helper.ConvertPin(*pin)

	return &output, nil
}

func (r *mutationResolver) EditPin(ctx context.Context, input model.UpdatedPin) (*model.Pin, error) {
	// ADMIN
	// Edit pin

	user := middleware.ForContext(ctx)
	if user == nil {
		return nil, &helper.PermissionDeniedError{}
	}

	if err := helper.IsAuthorize(*user, helper.Admin); err != nil {
		return nil, err
	}

	pin, err := service.EditPin(input, *user)
	if err != nil {
		return nil, err
	}

	output := helper.ConvertPin(*pin)

	return &output, err
}

func (r *mutationResolver) PreviewPin(ctx context.Context, input model.PreviewPinInput) (string, error) {
	// ADMIN
	// Preview Pin

	user := middleware.ForContext(ctx)
	if user == nil {
		return "", &helper.PermissionDeniedError{}
	}

	if err := helper.IsAuthorize(*user, helper.Admin); err != nil {
		return "", err
	}

	markertype, err := service.GetMarkerTypeById(input.TypeID)
	if err != nil {
		return "", err
	}

	outputpath, err := service.PreviewPin(input, *markertype)
	if err != nil {
		return "", err
	}

	return outputpath, nil
}

func (r *mutationResolver) RemovePin(ctx context.Context, input model.RemoveModel) (string, error) {
	// ADMIN
	// Remove pin

	user := middleware.ForContext(ctx)
	if user == nil {
		return "", &helper.PermissionDeniedError{}
	}

	if err := helper.IsAuthorize(*user, helper.Admin); err != nil {
		return "", err
	}

	err := service.RemovePin(input)
	if err != nil {
		return "", err
	}

	return "ok", nil
}

func (r *mutationResolver) UpdateDefault(ctx context.Context, input model.UpdatedDefault) (string, error) {
	// ADMIN
	// Update default

	user := middleware.ForContext(ctx)
	if user == nil {
		return "", &helper.PermissionDeniedError{}
	}

	if err := helper.IsAuthorize(*user, helper.Admin); err != nil {
		return "", err
	}

	if input.UpdatedType == constant.PinType {
		if input.IntValue == nil {
			return "", &helper.MissingIntUpdateDefault{}
		}

		_, err := service.EditDefaultPin(input, *user)
		if err != nil {
			return "", err
		}
	}

	return "ok", nil
}

func (r *mutationResolver) CreateSchedule(ctx context.Context, input model.NewSchedule) (*model.Schedule, error) {
	// USER
	// Create schedule by user

	user := middleware.ForContext(ctx)
	if user == nil {
		return nil, &helper.PermissionDeniedError{}
	}

	if err := helper.IsAuthorize(*user, helper.User); err != nil {
		return nil, err
	}

	relation, err := service.GetCurrentRelation(*user)
	if relation == nil {
		if err == nil {
			return nil, &helper.RelationNotFoundError{}
		}
		return nil, helper.CheckDatabaseError(err, &helper.RelationNotFoundError{})
	}

	var marker dbmodel.Marker
	marker.ID = uint(input.MarkerID)
	if err := marker.GetById(database.Connection); err != nil {
		return nil, helper.CheckDatabaseError(err, &helper.MarkerNotFound{})
	}

	transaction := database.Connection.Begin()

	schedule, err := service.CreateSchedule(transaction, input, marker, *user, *relation)
	if err != nil {
		transaction.Rollback()
		return nil, err
	}

	transaction.Commit()

	output := helper.ConvertSchedule(*schedule)
	return &output, nil
}

func (r *mutationResolver) UpdateScheduleStatus(ctx context.Context, input model.ScheduleStatusList) ([]*model.Schedule, error) {
	// USER
	// update schedule status, will also update marker status

	var result []*model.Schedule
	user := middleware.ForContext(ctx)
	if user == nil {
		return result, &helper.PermissionDeniedError{}
	}

	if err := helper.IsAuthorize(*user, helper.User); err != nil {
		return result, err
	}

	relation, err := service.GetCurrentRelation(*user)
	if relation == nil {
		if err == nil {
			return nil, &helper.RelationNotFoundError{}
		}
		return nil, helper.CheckDatabaseError(err, &helper.RelationNotFoundError{})
	}

	// might add validation here

	if err := helper.ValidateScheduleStatus(input); err != nil {
		return nil, err
	}

	transaction := database.Connection.Begin()

	affected_schedules, err := service.UpdateScheduleStatus(transaction, input, *relation, *user)
	if err != nil {
		transaction.Rollback()
		return result, err
	}

	transaction.Commit()

	for _, schedule := range affected_schedules {
		item := helper.ConvertSchedule(schedule)
		result = append(result, &item)
	}

	return result, nil
}

func (r *mutationResolver) RevokeMarker(ctx context.Context, input model.UpdateModel) (*model.Marker, error) {
	// USER
	// get previous markers

	user := middleware.ForContext(ctx)
	if user == nil {
		return nil, &helper.PermissionDeniedError{}
	}

	if err := helper.IsAuthorize(*user, helper.User); err != nil {
		return nil, err
	}

	relation, err := service.GetCurrentRelation(*user)
	if relation == nil {
		if err == nil {
			return nil, &helper.RelationNotFoundError{}
		}
		return nil, helper.CheckDatabaseError(err, &helper.RelationNotFoundError{})
	}

	marker, err := service.RevokeMarker(input, *user)
	if err != nil {
		return nil, err
	}

	output := helper.ConvertMarker(*marker)

	return &output, nil
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

func (r *mutationResolver) Logout(ctx context.Context, input model.Logout) (string, error) {
	// ANYONE WITH JWT
	// logout

	user := middleware.ForContext(ctx)
	if user == nil {
		return "", &helper.PermissionDeniedError{}
	}

	if err := service.Logout(user); err != nil {
		return "", err
	}

	return "ok", nil
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

	if err := preferencePtr.GetByUserId(database.Connection); err != nil {
		if !utils.RecordNotFound(err) {
			return nil, err
		}
	}

	defaultPins, err := service.GetAllDefaultPins()
	if err != nil {
		return nil, err
	}

	result := helper.UserPreferencePin(preferencePtr, defaultPins)
	if preferencePtr != nil {
		result.ID = int(preference.ID)
		if preference.RelationId != nil {
			var relation dbmodel.UserRelation
			relation.ID = *preference.RelationId
			if err := relation.GetWithUserById(database.Connection); err == nil {
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

	return &result, nil
}

func (r *queryResolver) Markers(ctx context.Context) ([]*model.Marker, error) {
	// USER
	// get markers

	var result []*model.Marker
	user := middleware.ForContext(ctx)
	if user == nil {
		return result, &helper.PermissionDeniedError{}
	}

	if err := helper.IsAuthorize(*user, helper.User); err != nil {
		return result, err
	}

	relation, err := service.GetCurrentRelation(*user)
	if relation == nil {
		if err == nil {
			return nil, &helper.RelationNotFoundError{}
		}
		return nil, helper.CheckDatabaseError(err, &helper.RelationNotFoundError{})
	}

	requested_field := utils.GetTopPreloads(ctx)

	markers, err := service.GetAllActiveMarker(requested_field, *relation)
	if err != nil {
		return result, err
	}

	for _, marker := range markers {
		item := helper.ConvertMarker(marker)
		result = append(result, &item)
	}

	return result, nil
}

func (r *queryResolver) Markertypes(ctx context.Context) ([]*model.MarkerType, error) {
	// ADMIN
	// get all marker types for selection

	var result []*model.MarkerType
	user := middleware.ForContext(ctx)
	if user == nil {
		return result, &helper.PermissionDeniedError{}
	}

	if err := helper.IsAuthorize(*user, helper.Admin); err != nil {
		return result, err
	}

	requested_field := utils.GetTopPreloads(ctx)

	types, err := service.GetAllMarkerType(requested_field)
	if err != nil {
		return result, err
	}

	for _, markertype := range types {
		item := helper.ConvertMarkerType(markertype)
		result = append(result, &item)
	}

	return result, nil
}

func (r *queryResolver) Eventtypes(ctx context.Context) ([]*model.EventType, error) {
	// USER
	// get all marker types for selection

	var result []*model.EventType
	user := middleware.ForContext(ctx)
	if user == nil {
		return result, &helper.PermissionDeniedError{}
	}

	if err := helper.IsAuthorize(*user, helper.User); err != nil {
		return result, err
	}

	types, err := service.GetAllEventType()
	if err != nil {
		return result, err
	}

	for _, markertype := range types {
		item := helper.ConvertMarkerTypeToEventType(markertype)
		result = append(result, &item)
	}

	return result, nil
}

func (r *queryResolver) Pins(ctx context.Context) ([]*model.Pin, error) {
	// ADMIN
	// get all pins

	var result []*model.Pin
	user := middleware.ForContext(ctx)
	if user == nil {
		return result, &helper.PermissionDeniedError{}
	}

	if err := helper.IsAuthorize(*user, helper.Admin); err != nil {
		return result, err
	}

	requested_field := utils.GetTopPreloads(ctx)

	pins, err := service.GetAllPin(requested_field)
	if err != nil {
		return result, err
	}

	for _, pin := range pins {
		item := helper.ConvertPin(pin)
		result = append(result, &item)
	}

	return result, nil
}

func (r *queryResolver) Defaultpins(ctx context.Context) ([]*model.DefaultPin, error) {
	// ADMIN
	// get all default pins value
	var result []*model.DefaultPin
	user := middleware.ForContext(ctx)
	if user == nil {
		return result, &helper.PermissionDeniedError{}
	}

	if err := helper.IsAuthorize(*user, helper.Admin); err != nil {
		return result, err
	}

	pins, err := service.GetAllDefaultPins()
	if err != nil {
		return result, err
	}

	for _, pin := range pins {
		item := helper.ConvertToDefaultPin(pin)
		result = append(result, &item)
	}

	return result, nil
}

func (r *queryResolver) Mappins(ctx context.Context) ([]*model.MapPin, error) {
	// ADMIN
	// get all default pins value
	var result []*model.MapPin
	user := middleware.ForContext(ctx)
	if user == nil {
		return result, &helper.PermissionDeniedError{}
	}

	if err := helper.IsAuthorize(*user, helper.User); err != nil {
		return result, err
	}

	// get the user preference first
	var preference dbmodel.UserPreference

	preference.UserId = user.ID

	preferencePtr := &preference

	if err := preferencePtr.GetByUserId(database.Connection); err != nil {
		if !utils.RecordNotFound(err) {
			return nil, err
		}
	}

	// get the default pins
	defaultPins, err := service.GetAllDefaultPins()
	if err != nil {
		return nil, err
	}

	userpreference := helper.UserPreferencePin(preferencePtr, defaultPins)

	typepin_list, err := service.FetchAllTypePinByUserPreference(userpreference)
	if err != nil {
		return nil, err
	}

	for _, typepin := range typepin_list {
		item := helper.ConvertToMapPin(typepin)
		result = append(result, &item)
	}

	if preferencePtr.RegularPin != nil {
		regularItem := helper.ConvertPinToMapPin(*preference.RegularPin, constant.RegularPin)
		result = append(result, &regularItem)
	}

	if preferencePtr.FavouritePin != nil {
		favouriteItem := helper.ConvertPinToMapPin(*preference.FavouritePin, constant.FavouritePin)
		result = append(result, &favouriteItem)
	}

	if preferencePtr.SelectedPin != nil {
		selectedItem := helper.ConvertPinToMapPin(*preference.SelectedPin, constant.SelectedPin)
		result = append(result, &selectedItem)
	}

	if preferencePtr.HurryPin != nil {
		hurryItem := helper.ConvertPinToMapPin(*preference.HurryPin, constant.HurryPin)
		result = append(result, &hurryItem)
	}

	return result, nil
}

func (r *queryResolver) Schedules(ctx context.Context, params model.CurrentTime) ([]*model.Schedule, error) {
	// USER
	// get schedules

	var result []*model.Schedule
	user := middleware.ForContext(ctx)
	if user == nil {
		return result, &helper.PermissionDeniedError{}
	}

	if err := helper.IsAuthorize(*user, helper.User); err != nil {
		return result, err
	}

	relation, err := service.GetCurrentRelation(*user)
	if relation == nil {
		if err == nil {
			return result, &helper.RelationNotFoundError{}
		}
		return result, helper.CheckDatabaseError(err, &helper.RelationNotFoundError{})
	}

	requested_field := utils.GetTopPreloads(ctx)

	schedules, err := service.GetAllSchedule(params, requested_field, *relation)
	if err != nil {
		return result, err
	}

	for _, schedule := range schedules {
		item := helper.ConvertSchedule(schedule)
		result = append(result, &item)
	}

	return result, nil
}

func (r *queryResolver) Today(ctx context.Context, params model.CurrentTime) (*model.TodayEvent, error) {
	// USER
	// get today event

	user := middleware.ForContext(ctx)
	if user == nil {
		return nil, &helper.PermissionDeniedError{}
	}

	if err := helper.IsAuthorize(*user, helper.User); err != nil {
		return nil, err
	}

	relation, err := service.GetCurrentRelation(*user)
	if relation == nil {
		if err == nil {
			return nil, &helper.RelationNotFoundError{}
		}
		return nil, helper.CheckDatabaseError(err, &helper.RelationNotFoundError{})
	}

	requested_field := utils.GetPreloads(ctx)

	yesterday_schedules, err := service.GetYesterdaySchedules(params, requested_field, *relation)
	if err != nil {
		return nil, err
	}

	// creating output object
	var result model.TodayEvent

	var schedules_output []*model.Schedule

	for _, schedule := range yesterday_schedules {
		item := helper.ConvertSchedule(schedule)
		schedules_output = append(schedules_output, &item)
	}

	result.YesterdayEvent = schedules_output

	return &result, nil
}

func (r *queryResolver) Previousmarkers(ctx context.Context) ([]*model.Marker, error) {
	// USER
	// get previous markers

	var result []*model.Marker
	user := middleware.ForContext(ctx)
	if user == nil {
		return nil, &helper.PermissionDeniedError{}
	}

	if err := helper.IsAuthorize(*user, helper.User); err != nil {
		return nil, err
	}

	relation, err := service.GetCurrentRelation(*user)
	if relation == nil {
		if err == nil {
			return nil, &helper.RelationNotFoundError{}
		}
		return nil, helper.CheckDatabaseError(err, &helper.RelationNotFoundError{})
	}

	requested_field := utils.GetTopPreloads(ctx)

	markers, err := service.GetAllPreviousMarker(requested_field, *relation)
	if err != nil {
		return nil, err
	}

	for _, marker := range markers {
		item := helper.ConvertMarker(marker)
		result = append(result, &item)
	}

	return result, nil
}

func (r *queryResolver) Markerschedules(ctx context.Context, params model.IDModel) ([]*model.Schedule, error) {
	// USER
	// get all schedules assiocated with this marker

	var result []*model.Schedule
	user := middleware.ForContext(ctx)
	if user == nil {
		return nil, &helper.PermissionDeniedError{}
	}

	if err := helper.IsAuthorize(*user, helper.User); err != nil {
		return nil, err
	}

	relation, err := service.GetCurrentRelation(*user)
	if relation == nil {
		if err == nil {
			return nil, &helper.RelationNotFoundError{}
		}
		return nil, helper.CheckDatabaseError(err, &helper.RelationNotFoundError{})
	}

	requested_field := utils.GetTopPreloads(ctx)

	schedules, err := service.GetSchedulesByMarker(uint(params.ID), requested_field, *relation)
	if err != nil {
		return nil, err
	}

	for _, schedule := range schedules {
		item := helper.ConvertSchedule(schedule)
		result = append(result, &item)
	}

	return result, nil
}

func (r *queryResolver) Me(ctx context.Context) (string, error) {
	user := middleware.ForContext(ctx)
	if user == nil {
		return "", &helper.PermissionDeniedError{}
	}

	return "ok", nil
}

// Mutation returns generated.MutationResolver implementation.
func (r *Resolver) Mutation() generated.MutationResolver { return &mutationResolver{r} }

// Query returns generated.QueryResolver implementation.
func (r *Resolver) Query() generated.QueryResolver { return &queryResolver{r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
