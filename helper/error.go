package helper

import (
	"mapmarker/backend/utils"
)

func GetDatabaseError(err error) error {
	if utils.RecordNotFound(err) {
		return &RecordNotFoundError{}
	}
	return err
}

func CheckDatabaseError(err error, notfounderr error) error {
	if utils.RecordNotFound(err) {
		return notfounderr
	}
	return err
}

type RecordNotFoundError struct{}

func (m *RecordNotFoundError) Error() string {
	return "cannot find record in database"
}

type PermissionDeniedError struct{}

func (m *PermissionDeniedError) Error() string {
	return "permission denied"
}

type SameUserNameExistError struct{}

func (m *SameUserNameExistError) Error() string {
	return "same username exist"
}

type RoleInvalidError struct{}

func (m *RoleInvalidError) Error() string {
	return "role is invalid"
}

type IncorrectPasswordError struct{}

func (m *IncorrectPasswordError) Error() string {
	return "incorrect password"
}

type LDAPLoginEnabledError struct{}

func (m *LDAPLoginEnabledError) Error() string {
	return "ldap login enabled, cannot create user"
}

type UploadFileNotImageError struct{}

func (n *UploadFileNotImageError) Error() string {
	return "upload file is not an image"
}

type RelationWithYourselfError struct{}

func (n *RelationWithYourselfError) Error() string {
	return "cannot setup relation with yourself"
}

type RelationNotFoundError struct{}

func (n *RelationNotFoundError) Error() string {
	return "cannot perform this without relation"
}

type ImageNotFoundError struct{}

func (n *ImageNotFoundError) Error() string {
	return "image is required for this request"
}

type FavouriteMarkerNotDeletableError struct{}

func (n *FavouriteMarkerNotDeletableError) Error() string {
	return "favourite marker cannot be deleted"
}

type MissingIntUpdateDefault struct{}

func (n *MissingIntUpdateDefault) Error() string {
	return "cannot update this field without int value"
}

type MarkerNotFound struct{}

func (n *MarkerNotFound) Error() string {
	return "cannot find marker in the database"
}

type InvalidRelationUpdateError struct{}

func (n *InvalidRelationUpdateError) Error() string {
	return "the item you are trying to update has the wrong relation"
}

type InvalidStatusError struct{}

func (n *InvalidStatusError) Error() string {
	return "status is not valid"
}

type QueryCannotEmptyError struct{}

func (n *QueryCannotEmptyError) Error() string {
	return "query cannot be empty"
}

type RestaurantNotFound struct{}

func (n *RestaurantNotFound) Error() string {
	return "restaurant not found"
}
