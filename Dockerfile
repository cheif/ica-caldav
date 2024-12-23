FROM golang:alpine as builder
RUN apk update && apk add --no-cache git
WORKDIR $GOPATH/src/ica-caldav
COPY go.mod go.sum .
RUN go mod download
COPY . .
RUN go get -v
RUN --mount=type=cache,target=/root/.cache/go-build \
            go build -o /go/bin/ica-caldav

FROM alpine:latest
COPY --from=builder /go/bin/ica-caldav /go/bin/ica-caldav

VOLUME /cache
CMD ["/bin/sh", "-c", "/go/bin/ica-caldav --cachePath /cache"]
