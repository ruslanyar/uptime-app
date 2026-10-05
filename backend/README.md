# Backend

Go 1.27+; PostgreSQL 17. Реализованы регистрация, вход, обновление access JWT,
выход и получение пользователя. Мониторинг, подтверждение email,
восстановление пароля и интеграция приложения фронтенда пока отсутствуют.

Все команды ниже выполняются из `backend/`.

## Структура

- `cmd/api` — API; `cmd/migrate` — отдельный запуск миграций.
- `internal/app` — подключение БД, HTTP-сервер, таймауты и graceful shutdown.
- `internal/config`, `internal/auth` — настройки и сервис авторизации.
- `internal/controllers/auth` — контроллеры регистрации, входа, refresh, logout и `/me`.
- `internal/httpapi` — маршруты, CORS/CSRF middleware и общие JSON-ответы.
- `internal/storage/postgres` — транзакции и преобразование типов/ошибок БД.
- `internal/storage/postgres/queries` — SQL; `sqlc` — коммитируемый генерируемый пакет.
- `migrations` — общая схема для Tern и sqlc.

## Локальный запуск

```sh
docker compose up -d --wait postgres
export DATABASE_URL='postgres://uptime:local-development-only@localhost:5432/uptime?sslmode=disable'
export HTTP_ADDR=':8080'
export JWT_SECRET='<replace-with-at-least-32-random-bytes>'
export JWT_ISSUER='uptime-api'
export JWT_AUDIENCE='uptime-client'
export ALLOWED_ORIGINS='http://localhost:3000'
export COOKIE_SECURE=false
export COOKIE_SAME_SITE=lax
go run ./cmd/migrate up
go run ./cmd/migrate status
go run ./cmd/api
```

Пароли Compose предназначены только для локальных одноразовых баз, не для production.
`JWT_SECRET` замените случайным секретом, например результатом `openssl rand -base64 32`.
API не загружает `.env` автоматически: экспортируйте переменные в окружение процесса.
API не применяет миграции самостоятельно; применяйте их перед запуском/развёртыванием.

Для фронтенда и API на несвязанных HTTPS-доменах:

```sh
export DATABASE_URL='postgres://<user>:<password>@<database-host>/<database>?sslmode=verify-full'
export HTTP_ADDR=':8080'
export JWT_SECRET='<replace-with-at-least-32-random-bytes>'
export JWT_ISSUER='https://api.example.net'
export JWT_AUDIENCE='https://dashboard.example.org'
export ALLOWED_ORIGINS='https://dashboard.example.org'
export COOKIE_SECURE=true
export COOKIE_SAME_SITE=none
```

Завершите HTTPS на reverse proxy перед API. В production задавайте отдельные
учётные данные и защищённое соединение PostgreSQL. Не сохраняйте секреты в Git.

| Переменная | Значение |
|---|---|
| `DATABASE_URL` | Обязательный PostgreSQL URL |
| `HTTP_ADDR` | Адрес HTTP, по умолчанию `:8080` |
| `JWT_SECRET` | Обязательный секрет: минимум 32 случайных байта |
| `JWT_ISSUER`, `JWT_AUDIENCE` | Обязательные issuer и audience |
| `ALLOWED_ORIGINS` | Обязательный список точных origins через запятую |
| `COOKIE_SECURE` | `true` по умолчанию |
| `COOKIE_SAME_SITE` | `none` по умолчанию; также `lax`, `strict` |

Wildcard, `null`, origins с путями/credentials/query/fragment не допускаются.
Схема и порт входят в origin; поддомены автоматически не разрешаются.
`none` с `COOKIE_SECURE=false` вызывает ошибку запуска.

## Миграции и SQL

Tern **v2.4.3** закреплён в `go.mod`. Команды используют отдельное соединение,
таблицу версии `public.schema_version`, транзакции и PostgreSQL advisory lock Tern:

```sh
go run ./cmd/migrate up      # до последней версии
go run ./cmd/migrate status  # current=N target=M
go run ./cmd/migrate down    # откат ровно одной миграции
```

