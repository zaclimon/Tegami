# syntax=docker/dockerfile:1

FROM --platform=${BUILDPLATFORM} golang:1.17-alpine as builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app
COPY go.mod go.sum tegami.go ./
RUN go mod download
RUN env GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o tegami

FROM alpine

COPY --from=builder /app/tegami /
EXPOSE 2525
CMD ["/tegami"]