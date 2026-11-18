FROM golang:1.26-alpine AS build-stage

WORKDIR /app
COPY . .
RUN for attempt in 1 2 3 4 5; do \
      if go mod download; then \
        exit 0; \
      fi; \
      echo "go mod download failed (attempt ${attempt}/5); retrying" >&2; \
      sleep 5; \
    done; \
    exit 1
ENV CGO_ENABLED=0
RUN go build -o /app/RMMarker

FROM alpine:3.20 AS production-stage
WORKDIR /app
COPY --from=build-stage /app/RMMarker /app/RMMarker
COPY --from=build-stage /app/json /app/json
COPY --from=build-stage /app/assets /app/assets

EXPOSE 3000

CMD ["/app/RMMarker"]