Новые файлы называйте `00002_description.sql`, `00003_description.sql` и т.д.,
с последовательными номерами шириной пять цифр. В одном файле обычный SQL применения
и отката разделяются строкой `---- create above / drop below ----`.
Не используйте шаблоны, Go-миграции или отключение транзакций.
Не редактируйте уже применённые миграции; конфликты номеров между ветками разрешайте
до применения. В production исправляйте схему новой миграцией. `down` предназначен
для разработки и тестов и может удалять данные. Ошибка миграции откатывает её
транзакцию без продвижения версии, ранее применённые миграции сохраняются.

sqlc **v1.31.1** закреплён в `.sqlc-version`; читает SQL применения из того же каталога
миграций, не SQL отката. Отдельного `schema.sql` нет.

```sh
go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
export PATH="$(go env GOPATH)/bin:$PATH"
sqlc compile
sqlc generate
sh scripts/check-sqlc.sh
```

После изменения SQL выполните генерацию и включите пакет `internal/storage/postgres/sqlc`
в коммит. Генерируемый код вручную не редактируйте. Скрипт проверяет версию, SQL и
две генерации без расхождений с сохранённым пакетом. Обычная Go-сборка sqlc не требует.

## HTTP-контракт

Префикс `/api/v1/auth`:

| Метод и путь | Вход | Ответ |
|---|---|---|
| `POST /register` | JSON `name`, `email`, `password` | `201`, `{user: {id,name,email}}`, две cookies |
| `POST /login` | JSON `email`, `password` | `200`, `{user: {id,name,email}}`, две cookies |
| `POST /refresh` | Refresh-cookie | `200`, `{user: {id,name,email}}`, две cookies |
| `POST /logout` | Refresh-cookie | `204`, удаление обеих cookies |
| `GET /me` | Access-cookie или `Authorization: Bearer <JWT>` | `200`, `{id,name,email}` |

Ответ регистрации, входа и refresh: `{user:{id,name,email}}` с
`Cache-Control: no-store`. Токены и поля `token_type` / `expires_in` в JSON
не возвращаются. Это несовместимое изменение прежнего контракта:
клиенты, ожидающие `access_token` в JSON, должны перейти на cookies.

Оба токена передаются в HttpOnly cookies без Domain, с настроенными
Secure/SameSite:

| Cookie | Path | Срок |
|---|---|---|
| `access_token` | `/api/v1` | 24 часа |
| `refresh_token` | `/api/v1/auth` | Оставшееся время 30-дневной сессии |

Refresh заменяет обе cookies, не продлевая абсолютный срок сессии.
`/me` проверяет JWT из access-cookie; присутствующий `Authorization` имеет
приоритет. Неверный, пустой или дублирующийся заголовок возвращает `401`,
даже если cookie действительна. Cookies выдаются только после успешного commit.
Logout и refresh с ответом `401` удаляют обе cookies с исходными параметрами;
внутренние ошибки refresh и logout cookies не изменяют. Logout без сессии
идемпотентен и также удаляет обе cookies.

Logout и повторное использование старого refresh отзывают refresh-сессию;
скопированный ранее access JWT действует до своего истечения. Входы создают
независимые сессии. БД хранит только SHA-256-хеши refresh и сохраняет
использованные токены для обнаружения повторного использования.

Имя: 1–100 символов после trim; email: trim + нижний регистр, корректный формат,
до 254 байт; пароль: 15–128 Unicode-символов, пробелы сохраняются, требований к составу нет.
Регистрация и вход принимают только JSON, неизвестные поля и тела более 16 KiB отклоняются.

Ошибка: `{error:{code,message}}`. Статусы: `400` некорректный запрос,
`401` неверные данные/токен, `403` Origin/CSRF, `409` занятый email, `500` ошибка сервера.
Неизвестный email и неверный пароль возвращают одинаковую ошибку входа.
Logout без сессии идемпотентен.

```sh
curl -i -c /tmp/uptime-cookies.txt \
  -H 'Content-Type: application/json' -H 'X-CSRF-Protection: 1' \
  -d '{"name":"Example","email":"user@example.com","password":"example password here"}' \
  http://localhost:8080/api/v1/auth/register
curl -i -b /tmp/uptime-cookies.txt -c /tmp/uptime-cookies.txt \
  -H 'Content-Type: application/json' -H 'X-CSRF-Protection: 1' \
  -d '{"email":"user@example.com","password":"example password here"}' \
  http://localhost:8080/api/v1/auth/login
curl -i -b /tmp/uptime-cookies.txt -c /tmp/uptime-cookies.txt \
  -H 'X-CSRF-Protection: 1' http://localhost:8080/api/v1/auth/refresh -X POST
curl -i -b /tmp/uptime-cookies.txt \
  http://localhost:8080/api/v1/auth/me
curl -i -b /tmp/uptime-cookies.txt -c /tmp/uptime-cookies.txt \
  -H 'X-CSRF-Protection: 1' http://localhost:8080/api/v1/auth/logout -X POST
```

