FROM golang:1.17.6-alpine as builder
# Install git + SSL ca certificates
# Git is required for fetching the dependencies
# Ca-certificates is required to call HTTPS endpoints
RUN apk update && apk add --no-cache git ca-certificates tzdata curl && update-ca-certificates

# Create appuser
RUN adduser -D -g '' appuser

WORKDIR $GOPATH/src/app
COPY . .

# Fetch dependencies
RUN go mod download

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="$GOFLAGS" -a -installsuffix cgo -o /go/bin/app .

# Use an unprivileged user
USER appuser

HEALTHCHECK CMD /go/bin/app health-check

ENTRYPOINT ["/go/bin/app"]
