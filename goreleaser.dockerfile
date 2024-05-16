FROM alpine:3.19
RUN apk add --no-cache libc6-compat libvirt-dev
COPY prometheus-libvirt-exporter /prometheus-libvirt-exporter
# Default listen on port 9177
EXPOSE 9177
ENTRYPOINT ["/prometheus-libvirt-exporter"]
