FROM ghcr.io/netcracker/qubership-core-base:2.4.2@sha256:78e39e1332035b539eb61f605a9a3b638ccad27b3c64ed0783f76b6b6d4de8c5
LABEL maintainer="qubership"

ADD ./install /tmp/

USER root
RUN chmod +x /tmp/install.sh && /tmp/install.sh
USER 1000

ENTRYPOINT ["/validation.sh"]
