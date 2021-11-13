FROM golang:1.15 AS build-stage

WORKDIR /app
COPY . .
RUN go get
ENV CGO_ENABLED=0
RUN go build -o /app/RMMarker

FROM alpine:3.9 AS production-stage
WORKDIR /app
COPY --from=build-stage /app/RMMarker /app/RMMarker

EXPOSE 3000

CMD ["/app/RMMarker"]