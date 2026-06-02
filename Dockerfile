FROM golang:1.26-alpine AS build-stage

WORKDIR /app
COPY . .
RUN go mod download
ENV CGO_ENABLED=0
RUN go build -o /app/RMMarker

FROM alpine:3.20 AS production-stage
WORKDIR /app
COPY --from=build-stage /app/RMMarker /app/RMMarker

EXPOSE 3000

CMD ["/app/RMMarker"]
