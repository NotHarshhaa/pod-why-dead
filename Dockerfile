# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o pod-why-dead .

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates curl

# Create non-root user
RUN addgroup -g 1000 podwhydead && \
    adduser -D -u 1000 -G podwhydead podwhydead

WORKDIR /home/podwhydead

COPY --from=builder /app/pod-why-dead .

# Add kubectl for cluster interaction with verification
RUN KUBECTL_VERSION=$(curl -L -s https://dl.k8s.io/release/stable.txt) && \
    curl -LO "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl" && \
    curl -LO "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl.sha256" && \
    echo "$(cat kubectl.sha256)  kubectl" | sha256sum -c - && \
    chmod +x kubectl && \
    mv kubectl /usr/local/bin/ && \
    rm kubectl.sha256

# Change ownership to non-root user
RUN chown -R podwhydead:podwhydead /home/podwhydead

USER podwhydead

ENTRYPOINT ["./pod-why-dead"]
