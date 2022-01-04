# RM-Map-Backend

### Introduction
- RoMarker Map, an application to store location that we want to visit. Just a personal project for myself to have practical usage

### Technologies
- GoFiber
- Postgres
- LDAP (togglable)
- GraphQL
- Simple Web scrapping using GoQuery

### Environment
- most of them are pretty easy to follow according to `config.example.toml`, `enable` under `[ldap]` section indicate that if you want to use LDAP to login, any new login in this system with LDAP will use `defaultrole` as their role

### Notes to self
run `go run github.com/99designs/gqlgen generate` if schema changed