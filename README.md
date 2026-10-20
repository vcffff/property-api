# API Task Manager

REST API на Go для работы с недвижимостью: регистрация и вход, объекты, заявки и запись на просмотр. Администратор управляет объектами и статусами заявок и просмотров.

## Стек

- Go 1.26.5, Gin
- PostgreSQL и GORM
- Redis — refresh-сессии и лимит запросов
- JWT — доступ к API
- Kafka — события создания объектов

## Запуск

Нужны Go, Docker и доступная база PostgreSQL. Базу `task_manager` создайте заранее: таблицы API создаёт сам при старте.

**1. Создайте `.env` в корне:**

```dotenv
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=task_manager
REDIS_HOST=localhost:6379
KAFKA_BROKERS=localhost:9092
```

В текущем коде подключение к PostgreSQL требует SSL (`sslmode=require`). Сервер базы должен его поддерживать. Не публикуйте `.env` с паролями.

**2. Запустите Redis и Kafka:**

```sh
docker compose up -d redis kafka
```

Когда Kafka будет готова, создайте топик (один раз):

```sh
docker compose exec kafka /opt/kafka/bin/kafka-topics.sh --bootstrap-server kafka:29092 --create --if-not-exists --topic property.events --partitions 1 --replication-factor 1
```

**3. Запустите API:**

```sh
go run ./cmd/api
```

Адрес сервера: `http://localhost:8080`. Все маршруты начинаются с `/api/v1`.

### API в Docker

Для запуска API в контейнере измените адреса в `.env`:

```dotenv
DB_HOST=host.docker.internal
REDIS_HOST=redis:6379
KAFKA_BROKERS=kafka:29092
```

`host.docker.internal` подходит для PostgreSQL на вашем компьютере при использовании Docker Desktop. Для внешней базы укажите её адрес. PostgreSQL в `docker-compose.yml` не входит.

После запуска Redis, Kafka и создания топика:

```sh
docker compose up -d --build api
```

Если API не запускается, посмотрите логи: `docker compose logs api`.

## Как пользоваться

1. Зарегистрируйтесь через `POST /api/v1/register`.
2. Войдите через `POST /api/v1/login` с теми же данными:

```json
{"email":"user@example.com","password":"ExamplePassword123!"}
```

3. Скопируйте `access_token` из ответа и передавайте его в защищённых запросах:

```http
Authorization: Bearer YOUR_ACCESS_TOKEN
```

Для JSON-запросов добавьте `Content-Type: application/json`.

Access token действует 15 минут. Для обновления отправьте `refresh_token` на `/api/v1/auth/refresh` и сохраните оба новых токена. Для выхода отправьте его на `/api/v1/auth/logout`:

```json
{"refresh_token":"YOUR_REFRESH_TOKEN"}
```

Эти два маршрута не требуют Bearer-токена. Выход закрывает refresh-сессию, но уже выданный access token действует до истечения срока.

## Маршруты

Пути ниже указаны без `/api/v1`. «Пользователь» — любой вошедший пользователь, включая администратора.

| Метод | Путь | Что делает | Доступ |
|---|---|---|---|
| POST | `/register` | Регистрация | Без входа |
| POST | `/login` | Вход и получение токенов | Без входа |
| POST | `/auth/refresh` | Обновить токены | Refresh token |
| POST | `/auth/logout` | Закрыть сессию | Refresh token |
| GET | `/users/me` | Мой профиль | Пользователь |
| GET | `/properties` | Список объектов | Пользователь |
| GET | `/properties/:id` | Объект по ID | Пользователь |
| POST | `/properties` | Создать объект | Администратор |
| PATCH | `/properties/:id` | Изменить объект | Администратор |
| DELETE | `/properties/:id` | Удалить объект | Администратор |
| POST | `/applications` | Подать заявку | Пользователь |
| GET | `/applications/me` | Мои заявки | Пользователь |
| GET | `/admin/applications` | Все заявки | Администратор |
| PATCH | `/admin/applications/:id/status` | Одобрить или отклонить заявку | Администратор |
| GET | `/properties/:id/available-slots?date=YYYY-MM-DD` | Свободное время для просмотра | Пользователь |
| POST | `/viewings` | Записаться на просмотр | Пользователь |
| GET | `/viewings/me` | Мои просмотры | Пользователь |
| GET | `/admin/viewings` | Все просмотры | Администратор |
| PATCH | `/admin/viewings/:id/status` | Изменить статус просмотра | Администратор |

Примеры тел запросов, ответов и правила смены статусов — в [API_CONTRACT.md](API_CONTRACT.md).

### Подать заявку

Объект должен уже существовать. В Insomnia отправьте:

```http
POST http://localhost:8080/api/v1/applications
Authorization: Bearer YOUR_ACCESS_TOKEN
Content-Type: application/json

{"property_id":1}
```

API вернёт `201` и заявку со статусом `pending`. Пользователь определяется по токену. Если на этот объект уже есть ваша заявка `pending` или `approved`, ответ будет `409`.

### Записаться на просмотр

Сначала получите свободные слоты через `/properties/:id/available-slots?date=YYYY-MM-DD`. Затем отправьте выбранный `starts_at`:

```http
POST http://localhost:8080/api/v1/viewings
Authorization: Bearer YOUR_ACCESS_TOKEN
Content-Type: application/json

{"property_id":1,"starts_at":"2030-10-24T10:00:00+05:00"}
```

Дата должна быть в будущем. Просмотр длится 30 минут; начало — каждые полчаса с 10:00 до 18:30 по времени Астаны (UTC+5). Занятый слот вернёт `409`.

### Что важно при проверке

- Общий лимит — **5 запросов в минуту с одного IP**, включая регистрацию и вход. Ответ `429` означает, что нужно подождать.
- Без корректного access token защищённые маршруты вернут `401`, без роли администратора — `403` на админских маршрутах.
- Если создание объекта вернуло `500` из-за Kafka, проверьте список объектов перед повтором: запись могла уже сохраниться.
- При старте API проверяет, нет ли активных дубликатов заявок и слотов просмотра. Если они есть, запуск остановится; данные автоматически не удаляются.

## Роль администратора для локальной проверки

Добавьте в `.env`:

```dotenv
APP_ENV=development
ENABLE_DEV_ROLE_ENDPOINT=true
```

Перезапустите API. Для Docker: `docker compose up -d --build api`.

```http
PATCH http://localhost:8080/api/v1/dev/users/me/role
Authorization: Bearer YOUR_ACCESS_TOKEN
Content-Type: application/json

{"role":"admin"}
```

Сохраните новый `access_token` из ответа. Чтобы вернуть обычную роль, отправьте `{"role":"user"}`. Старые access tokens и refresh-сессии сохраняют прежнюю роль; для новой refresh-сессии войдите заново.

**Не включайте это на общедоступном сервере:** любой вошедший пользователь сможет стать администратором. Без обоих параметров маршрут не подключается и возвращает `404`.

## Тесты

```sh
go test ./...
```

Для тестов с настоящей PostgreSQL создайте отдельную тестовую базу. Пример для PowerShell:

```powershell
$env:APPLICATION_TEST_DATABASE_URL = "postgres://postgres:postgres@localhost:5432/task_manager_test?sslmode=disable"
go test ./internal/routes -count=1
```

Пользователю базы нужны права на создание схем. Тесты создают отдельные схемы и удаляют их после проверки. Без этой переменной проверки PostgreSQL пропускаются, остальные тесты выполняются.
