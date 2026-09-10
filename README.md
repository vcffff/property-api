# API Task Manager

Простой REST API на Go (Gin + GORM + PostgreSQL): регистрация, логин с JWT и rate limiting.

## Стек

- Go 1.26
- Gin (HTTP)
- GORM + PostgreSQL
- JWT (golang-jwt/v5)

## Быстрый старт

1. Склонируйте репозиторий и создайте `.env` в корне:

```
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=task_manager
```

2. Запустите:

```
go run ./cmd/api
```

Сервер поднимется на `http://localhost:8080`. Миграции применяются автоматически при старте.

## Docker

```
docker build -t api-task-manager .
docker run --env-file .env -p 8080:8080 api-task-manager
```

## API

| Метод | Путь                  | Описание                          |
|-------|-----------------------|-----------------------------------|
| POST  | `/api/v1/register`    | Регистрация пользователя          |
| POST  | `/api/v1/login`       | Логин, возвращает JWT             |
| GET   | `/api/v1/users/me`    | Текущий пользователь (Bearer JWT) |

## Тесты

```
go test ./...
```
