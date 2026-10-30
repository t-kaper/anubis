### ---------- BUILDER ----------
FROM golang:1.23-alpine AS builder

# Добавляем git чтобы go mod мог подтягивать private deps
RUN apk add --no-cache git

WORKDIR /app

# Копируем только зависимостьные файлы сначала
COPY go.mod go.sum ./
RUN go mod download

# Копируем всё остальное
COPY . .

# Сборка без CGO
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/anubis ./cmd/server/main.go


### ---------- FINAL ----------
FROM alpine:3.19

# Добавляем CA сертификаты (для HTTPS)
RUN apk add --no-cache ca-certificates tzdata

# Создаём unprivileged user
RUN adduser -D anubis
USER anubis

# Копируем бинарь
COPY --from=builder /app/anubis /usr/local/bin/anubis

# Рабочая директория
WORKDIR /app

# EXPOSE 1010 # если надо

# Опционально HEALTHCHECK
HEALTHCHECK CMD pidof anubis || exit 1

CMD ["/usr/local/bin/anubis"]
