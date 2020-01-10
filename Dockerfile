FROM gcr.io/moonrhythm-containers/alpine

RUN mkdir -p /app
WORKDIR /app

COPY tirev ./
ENTRYPOINT ["/app/tirev"]
