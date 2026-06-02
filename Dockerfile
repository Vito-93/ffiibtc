FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS build
WORKDIR /src
COPY internal/ ./internal
COPY go.mod go.sum ./
COPY *.go ./
RUN go mod download
RUN go test ./...
ARG TARGETPLATFORM TARGETARCH TARGETOS
RUN echo "Building for ${TARGETPLATFORM}..."
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/ffiiibc

FROM alpine:latest AS release
WORKDIR /app
RUN mkdir -p /app/data
COPY --from=build /out/ffiiibc /app/ffiiibc
EXPOSE 8080
ENTRYPOINT ["/app/ffiiibc"]
