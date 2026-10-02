FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build

ARG TARGETOS
ARG TARGETARCH
ARG BUILD_VERSION=""

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/web ./cmd/web \
    && CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/seed ./cmd/seed

FROM alpine:3.22 AS runtime

ARG BUILD_VERSION=""

RUN apk add --no-cache ca-certificates \
    && addgroup -S app \
    && adduser -S -G app -h /home/app app \
    && mkdir -p /home/data/uploads \
    && chown -R app:app /home/data

WORKDIR /app
COPY --from=build --chown=app:app /out/web /app/web
COPY --from=build --chown=app:app /out/seed /app/seed
COPY --from=build --chown=app:app /src/static /app/static

USER app:app
ENV APP_ENV=production \
    PORT=8080 \
    SITE_DB=/home/data/site.db \
    BUILD_VERSION=${BUILD_VERSION}
EXPOSE 8080
ENTRYPOINT ["/app/web"]
