ARG GOLANG_VER=1.11
ARG ALPINE_VER=3.8
##### build stage ###########################################################
FROM golang:${GOLANG_VER}-alpine${ALPINE_VER} as golang

ADD . /go/src/git.sami.int.thomsonreuters.com/rcom-api/rcomproxy/
RUN cd /go/src/git.sami.int.thomsonreuters.com/rcom-api/rcomproxy; go build -o /go/bin/rcomproxy


##### run stage #############################################################
FROM alpine:${ALPINE_VER}
LABEL author=yong.zu
LABEL email=yong.zu@thomsonreuters.com

ENV EXPOSE_PORT=3128

EXPOSE ${EXPOSE_PORT}/tcp
COPY --from=golang /go/bin/rcomproxy        /usr/local/bin/rcomproxy
RUN apk add ca-certificates

ENTRYPOINT ["/usr/local/bin/rcomproxy"]
