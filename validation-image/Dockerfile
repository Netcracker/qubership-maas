FROM ghcr.io/netcracker/qubership-core-base:2.3.3@sha256:1339716127a7d170ba307b89f3a933f5e09c447607c89e16bf8d5a379db4e1f6
LABEL maintainer="qubership"

ADD ./install /tmp/

USER root
RUN chmod +x /tmp/install.sh && /tmp/install.sh
USER 1000

ENTRYPOINT ["/validation.sh"]

