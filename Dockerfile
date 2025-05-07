FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY . .

RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o reddit2dynalist .

FROM gcr.io/distroless/static:nonroot
WORKDIR /app/
COPY --from=builder /app/reddit2dynalist .
USER nonroot:nonroot

ENTRYPOINT ["/app/reddit2dynalist"]
