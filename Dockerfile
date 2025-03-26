FROM ghcr.io/netcracker/qubership/core-base:main-20250325181638-12

COPY --chown=10001:0 --chmod=555 maas-service/maas-service /app/maas
COPY --chown=10001:0 --chmod=444 maas-service/application.yaml /app/
COPY --chown=10001:0 --chmod=444 maas-service/docs/swagger.json /app/
COPY --chown=10001:0 --chmod=444 maas-service/docs/swagger.yaml /app/

WORKDIR /app

USER 10001:10001

CMD ["/app/maas"]