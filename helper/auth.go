package helper

import (
	"context"
	"mapmarker/backend/middleware"
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

func IsAuthorize(ctx context.Context, required_role UserRole) error {
	user := middleware.ForContext(ctx)
	if user == nil {
		return &PermissionDeniedError{}
	}

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
