.PHONY: test test-race generate migrate-station-lines

test:
	GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod-cache go test ./...

test-race:
	GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod-cache go test -race ./...

generate:
	GOBIN=/tmp/gqlgen-bin go install github.com/99designs/gqlgen@v0.17.81
	/tmp/gqlgen-bin/gqlgen generate --verbose --config gqlgen.yml

migrate-station-lines:
	./scripts/apply_train_station_line_migration.sh
