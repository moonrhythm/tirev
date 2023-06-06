FROM golang:1.20.4-bullseye

RUN apt-get update && \
	apt-get -y install libbrotli-dev

ENV CGO_ENABLED=1

WORKDIR /workspace
ADD go.mod go.sum ./
RUN go mod download

ADD . .
RUN go build \
        -o tirev \
        -ldflags "-w -s" \
        -tags=cbrotli \
        main.go

FROM debian:bullseye-slim

RUN apt-get update && \
	apt-get -y install libbrotli1 ca-certificates && \
	rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=0 /workspace/tirev ./
ENTRYPOINT ["/app/tirev"]
