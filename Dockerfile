FROM golang:1.19-alpine as builder

RUN apk add --no-cache make git

COPY main.go go.mod go.sum /app/

ENV CGO_ENABLED=0 \
  GOOS=linux \
  GOARCH=amd64
WORKDIR /app
RUN go get && go build -o /redeemlog

# Lightweight Runtime Env
FROM gcr.io/distroless/base-debian10
COPY --from=builder /redeemlog /redeemlog
CMD ["/redeemlog"]
