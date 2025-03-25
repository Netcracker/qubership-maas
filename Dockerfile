FROM ghcr.io/netcracker/qubership/core-base:main-20250325181638-12

ADD --chown=10001:0 maas-service/maas-service /app/maas
ADD --chown=10001:0 maas-service/application.yaml /app/
COPY --chown=10001:0 maas-service/docs /app/

WORKDIR /app

USER 10001:10001

CMD ["/app/maas"]