FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.26-alpine AS server
# The delivery map links the cubiomes C library through cgo.
RUN apk add --no-cache build-base
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
RUN CGO_ENABLED=1 go build -trimpath -tags netgo,osusergo -ldflags="-s -w -extldflags '-static'" -o /out/hamazon ./cmd/hamazon

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata \
	&& adduser -D -H -u 10001 hamazon \
	&& mkdir -p /data && chown hamazon /data
COPY --from=server /out/hamazon /usr/local/bin/hamazon
USER hamazon
ENV HAMAZON_DATA_DIR=/data HAMAZON_ADDR=:8080
VOLUME /data
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["hamazon"]
CMD ["serve"]
