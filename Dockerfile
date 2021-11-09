# syntax=docker/dockerfile:1

FROM golang:1.17-alpine as builder
WORKDIR /app
COPY go.mod go.sum tegami.go ./
RUN go mod download
RUN go build -o tegami

FROM golang:1.17-alpine

COPY --from=builder /app/tegami ./
EXPOSE 2525
CMD ["./tegami"]