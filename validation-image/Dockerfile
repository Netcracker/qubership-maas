FROM ghcr.io/netcracker/qubership-core-base:2.5.4@sha256:9e9c011ea0671546ae74d0a3b6e8740a8098087ddd98bd9a7b02d9ab802c9ba6
LABEL maintainer="qubership"

ADD ./install /tmp/

USER root
RUN chmod +x /tmp/install.sh && /tmp/install.sh
USER 1000

ENTRYPOINT ["/validation.sh"]
