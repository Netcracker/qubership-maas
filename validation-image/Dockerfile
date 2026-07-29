FROM ghcr.io/netcracker/qubership-core-base:2.3.6@sha256:b77f160cfd406124b3f1be223e82b21e742d5533638c64409e5992342587304a
LABEL maintainer="qubership"

ADD ./install /tmp/

USER root
RUN chmod +x /tmp/install.sh && /tmp/install.sh
USER 1000

ENTRYPOINT ["/validation.sh"]
