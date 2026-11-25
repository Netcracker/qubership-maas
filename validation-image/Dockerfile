FROM ghcr.io/netcracker/qubership-core-base:alpine-2.5.15@sha256:a3af3751d9187ce7f8ffd67a5aa8465fee78056c79f964bfa7727ea00da681da
LABEL maintainer="qubership"

ADD ./install /tmp/

USER root
RUN echo "https://dl-cdn.alpinelinux.org/alpine/edge/main" > /etc/apk/repositories \
    && echo "https://dl-cdn.alpinelinux.org/alpine/edge/community" >> /etc/apk/repositories \
    && chmod +x /tmp/install.sh && /tmp/install.sh
USER 1000

ENTRYPOINT ["/validation.sh"]
