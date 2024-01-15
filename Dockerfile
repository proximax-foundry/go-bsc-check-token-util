FROM golang:1.21.5-alpine3.19 AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
RUN apk add --no-cache build-base
COPY . ./
RUN CGO_ENABLED=1 go build -o /app/check-bsc-token-util

FROM alpine:3.19
WORKDIR /app
COPY --from=build /app/check-bsc-token-util .
ENTRYPOINT ["/app/check-bsc-token-util"]