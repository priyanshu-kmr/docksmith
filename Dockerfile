# Docksmith requires Linux namespaces, chroot, and mount.
# This image provides the Linux environment for running on macOS via Docker.

FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /docksmith .

FROM alpine:3.18
RUN apk add --no-cache bash coreutils curl tar
COPY --from=builder /docksmith /usr/local/bin/docksmith
COPY scripts/setup-base-image.sh /usr/local/bin/setup-base-image.sh
RUN mkdir -p /root/.docksmith
WORKDIR /workspace
ENTRYPOINT ["/usr/local/bin/docksmith"]
