ARG GOLANG_VERSION=1

FROM golang:${GOLANG_VERSION} AS build

COPY . /app
WORKDIR /app

ARG CGO_ENABLED=0
ARG GOOS=linux
ARG GOARCH=amd64

RUN go build main.go \
  && go test -v ./... \
  && go vet ./...

FROM scratch

COPY --from=build /app/main /main
COPY --from=build /etc/ssl/certs /etc/ssl/certs

ENTRYPOINT ["/main"]
