FROM node:26-slim AS web

WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist

ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/tuck ./cmd/tuck

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/tuck /tuck

EXPOSE 8080
USER nonroot

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s CMD ["/tuck", "healthcheck"]
ENTRYPOINT ["/tuck"]
