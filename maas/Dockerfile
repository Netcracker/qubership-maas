FROM golang:1.26.4@sha256:f96cc555eb8db430159a3aa6797cd5bae561945b7b0fe7d0e284c63a3b291609 AS build

WORKDIR /app

COPY maas-service .

RUN go mod download
RUN go build -o maas-service .

# do not rewrite this as "COPY --chmod=..." it uses BuildKit and not supported by Testcontainers
RUN chmod 555 maas-service && \
    chmod 444 application.yaml docs/swagger.json docs/swagger.yaml

FROM ghcr.io/netcracker/qubership-core-base:2.3.3@sha256:1339716127a7d170ba307b89f3a933f5e09c447607c89e16bf8d5a379db4e1f6 AS run

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

