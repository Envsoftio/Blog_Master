FROM golang:1.25-bookworm AS build

WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api
RUN CGO_ENABLED=0 go build -o /out/worker ./cmd/worker
RUN CGO_ENABLED=0 go build -o /out/admincli ./cmd/admincli

FROM debian:bookworm-slim

WORKDIR /app
COPY --from=build /out/api /app/api
COPY --from=build /out/worker /app/worker
COPY --from=build /out/admincli /app/admincli
RUN groupadd --system seoblog \
    && useradd --system --gid seoblog --home-dir /app seoblog \
    && mkdir -p /data \
    && chown -R seoblog:seoblog /app /data
USER seoblog

EXPOSE 8080
CMD ["/app/api"]
