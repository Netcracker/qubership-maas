FROM ghcr.io/netcracker/qubership-core-base:2.2.12@sha256:a53c3219e1b6ffea834430f544d79ea350a2cd244a02077ef9cce5d035b2ea5a
LABEL maintainer="qubership"

ADD ./install /tmp/

USER root
RUN chmod +x /tmp/install.sh && /tmp/install.sh
USER 1000

ENTRYPOINT ["/validation.sh"]

