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
