FROM ghcr.io/netcracker/qubership-core-base:2.5.3@sha256:51063717c4536e8e6ee7afc2d1e0bee6f4c12237e818d6112cf55e149d6cbeca
LABEL maintainer="qubership"

ADD ./install /tmp/

USER root
RUN chmod +x /tmp/install.sh && /tmp/install.sh
USER 1000

ENTRYPOINT ["/validation.sh"]
