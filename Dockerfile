FROM golang:latest AS build

WORKDIR /app

COPY maas-service/ .

RUN go mod download
RUN go build -o maas-service .

FROM ghcr.io/netcracker/qubership/core-base:main-20250325181638-12 AS run

COPY --chown=10001:0 --chmod=555 --from=build app/maas-service /app/maas
COPY --chown=10001:0 --chmod=444 --from=build app/application.yaml /app/
COPY --chown=10001:0 --chmod=444 --from=build app/docs/swagger.json /app/
COPY --chown=10001:0 --chmod=444 --from=build app/docs/swagger.yaml /app/

WORKDIR /app

USER 10001:10001

CMD ["/app/maas"]