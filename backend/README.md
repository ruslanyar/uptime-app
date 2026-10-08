# Backend

Для совместного локального запуска из корня репозитория выполните `npm run dev`.
Команда поднимает БД, применяет миграции и запускает API с фронтендом.
Подготовка окружения и остановка — в
[README репозитория](../README.md#запуск-всего-окружения).

Требуются Go 1.27+ и PostgreSQL 17.

Все команды ниже выполняются из `backend/`.

## Структура

[docs/rules/backend/architecture.md](../docs/rules/backend/architecture.md)

## Локальный запуск

```sh
# Один раз создайте личный JWT-секрет (файл игнорируется Git):
(umask 077; printf 'JWT_SECRET=%s\n' "$(openssl rand -hex 32)" > .env.development.local)
docker compose up -d --wait postgres
go run ./cmd/migrate up
go run ./cmd/migrate status
go run ./cmd/api
```

API не применяет миграции автоматически. Требуются Docker и Compose 2.24+.

## Конфигурация

[docs/rules/backend/configuration.md](../docs/rules/backend/configuration.md)

## Миграции и SQL

[docs/guides/backend/migrations.md](../docs/guides/backend/migrations.md)

## Документация API

Установите инструменты командой `npm ci` из корня репозитория.
Из `backend/` выполните `go generate ./cmd/api`, затем откройте
`api/openapi/redoc.html` в браузере. Для генерации только HTML из существующего
Swagger: `sh scripts/generate-redoc.sh`.
HTML использует ReDoc с CDN; для его загрузки нужен доступ к интернету.

## HTTP-контракт

Пути указаны относительно `/api/v1`:

| Метод и путь | Вход | Ответ |
| --- | --- | --- |
| `POST /auth/register` | JSON `name`, `email`, `password` | `201`, `{user: User}`, две cookies |
| `POST /auth/login` | JSON `email`, `password` | `200`, `{user: User}`, две cookies |
| `POST /auth/refresh` | Refresh-cookie | `200`, `{user: User}`, две cookies |
| `POST /auth/logout` | Refresh-cookie | `204`, удаление обеих cookies |
| `GET /auth/me` | Access-cookie или Bearer JWT | `200`, `User` |
| `POST /auth/profile` | Access token; JSON `name`, необязательный `remove_avatar` или multipart `name` + `avatar` | `200`, `{user: User}` |
| `GET /avatars/{filename}` | Без авторизации | `200`, изображение; `404`, если файл отсутствует или имя недопустимо |
| `POST /monitors` | Access token; JSON `url`, `interval_seconds` | `201`, `Monitor` |
| `GET /monitors` | Access token | `200`, `{monitors: Monitor[]}` |
| `PUT /monitors/{id}` | Access token; UUID; JSON `url`, `interval_seconds` | `200`, `Monitor` |
| `DELETE /monitors/{id}` | Access token; UUID | `204`, без тела |

`User` содержит `id`, `name`, `email` и необязательный `avatar_url`.
`Monitor` содержит `id`, `url`, `interval_seconds`, `created_at`, `updated_at`.
Ответы контроллеров авторизации, профиля и мониторов имеют `Cache-Control: no-store`.
Токены в JSON не возвращаются; параметры cookies описаны в
[авторизации](../docs/rules/backend/authentication.md), кеширование изображений —
в [профиле и аватарах](../docs/rules/backend/profile.md).

Маршруты, требующие access token, принимают JWT из access-cookie или
`Authorization: Bearer <JWT>`; присутствующий `Authorization` имеет приоритет.
Неверный, пустой или дублирующийся заголовок возвращает `401`,
даже если cookie действительна.

JSON-запросы требуют `Content-Type: application/json`: принимается один объект,
неизвестные поля, дополнительные JSON-значения и тела более 16 KiB отклоняются.
Для multipart профиля действуют отдельные ограничения из документации аватаров.

Ошибка: `{error:{code,message}}`. Статусы: `400` некорректный запрос,
`401` неверные данные/токен, `403` Origin/CSRF, `404` монитор не найден,
`409` занятый email или URL монитора, `413` превышен лимит multipart, `500` ошибка сервера.
Ответы публичного маршрута аватаров с ошибкой `404` не используют JSON-конверт.

На каждом POST, PUT и DELETE требуется `X-CSRF-Protection: 1`, включая CLI без Origin.
Присутствующий Origin всегда проверяется до выполнения обработчика.
Для разрешённого Origin, включая ответы с ошибками, возвращаются точный
`Access-Control-Allow-Origin`, `Access-Control-Allow-Credentials: true`, `Vary: Origin`.
Preflight OPTIONS проверяет Origin, методы GET/POST/PUT/DELETE и заголовки Content-Type,
Authorization, X-CSRF-Protection, возвращает 204 без auth/CSRF и изменения состояния.
`Sec-Fetch-Site: cross-site` сам по себе не запрещает запрос.

## Авторизация

[docs/rules/backend/authentication.md](../docs/rules/backend/authentication.md)

## Проверки

Для полной проверки из `backend/`:

```sh
docker compose --profile test up -d --wait postgres-test
sh scripts/check-sqlc.sh  # требует sqlc версии из .sqlc-version
sh scripts/check-openapi.sh
go test ./... -count=1
go vet ./...
go build -o bin/api ./cmd/api
```

Подготовка sqlc — в [миграциях и SQL](../docs/guides/backend/migrations.md).
Интеграционные тесты должны выполняться без пропусков; браузерный сценарий
запускается отдельно. Подробности, дополнительные проверки и остановка БД —
в [docs/guides/backend/testing.md](../docs/guides/backend/testing.md).

## Профиль и аватары

[docs/rules/backend/profile.md](../docs/rules/backend/profile.md)

## Мониторы

[docs/rules/backend/monitors.md](../docs/rules/backend/monitors.md)
