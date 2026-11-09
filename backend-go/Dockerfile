FROM golang:1.26-alpine@sha256:a2012b4a1d1f9f7e9f44e0d39b805a901e856b1dd80b4e6ee4338db547050308 AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /api ./cmd/api && CGO_ENABLED=0 go build -trimpath -o /migrate ./cmd/migrate
FROM alpine:3.21@sha256:ee16d92f96db08d9d2d07da48555a27f3a69d8f0759021dbc601195ed0d2b5bd
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 app
COPY --from=build /api /api
COPY --from=build /migrate /migrate
USER app
EXPOSE 8080
ENTRYPOINT ["/api"]
