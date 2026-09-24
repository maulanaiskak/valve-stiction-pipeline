FROM golang:1.25 AS build
WORKDIR /app
COPY ingestion/go.mod ingestion/go.sum ./
RUN go mod download
COPY ingestion/ .
RUN CGO_ENABLED=0 GOOS=linux go build -o /ingestion .

FROM gcr.io/distroless/static-debian12
COPY --from=build /ingestion /ingestion
ENTRYPOINT ["/ingestion"]
