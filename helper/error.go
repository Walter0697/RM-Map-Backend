package helper

import (
	"errors"

	"gorm.io/gorm"
)

func GetDatabaseError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
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
