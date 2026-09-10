FROM ghcr.io/netcracker/qubership-core-base:2.4.3@sha256:346237ade5d4a91a2ba3099aa9377e744b40241b105bc580fb48ad81f56137a1
LABEL maintainer="qubership"

ADD ./install /tmp/

USER root
RUN chmod +x /tmp/install.sh && /tmp/install.sh
USER 1000

ENTRYPOINT ["/validation.sh"]
