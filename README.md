# tirev

Tiny Sidecar Reverse Proxy Powered by [Parapet](https://github.com/moonrhythm/parapet)

> gcr.io/moonrhythm-containers/tirev

### Example

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
        image: gcr.io/moonrhythm-containers/tirev
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