На каждом POST требуется `X-CSRF-Protection: 1`, включая CLI без Origin.
Присутствующий Origin всегда проверяется до выполнения обработчика.
Для разрешённого Origin, включая ответы с ошибками, возвращаются точный
`Access-Control-Allow-Origin`, `Access-Control-Allow-Credentials: true`, `Vary: Origin`.
Preflight OPTIONS проверяет Origin, методы GET/POST и заголовки Content-Type,
Authorization, X-CSRF-Protection, возвращает 204 без auth/CSRF и изменения состояния.
`Sec-Fetch-Site: cross-site` сам по себе не запрещает запрос.

Клиент для всех маршрутов, включая `/me`, использует `credentials: "include"`.
На каждом POST нужен CSRF-заголовок; токены не читаются из JSON или cookies
и не сохраняются в JavaScript. Например:

```js
await fetch(`${apiBase}/api/v1/auth/refresh`, {
  method: "POST",
  credentials: "include",
  headers: { "X-CSRF-Protection": "1" },
});
```

На клиенте сериализуйте refresh: одновременные запросы одним токеном вызовут отзыв
сессии при обнаружении повторного использования. Браузер может блокировать
сторонние cookie даже с правильным CORS и `SameSite=None; Secure`
([MDN](https://developer.mozilla.org/en-US/docs/Web/HTTP/Guides/CORS#third-party_cookies)).
В таком окружении потребуется проксирование API через origin фронтенда;
реализация прокси в эту задачу не входит.

## Проверки

Используются стандартный Go testing, настоящая PostgreSQL через Docker Compose,
а для отдельного браузерного сценария — Node.js и Playwright.

```sh
sh scripts/check-sqlc.sh  # перед Go-проверками
go test ./...
go vet ./...
go build -o bin/api ./cmd/api

docker compose --profile test up -d --wait postgres-test
export TEST_DATABASE_URL='postgres://uptime_test:local-test-only@localhost:55432/uptime_test?sslmode=disable'
go test ./... -count=1
go test -race ./... -count=1
```

Интеграционные тесты создают уникальные временные БД и удаляют их после завершения;
пользователь `TEST_DATABASE_URL` должен иметь CREATEDB. Не используйте production URL.
Без `TEST_DATABASE_URL` они пропускаются; это не считается успешной интеграционной проверкой.
База тестового сервиса хранится в tmpfs и изолирована от volume базы разработки.

Браузерный тест поднимает временные HTTPS API и страницу на разных сайтах
`api.auth-service.test` и `frontend.auth-client.test`; приложение фронтенда не меняет.
Chromium разрешает сторонние cookie для этого сценария; тестовые TLS-сертификаты
принимаются только внутри браузерного контекста проверки.

```sh
npm install --prefix /tmp/uptime-auth-browser --no-save playwright@1.56.1
export PLAYWRIGHT_MODULE=/tmp/uptime-auth-browser/node_modules/playwright
# Если Chrome установлен, укажите его.
export BROWSER_EXECUTABLE=/usr/bin/google-chrome

# Альтернатива без системного Chrome:
# node /tmp/uptime-auth-browser/node_modules/playwright/cli.js install chromium
# unset BROWSER_EXECUTABLE
AUTH_BROWSER_CHECK=1 go test -v ./internal/storage/postgres -run TestBrowserCrossSite -count=1
```

Результаты реализации, включая интеграционные и браузерные проверки,
записаны в [исходном плане](docs/plan/AUTH_PLAN.md) и
[плане cookie-контракта](docs/plan/COOKIE_AUTH_PLAN.md). Генерируемые сборки `bin/`, зависимости,
coverage, cookie-файлы и секреты не включайте в коммиты.
