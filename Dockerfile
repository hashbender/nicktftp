FROM golang

COPY . /go/src/github.com/nitronick600/nicktftp

RUN go install github.com/nitronick600/nicktftp

WORKDIR /go/src/github.com/nitronick600/nicktftp

ENTRYPOINT /go/bin/nicktftp

EXPOSE 6969
EXPOSE 6969/udp