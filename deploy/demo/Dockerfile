FROM golang:1.27.2-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/byeclaude ./cmd/byeclaude

FROM alpine:3.24
RUN apk add --no-cache ca-certificates git \
    && adduser -D -H -s /sbin/nologin byeclaude
COPY --from=build /out/byeclaude /usr/local/bin/byeclaude
USER byeclaude
EXPOSE 8080
ENTRYPOINT ["byeclaude"]
CMD ["serve", "--listen", "0.0.0.0:8080", "--max-inflight", "2", "--max-per-client", "1", "--timeout", "60s"]
