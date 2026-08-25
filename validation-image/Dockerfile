FROM ghcr.io/netcracker/qubership-core-base:2.4.0@sha256:803d5b5411835a2f46d577d6fb6810e4a11f9a954a12993e705f2a912db2621d
LABEL maintainer="qubership"

ADD ./install /tmp/

USER root
RUN chmod +x /tmp/install.sh && /tmp/install.sh
USER 1000

ENTRYPOINT ["/validation.sh"]
