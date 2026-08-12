FROM golang:1.26.5@sha256:55d3b3d8ea3ae125d21f528f392a7ae0efbd9c69cbac7a479921121af3c7b2a2 AS build

WORKDIR /app

COPY maas-service .

RUN go mod download
RUN go build -o maas-service .

# do not rewrite this as "COPY --chmod=..." it uses BuildKit and not supported by Testcontainers
RUN chmod 555 maas-service && \
    chmod 444 application.yaml docs/swagger.json docs/swagger.yaml

FROM ghcr.io/netcracker/qubership-core-base:2.3.7@sha256:b917b3a1731a2ae26b507d22565f030ec25ff8d28b75a80b8b08bbc946f4d73b AS run

COPY --chown=10001:0 --from=build app/maas-service /app/maas
COPY --chown=10001:0 --from=build app/application.yaml /app/
COPY --chown=10001:0 --from=build app/docs/swagger.json /app/
COPY --chown=10001:0 --from=build app/docs/swagger.yaml /app/

USER root

RUN echo "https://dl-cdn.alpinelinux.org/alpine/edge/main" > /etc/apk/repositories  \
    && echo "https://dl-cdn.alpinelinux.org/alpine/edge/community" >> /etc/apk/repositories \
    && apk add --update --upgrade --no-cache \
        pcre2 curl gnutls libcurl jq openssl libcrypto3 binutils libssl3 zlib musl musl-utils

WORKDIR /app

USER 10001:10001

CMD ["/app/maas"]
