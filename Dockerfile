ARG GOLANG_VERSION=1

FROM golang:${GOLANG_VERSION} AS build

WORKDIR /app

COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" main.go
RUN go test -v ./...
RUN go vet ./...

FROM scratch

COPY --from=build /app/main /main
COPY --from=build /etc/ssl/certs /etc/ssl/certs

ENTRYPOINT ["/main"]
