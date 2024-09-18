# Build the application from source
FROM golang:1.23 AS build-stage

# Set destination for COPY
WORKDIR /app

# Download Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY *.go ./

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /docker-kubernetes-api

# Run the tests in the container
FROM build-stage AS run-test-stage
RUN go test -v ./

# Deploy the application binary into a lean image
FROM gcr.io/distroless/base-debian11 AS build-release-stage

WORKDIR /

COPY --from=build-stage /docker-kubernetes-api /docker-kubernetes-api

ENV GIN_MODE=release
ENV ENVIRONMENT=production

EXPOSE 8080

USER nonroot:nonroot

# Run
CMD ["/docker-kubernetes-api", "/config-volume/config_sa"]