FROM golang:1.22-alpine as builder
# Requirements/Dependencies
RUN apk add ca-certificates g++ git libvirt-dev libvirt
# Build
COPY .  $GOPATH/src/libvirt-exporter
WORKDIR $GOPATH/src/libvirt-exporter
RUN git config --global --add safe.directory /app &&\
    go build -ldflags="-s -w" -o /bin/libvirt-exporter .

FROM scratch
COPY --from=builder /bin/libvirt-exporter /bin/libvirt-exporter
USER nobody:nobody
# Default listen on port 9177
EXPOSE 9177
# Start
CMD ["/bin/libvirt-exporter"]
