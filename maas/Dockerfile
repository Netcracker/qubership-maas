FROM golang:1.27.2@sha256:2d658507a011f1eadc0114d7a421b312b2e77d9042a24036a26e267cc9231677 AS build

WORKDIR /app

COPY maas-service .

RUN go mod download
RUN go build -o maas-service .

# do not rewrite this as "COPY --chmod=..." it uses BuildKit and not supported by Testcontainers
RUN chmod 555 maas-service && \
    chmod 444 application.yaml docs/swagger.json docs/swagger.yaml

FROM ghcr.io/netcracker/qubership-core-base:alpine-2.5.7@sha256:f0836415633bc2a63c2a0431c4c0701d17bd7b31132176b26651b49927d9ad7e AS run

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

