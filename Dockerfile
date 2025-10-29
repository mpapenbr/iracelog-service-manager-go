# This file is used by goreleaser
FROM alpine:3.22
ARG ARCH
ENTRYPOINT ["/iracelog-service-manager-go"]
HEALTHCHECK --interval=2s --timeout=2s --start-period=5s --retries=3 CMD [ "/grpc_health_probe", "-addr", "localhost:8080" ]
RUN apk add --no-cache ca-certificates
COPY iracelog-service-manager-go /
COPY pkg/db/migrate/migrations /migrations
COPY samples /
COPY ext/healthcheck/grpc_health_probe.$ARCH /grpc_health_probe
