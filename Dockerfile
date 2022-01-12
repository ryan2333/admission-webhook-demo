FROM alpine:3.15.0

RUN sed -i 's@http://dl-cdn.alpinelinux.org/@https://mirrors.aliyun.com/@g' /etc/apk/repositories
RUN apk add --no-cache --virtual .persistent-deps \
    curl \
    tcpdump \
    iproute2 \
    bind-tools \
    ethtool \
    busybox-extras \
    libressl \
    openssh-client \
    busybox \
    net-tools
COPY ./webhook-server /
ENTRYPOINT [ "/webhook-server" ]