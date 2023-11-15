FROM openjdk:21-slim AS api

COPY . /app
WORKDIR /app/api-generator

RUN apk add curl
RUN /bin/sh generate.sh

FROM docker-proxy.interhyp-intern.de/golangci/golangci-lint:v1.55-alpine as lint

COPY --from=api /app /app
WORKDIR /app

RUN golangci-lint run


FROM golang:1 as build

COPY --from=api /app /app
WORKDIR /app


RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" main.go
RUN go test -v ./... -coverpkg=./internal/... 2>&1 | go-junit-report -set-exit-code -iocopy -out report.xml || (touch build.failed && echo "There were failing unit tests!")
RUN go vet ./...


FROM scratch

COPY --from=build /app/main /main
COPY --from=build /app/api/openapi-v3-spec.json /api/openapi-v3-spec.json

ENTRYPOINT ["/main"]
