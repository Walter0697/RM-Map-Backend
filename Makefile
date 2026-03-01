.PHONY: test test-race generate

test:
	GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod-cache go test ./...

test-race:
	GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod-cache go test -race ./...

generate:
	/tmp/gqlgen4 generate --verbose --config gqlgen.yml
