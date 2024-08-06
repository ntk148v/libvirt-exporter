FROM golang:1.22-alpine AS builder
# Requirements/Dependencies
RUN apk --update --no-cache add ca-certificates g++ git libvirt-dev libvirt
# Build
COPY . $GOPATH/src/libvirt-exporter/
WORKDIR $GOPATH/src/libvirt-exporter/
RUN git config --global --add safe.directory /app && \
    go build -ldflags="-s -w" -o /bin/libvirt-exporter -mod=vendor .

FROM alpine:3.20
RUN apk --update --no-cache add ca-certificates libvirt-dev
COPY --from=builder /bin/libvirt-exporter /bin/libvirt-exporter
# Default listen on port 9177
EXPOSE 9177
# Start
ENTRYPOINT ["/bin/libvirt-exporter"]
