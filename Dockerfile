# This file is used by goreleaser

ARG BUILDPLATFORM
FROM --platform=$BUILDPLATFORM alpine:3.22
# TARGETPLATFORM needs to be set after FROM
ARG TARGETPLATFORM

ENTRYPOINT ["/iracelog-service-manager-go"]
HEALTHCHECK --interval=2s --timeout=2s --start-period=5s --retries=3 CMD [ "/grpc_health_probe", "-addr", "localhost:8080" ]
RUN apk add --no-cache ca-certificates
COPY $TARGETPLATFORM/iracelog-service-manager-go /
COPY pkg/db/migrate/migrations /migrations
COPY samples /
COPY ext/healthcheck/$TARGETPLATFORM/grpc_health_probe /grpc_health_probe
