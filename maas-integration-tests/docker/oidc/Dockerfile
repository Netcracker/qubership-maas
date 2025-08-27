FROM golang:1.23 AS build

WORKDIR /app

COPY ./server.go .

RUN CGO_ENABLED=0 GOOS=linux go build -o server server.go

FROM alpine:3.20

WORKDIR /app/

COPY --from=build /app/server .

EXPOSE 8080

CMD ["./server"]
