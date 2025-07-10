FROM golang:1.23-bookworm AS build

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . ./

ARG VERSION=unknown
ARG SHA=unknown

RUN CGO_ENABLED=0 go build -ldflags="-X 'main.Version=${VERSION}' -X 'main.SHA=${SHA}'" -o /bin/app cmd/server/main.go

## Deploy
FROM scratch

WORKDIR /

COPY --from=build /bin/app /bin/app
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

EXPOSE 3000

ENTRYPOINT ["/bin/app"]
