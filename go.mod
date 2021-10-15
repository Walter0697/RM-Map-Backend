module mapmarker/backend

go 1.15

require (
	github.com/99designs/gqlgen v0.14.0
	github.com/go-chi/chi v1.5.4
	github.com/go-chi/cors v1.2.0
	github.com/golang-jwt/jwt v3.2.2+incompatible
	github.com/pkg/errors v0.9.1 // indirect
	github.com/shaj13/go-guardian/v2 v2.11.3
	github.com/shaj13/libcache v1.0.0
	github.com/spf13/viper v1.9.0
	github.com/vektah/gqlparser/v2 v2.2.0
	golang.org/x/crypto v0.0.0-20210921155107-089bfa567519
	gorm.io/driver/postgres v1.1.2
	gorm.io/gorm v1.21.15
)

replace github.com/spf13/afero => github.com/spf13/afero v1.5.1
