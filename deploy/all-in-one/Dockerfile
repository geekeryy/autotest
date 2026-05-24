# syntax=docker/dockerfile:1

ARG GO_VERSION=1.25
ARG NODE_VERSION=22

FROM node:${NODE_VERSION}-alpine AS frontend
WORKDIR /src/web/admin
COPY web/admin/package.json web/admin/package-lock.json* ./
RUN npm ci --ignore-scripts 2>/dev/null || npm install --ignore-scripts
COPY web/admin/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS backend
ARG TARGETOS
ARG TARGETARCH
RUN apk add --no-cache ca-certificates git
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /src/web/admin/dist ./cmd/api/webdist
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -tags embedadmin -o /out/api ./cmd/api

FROM alpine:3.21 AS runtime
RUN apk add --no-cache ca-certificates postgresql-client tzdata
WORKDIR /app
COPY --from=backend /out/api /app/api
COPY migrations /app/migrations
COPY scripts/docker-entrypoint.sh /app/docker-entrypoint.sh
RUN chmod +x /app/docker-entrypoint.sh
EXPOSE 8080
ENTRYPOINT ["/app/docker-entrypoint.sh"]
CMD ["/app/api"]
