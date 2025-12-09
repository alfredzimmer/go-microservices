FROM golang:alpine AS build
RUN apk --no-cache add gcc g++ make ca-certificates
WORKDIR /go/src/github.com/alfredzimmer/go-microservices
COPY go.mod go.sum ./
RUN go mod download
COPY account account
COPY catalog catalog
COPY order order
COPY graphql graphql
RUN GO111MODULE=on go build -o /go/bin/app ./graphql

FROM alpine:latest
WORKDIR /usr/bin
COPY --from=build /go/bin .
EXPOSE 8080
CMD ["app"]