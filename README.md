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

## Dev: смена собственной роли

Только для локальной разработки. По умолчанию endpoint отсутствует (404).
Для включения добавьте в `.env` оба параметра:

```dotenv
APP_ENV=development
ENABLE_DEV_ROLE_ENDPOINT=true
```

Пересоберите API: `docker compose up -d --build api`.
Не включайте эти параметры на общедоступном сервере: любой авторизованный
пользователь сможет назначить себе роль администратора.

```http
PATCH http://localhost:8080/api/v1/dev/users/me/role
Authorization: Bearer YOUR_ACCESS_TOKEN
Content-Type: application/json

{"role":"admin"}
```

Для возврата обычной роли отправьте `{"role":"user"}`. Ответ 200 содержит
`id`, `role`, новый `access_token`, `token_type` и `expires_in`.
Замените access token в Insomnia на новый. Ранее выданные access tokens
действуют до истечения срока; существующие refresh-сессии сохраняют старую роль.
Для получения refresh token с новой ролью выполните `/api/v1/login` заново.

## API

| Метод | Путь                  | Описание                          |
|-------|-----------------------|-----------------------------------|
| POST  | `/api/v1/register`    | Регистрация пользователя          |
| POST  | `/api/v1/login`       | Логин, возвращает JWT             |
| GET   | `/api/v1/users/me`    | Текущий пользователь (Bearer JWT) |
| POST  | `/api/v1/applications` | Создать заявку на объект (Bearer JWT) |
| GET   | `/api/v1/applications/me` | Заявки текущего пользователя (Bearer JWT) |
| GET   | `/api/v1/admin/applications` | Все заявки (Bearer JWT, роль `admin`) |

### Заявки через Insomnia

Получите `access_token` через `/api/v1/login`. Для создания заявки отправьте:

```http
POST http://localhost:8080/api/v1/applications
Authorization: Bearer USER_TOKEN
Content-Type: application/json

{"property_id": 1}
```

Объект с указанным ID должен существовать. Ответ — `201 Created` с полями
`id`, `user_id`, `property_id`, `status: "pending"`, `created_at`, `updated_at`.
Пользователь определяется по JWT; статус задаётся сервером.

- Повторный запрос при наличии заявки `pending` или `approved` — `409 Conflict`.
- Несуществующий объект — `404 Not Found`; некорректный `property_id` — `400 Bad Request`.
- `GET /api/v1/applications/me` — `200 OK`, только свои заявки, сначала новые.
- `GET /api/v1/admin/applications` — `200 OK` для администратора, `403 Forbidden` для пользователя.
- Без корректного JWT — `401 Unauthorized`. Пустой список возвращается как `[]`.

При запуске создаётся частичный уникальный индекс
`idx_applications_one_active_per_user_property`, защищающий от одновременных
активных заявок одного пользователя на один объект. История заявок с другими
статусами сохраняется и не блокирует новую заявку. Если активные дубликаты уже
есть, запуск остановится с понятной ошибкой; данные автоматически не меняются.
Для поиска таких записей:

```sql
SELECT user_id, property_id, COUNT(*)
FROM applications
WHERE status IN ('pending', 'approved')
GROUP BY user_id, property_id
HAVING COUNT(*) > 1;
```

Общий лимит API — 5 запросов в минуту с одного IP. При ручной проверке учитывайте
также запросы регистрации и входа; ответ `429` означает срабатывание лимита.

## Тесты

```
go test ./...
```

Интеграционные тесты заявок проверяют настоящую PostgreSQL: создание и чтение,
изоляцию пользователей, права администратора, конкурирующие вставки и миграцию.
Укажите подключение к тестовой базе с правом создания схем (PowerShell):

```powershell
$env:APPLICATION_TEST_DATABASE_URL = "postgres://postgres:postgres@localhost:5432/task_manager_test?sslmode=disable"
go test ./internal/routes -run Application -count=1
```

Каждый тест создаёт отдельную схему и удаляет её после завершения.
Без `APPLICATION_TEST_DATABASE_URL` проверки PostgreSQL пропускаются,
а проверки JWT, ролей и валидации выполняются.
