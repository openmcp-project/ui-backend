FROM golang:1.27-bookworm@sha256:5cf287a799e6b94384bad13d16b14904c531f51ba65792237e122ce42b392f61 AS build

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . ./

RUN CGO_ENABLED=0 go build -o /bin/app cmd/server/main.go

## Deploy
FROM scratch

WORKDIR /

COPY --from=build /bin/app /bin/app
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

USER 65532:65532

EXPOSE 3000

ENTRYPOINT ["/bin/app"]
