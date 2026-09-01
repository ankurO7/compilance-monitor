FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server

FROM alpine:3.20
RUN adduser -D -u 10001 appuser
COPY --from=build /out/server /usr/local/bin/server
USER appuser
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/server"]