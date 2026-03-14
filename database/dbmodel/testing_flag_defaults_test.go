package dbmodel

import (
	"reflect"
	"strings"
	"testing"
)

func TestTestingFlagGormDefaults(t *testing.T) {
	type fieldCheck struct {
		model interface{}
		name  string
	}

	checks := []fieldCheck{
		{model: APIKey{}, name: "testing"},
		{model: Marker{}, name: "testing"},
		{model: Schedule{}, name: "testing"},
	}

	for _, check := range checks {
		typ := reflect.TypeOf(check.model)
		field, ok := typ.FieldByNameFunc(func(n string) bool {
			return strings.EqualFold(n, check.name)
		})
		if !ok {
			t.Fatalf("%s missing field %s", typ.Name(), check.name)
		}
		tag := field.Tag.Get("gorm")
		if !strings.Contains(tag, "default:false") {
			t.Fatalf("%s.%s missing default:false gorm tag (tag=%q)", typ.Name(), field.Name, tag)
		}
		if !strings.Contains(tag, "not null") {
			t.Fatalf("%s.%s missing not null gorm tag (tag=%q)", typ.Name(), field.Name, tag)
		}
	}
}
