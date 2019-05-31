FROM golang:1.12 AS builder

# Helps using module proxy for caching deps for faster build
ARG GO111MODULE
ARG GOPROXY

WORKDIR $GOPATH/src/github.com/GoogleCloudPlatform/govanityurls
COPY . .
RUN go get -v .

FROM gcr.io/distroless/base
COPY --from=builder /go/bin/govanityurls /go/bin/
WORKDIR /app/
ENTRYPOINT [ "/go/bin/govanityurls" ]
