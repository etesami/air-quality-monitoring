FROM golang:1.23 AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o dashboard cmd/main.go

FROM alpine:3.21.3
RUN apk add --no-cache libc6-compat

WORKDIR /root/
COPY --from=build /app/dashboard .

EXPOSE 50052 8001
CMD ["/root/dashboard"]
