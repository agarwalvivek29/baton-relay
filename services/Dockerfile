# One image per service: docker build --build-arg SERVICE=orders ...
# Build context is the repo root; the `baton` additional context is the SDK
# checkout (development only, see go.mod replace).
FROM golang:1.26-alpine AS build
ARG SERVICE
WORKDIR /src/baton-relay
COPY --from=baton . /src/baton
COPY go.mod go.sum* ./
COPY services/ services/
# Symbols are kept (no -s -w): the eBPF agent reads them to find Go functions.
RUN CGO_ENABLED=0 go build -o /out/${SERVICE} ./services/${SERVICE}

FROM alpine:3.22
ARG SERVICE
COPY --from=build /out/${SERVICE} /usr/local/bin/${SERVICE}
ENV SERVICE=${SERVICE}
ENTRYPOINT ["/bin/sh", "-c", "exec /usr/local/bin/$SERVICE"]
