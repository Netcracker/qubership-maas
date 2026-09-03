FROM golang:1.27.1@sha256:b475798fb16158e6c38e8b5ca2d870fbeaa8b7fec0fc8ec64b3dc20966040635 AS build

WORKDIR /app

COPY maas-service .

RUN go mod download
RUN go build -o maas-service .

# do not rewrite this as "COPY --chmod=..." it uses BuildKit and not supported by Testcontainers
RUN chmod 555 maas-service && \
    chmod 444 application.yaml docs/swagger.json docs/swagger.yaml

FROM ghcr.io/netcracker/qubership-core-base:2.4.2@sha256:78e39e1332035b539eb61f605a9a3b638ccad27b3c64ed0783f76b6b6d4de8c5 AS run

COPY --chown=10001:0 --from=build app/maas-service /app/maas
COPY --chown=10001:0 --from=build app/application.yaml /app/
COPY --chown=10001:0 --from=build app/docs/swagger.json /app/
COPY --chown=10001:0 --from=build app/docs/swagger.yaml /app/

USER root

RUN echo "https://dl-cdn.alpinelinux.org/alpine/edge/main" > /etc/apk/repositories  \
    && echo "https://dl-cdn.alpinelinux.org/alpine/edge/community" >> /etc/apk/repositories \
    && apk add --update --upgrade --no-cache \
        pcre2 curl gnutls libcurl jq openssl libcrypto3 libssl3 zlib musl musl-utils

WORKDIR /app

USER 10001:10001

CMD ["/app/maas"]
