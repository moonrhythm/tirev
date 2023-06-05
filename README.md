# tirev

Tiny Sidecar Reverse Proxy Powered by [Parapet](https://github.com/moonrhythm/parapet)

> gcr.io/moonrhythm-containers/tirev

### Config

### Type

| Type     | Example |
|----------|---------|
| header   | a=b     |
| []header | a=b,c=d |
| []string | abc,def |

#### Env Config

| Name                   | Type     | Default  | Description                                                  |
|------------------------|----------|----------|--------------------------------------------------------------|
| FRONT                  | boolean  | false    | Run proxy as frontend without another reverse proxy in front |
| PORT                   | int      | 8080     | HTTP(S) Port                                                 |
| NO_HEALTHZ             | boolean  | false    | Disable health check path                                    |
| HEALTHZ_PATH           | string   | /healthz | Health check path                                            |
| NO_PROM                | boolean  | false    | Disable prometheus endpoint                                  |
| PROM_PORT              | int      | 9187     | Prometheus endpoint port                                     |
| NO_GZIP                | boolean  | false    | Disable gzip compression                                     |
| NO_BR                  | boolean  | false    | Disable br compression                                       |
| NO_LOG                 | boolean  | false    | Disable log                                                  |
| NO_REQID               | boolean  | false    | Disable request id                                           |
| REQHEADER_SET          | []header |          | Set (override) headers to request before send to upstream    |
| REQHEADER_ADD          | []header |          | Add headers to request before send to upstream               |
| REQHEADER_DEL          | []string |          | Delete headers from request before send to upstream          |
| RESPHEADER_SET         | []header |          | Set (override) headers to response before send to client     |
| RESPHEADER_ADD         | []header |          | Add headers to response before send to client                |
| RESPHEADER_DEL         | []string |          | Delete headers from response before send to client           |
| RATELIMIT_S            | int      | 0        | Allow requests per second per client ip                      |
| RATELIMIT_M            | int      | 0        | Allow requests per minute per client ip                      |
| RATELIMIT_H            | int      | 0        | Allow requests per hour per client ip                        |
| BODY_BUFFERREQUEST     | boolean  | false    | Buffered request body before send to upstream                |
| BODY_LIMITREQUEST      | int64    | 0        | Limit request body (bytes)                                   |
| REDIRECT_HTTPS         | boolean  | false    | Redirect HTTP request to HTTPS                               |
| HSTS                   | string   |          | Add HSTS header ("", "preload", "default")                   |
| REDIRECT_WWW           | string   |          | Redirect www/non-www config ("", "www", "non")               |
| UPSTREAM_ADDR          | []string |          | Upstream addresses (ex. "192.168.0.2:8080,192.168.0.3:8080") |
| UPSTREAM_PROTO         | string   | http     | Upstream protocol ("http", "h2c", "https", "unix")           |
| UPSTREAM_OVERRIDE_HOST | string   |          | Override host header before send to upstream                 |
| UPSTREAM_PATH          | string   |          | Add prefix path to request                                   |
| UPSTREAM_MAXIDLECONNS  | int      | 32       | Max idle connections to upstream                             |
| TLS_KEY                | string   |          | TLS key file path                                            |
| TLS_CERT               | string   |          | TLS cert file path                                           |
| TLS_MIN_VERSION        | string   |          | TLS min version (tls1.0, tls1.1, tls1.2, tls1.3)             |
| AUTOCERT_DIR           | string   |          | Directory to store certificate (lets encrypt)                |
| AUTOCERT_HOSTS         | []string |          | Host to request certificate (lets encrypt)                   |
| AUTH_BASIC_USERNAME    | string   |          | Basic auth username                                          |
| AUTH_BASIC_PASSWORD    | string   |          | Basic auth password                                          |

### Example

#### Docker

```sh
#!/bin/bash
NAME=https
IMAGE=gcr.io/moonrhythm-containers/tirev
TAG=v1.3.16
ARGS=
MOUNT_SOURCE=/data/https
MOUNT_TARGET=/cert
PORT_SOURCE=443
PORT_TARGET=443

docker pull $IMAGE:$TAG
docker stop $NAME
docker rm $NAME
docker run -d --restart=always --name=$NAME \
  -e FRONT=true \
  -e PORT=443 \
  -e NO_HEALTHZ=true \
  -e NO_PROM=true \
  -e NO_REQID=true \
  -e UPSTREAM_ADDR=app:8080 \
  -e UPSTREAM_PROTO=http \
  -e TLS_KEY=/cert/tls.key \
  -e TLS_CERT=/cert/tls.crt \
  -e REDIRECT_WWW=non \
  --link app:app \
  -p $PORT_SOURCE:$PORT_TARGET -v $MOUNT_SOURCE:$MOUNT_TARGET $IMAGE:$TAG $ARGS
```

#### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: echoserver
  name: echoserver
spec:
  replicas: 2
  selector:
    matchLabels:
      app: echoserver
  template:
    metadata:
      annotations:
        prometheus.io/port: "9187"
        prometheus.io/scrape: "true"
      labels:
        app: echoserver
      name: echoserver
    spec:
      containers:
      - name: echoserver
        image: gcr.io/google-containers/echoserver:1.10
        ports:
        - containerPort: 8080
      - name: tirev
        image: gcr.io/moonrhythm-containers/tirev:v1.3.16
        env:
        - name: PORT
          value: "80"
        - name: UPSTREAM_ADDR
          value: 127.0.0.1:8080
        - name: RATELIMIT_S
          value: "30"
        - name: BODY_LIMITREQUEST
          value: "15728640"
        ports:
        - containerPort: 80
        livenessProbe:
          httpGet:
            path: /healthz
            port: 80
            scheme: HTTP
          successThreshold: 1
          failureThreshold: 2
          timeoutSeconds: 5
        readinessProbe:
          httpGet:
            path: /healthz?ready=1
            port: 80
            scheme: HTTP
          successThreshold: 1
          failureThreshold: 2
          timeoutSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  labels:
    app: echoserver
  name: echoserver
spec:
  type: ClusterIP
  selector:
    app: echoserver
  ports:
  - name: http
    port: 80
    targetPort: 80
```
