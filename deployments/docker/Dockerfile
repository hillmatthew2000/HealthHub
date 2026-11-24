# syntax=docker/dockerfile:1

# Build stage: compile a static binary for the target platform.
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .
ARG VERSION=dev
ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/healthhub ./cmd/server

# Runtime stage: no shell or package manager, CA certificates and time zone
# data only, running as an unprivileged user.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/healthhub /healthhub
USER nonroot:nonroot
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD ["/healthhub", "healthcheck"]

ENTRYPOINT ["/healthhub"]
CMD ["serve"]
