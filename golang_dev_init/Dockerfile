From golang:1.17

MAINTAINER saber

RUN ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime
RUN echo 'Asia/Shanghai' >/etc/timezone
COPY ./golangci-lint /go/bin
COPY ./linux/protoc /go/bin
COPY ./linux/protoc-gen-go /go/bin

