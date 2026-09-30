# One image per service: docker build --build-arg SERVICE=orders ...
# Build context is the repo root.
FROM golang:1.27-alpine AS build
ARG SERVICE
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY services/ services/
# Symbols are kept (no -s -w): the eBPF agent reads them to find Go functions.
RUN CGO_ENABLED=0 go build -o /out/${SERVICE} ./services/${SERVICE}

FROM alpine:3.22
ARG SERVICE
COPY --from=build /out/${SERVICE} /usr/local/bin/${SERVICE}
ENV SERVICE=${SERVICE}
ENTRYPOINT ["/bin/sh", "-c", "exec /usr/local/bin/$SERVICE"]
