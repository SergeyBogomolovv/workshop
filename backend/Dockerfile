# Сборка API и программы интерактивного создания администратора.
FROM golang:1.26.1-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/api ./cmd/api \
    && CGO_ENABLED=0 go build -trimpath -o /out/create-admin ./cmd/create-admin

FROM alpine:3.23.3

RUN apk add --no-cache tzdata \
    && adduser -D -u 10001 app

COPY --from=build /out/ /usr/local/bin/

USER app
EXPOSE 8080

ENTRYPOINT ["api"]
