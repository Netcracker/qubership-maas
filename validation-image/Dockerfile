FROM ghcr.io/netcracker/qubership-core-base:alpine-2.5.13@sha256:891386ea91d90a1376e12f30b458fc23eb9793afd35f82817c90e4911cddd633
LABEL maintainer="qubership"

ADD ./install /tmp/

USER root
RUN chmod +x /tmp/install.sh && /tmp/install.sh
USER 1000

ENTRYPOINT ["/validation.sh"]
