# This file is used by goreleaser
FROM scratch
ENTRYPOINT ["/iracelog-service-manager-go"]
COPY iracelog-service-manager-go /
COPY pkg/db/migrate/migrations /migrations
COPY samples /
