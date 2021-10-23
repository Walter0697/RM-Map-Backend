package utils

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

func ConvertFieldsToDBColumns(fields []string) string {
	return strings.Join(fields, ", ")
}

func StringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func ConvertToOutputTime(t time.Time) string {
	return t.Format("2006-01-02T15:04:05Z")
}

func RecordNotFound(err error) bool {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return true
	}
	return false
}
