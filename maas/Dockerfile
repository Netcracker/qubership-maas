FROM golang:1.27.1@sha256:7bffdb405cd12940d2980daa49a86ef575ed4525a17ee7d0c9562547357ab46a AS build

WORKDIR /app

COPY maas-service .

RUN go mod download
RUN go build -o maas-service .

# do not rewrite this as "COPY --chmod=..." it uses BuildKit and not supported by Testcontainers
RUN chmod 555 maas-service && \
    chmod 444 application.yaml docs/swagger.json docs/swagger.yaml

FROM ghcr.io/netcracker/qubership-core-base:alpine-2.5.6@sha256:e85e971e33cdb218bf1c23ccbad0d0ddaee7d3abfc442247d81081e3d0467467 AS run

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
