FROM golang:1.23 AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o local-storage cmd/main.go

FROM ubuntu:22.04

WORKDIR /root/
COPY --from=build /app/local-storage .

EXPOSE 50052 8001
CMD ["/root/local-storage"]
