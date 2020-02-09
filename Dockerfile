FROM golang:latest AS builder

#RUN apk update && apk add --no-cache git

WORKDIR $GOPATH/src/jjs-client/
COPY . .

RUN go get -d -v && go build -tags 'osusergo netgo' -ldflags '-extldflags "-static"' -o /go/bin/jjs-client && cp -r ./static /go/bin/ && cp -r ./templates /go/bin/
FROM scratch

COPY --from=builder /go/bin/ /jjs-client/

WORKDIR /jjs-client/
ENTRYPOINT ["./jjs-client"]

EXPOSE 80
