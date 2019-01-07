FROM golang:1.11 AS build-env
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -o /agent ./cmd/webhook-proxy-client

FROM alpine:3.6
COPY --from=build-env /agent /agent
RUN apk update && apk add --no-cache ca-certificates
ENTRYPOINT [ "/agent" ]
