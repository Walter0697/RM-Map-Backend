package helper

import (
	"mapmarker/backend/database/dbmodel"
)

type UserRole string

const (
	Admin UserRole = "admin"
	User  UserRole = "user"
)

func UserRoles() []UserRole {
	return []UserRole{Admin, User}
}

func IsValidRoles(input string) bool {
	for _, role := range UserRoles() {
		if role == UserRole(input) {
			return true
		}
	}

	return false
}

func IsAuthorize(user dbmodel.User, required_role UserRole) error {
	if required_role == UserRole(user.Role) {
		return nil
	}

	for _, role := range UserRoles() {
		if role == UserRole(user.Role) {
			return nil
		}
		if role == required_role {
			return &PermissionDeniedError{}
		}
	}

	return nil
}
