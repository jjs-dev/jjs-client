FROM golang:latest AS builder

WORKDIR $GOPATH/src/jjs-client/
COPY . .

RUN go get -d -v && go build -tags 'osusergo netgo' -ldflags '-extldflags "-static"' -o /go/bin/jjs-client && cp -r ./static /go/bin/ && cp -r ./templates /go/bin/
FROM scratch

COPY --from=builder /go/bin/ /jjs-client/
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs

WORKDIR /jjs-client/
ENTRYPOINT ["./jjs-client"]

EXPOSE 80
EXPOSE 443
