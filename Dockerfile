FROM golang:1.17.7-alpine

RUN apk --no-cache add git build-base brotli-dev

ENV CGO_ENABLED=1
WORKDIR /workspace

ADD go.mod go.sum ./
RUN go mod download

ADD . .
RUN go build -o tirev -ldflags "-w -s" -tags=cbrotli main.go

FROM alpine

RUN apk add --no-cache ca-certificates tzdata brotli

WORKDIR /app

COPY --from=0 /workspace/tirev ./
ENTRYPOINT ["/app/tirev"]
