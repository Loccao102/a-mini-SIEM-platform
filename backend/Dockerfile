FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/siem-api ./cmd/api
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/siem-ingest ./cmd/ingest
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/siem-parser ./cmd/parser

FROM alpine:3.22
RUN adduser -D -H -u 10001 appuser
USER appuser
COPY --from=build /out/siem-api /siem-api
COPY --from=build /out/siem-ingest /siem-ingest
COPY --from=build /out/siem-parser /siem-parser
EXPOSE 8080
CMD ["/siem-api"]
