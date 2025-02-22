FROM golang:1.24.0-bookworm

ARG VERSION

RUN apt-get update && \
	apt-get -y install libbrotli-dev

ENV CGO_ENABLED=1

WORKDIR /workspace
ADD go.mod go.sum ./
RUN go mod download

ADD . .
RUN go build \
        -o tirev \
        -ldflags "-w -s -X main.version=$VERSION" \
        -tags=cbrotli \
        main.go

FROM debian:bookworm-slim

RUN apt-get update && \
	apt-get -y install libbrotli1 ca-certificates && \
	rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=0 /workspace/tirev ./
ENTRYPOINT ["/app/tirev"]
