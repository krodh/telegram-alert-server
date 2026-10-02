# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.23-bookworm AS builder
WORKDIR /app
COPY go.mod ./
COPY main.go ./
ARG TARGETOS=linux TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags="-s -w" -o server .

FROM gcr.io/distroless/static-debian12
COPY --from=builder /app/server /server
EXPOSE 8080
ENTRYPOINT ["/server"]
