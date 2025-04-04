FROM golang:1.23 AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o collecotr cmd/main.go

FROM alpine:latest

WORKDIR /root/
COPY --from=build /app/collector .

ARG LAT1
ARG LNG1
ARG LAT2
ARG LNG2
ARG TOKEN
ARG SVC_ADD
ARG SVC_PORT

ENV LAT1=${LAT1}
ENV LNG1=${LNG1}
ENV LAT2=${LAT2}
ENV LNG2=${LNG2}
ENV TOKEN=${TOKEN}
ENV SVC_ADD=${SVC_ADD}
ENV SVC_PORT=${SVC_PORT}

EXPOSE 50051
CMD ["./collector"]
