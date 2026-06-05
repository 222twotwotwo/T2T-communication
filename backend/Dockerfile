FROM golang:1.22-alpine AS build

WORKDIR /src
COPY backend/go.mod backend/go.sum* ./
RUN go mod download
COPY backend/ ./
RUN go build -o /out/t2t-server ./cmd/server

FROM alpine:3.20

WORKDIR /app
COPY --from=build /out/t2t-server /app/t2t-server
COPY config /app/config

EXPOSE 8080
CMD ["/app/t2t-server"]
