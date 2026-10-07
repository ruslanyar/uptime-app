# Backend

Для совместного локального запуска из корня репозитория выполните `npm run dev`.
Команда поднимает PostgreSQL, применяет миграции и запускает API с фронтендом;
настройки и постоянный локальный JWT-секрет готовятся автоматически.
Ctrl+C останавливает приложения и только самостоятельно запущенную PostgreSQL,
сохраняя данные. Подробности — в [README репозитория](../README.md#запуск-всего-окружения).
Ниже описан самостоятельный запуск этого проекта.


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
# Один раз создайте личный JWT-секрет (файл игнорируется Git):
(umask 077; printf 'JWT_SECRET=%s\n' "$(openssl rand -hex 32)" > .env.development.local)
docker compose up -d --wait postgres
go run ./cmd/migrate up
go run ./cmd/migrate status
go run ./cmd/api
```

API и мигратор автоматически читают `.env.development` из корня backend,
включая запуск из вложенных каталогов. `APP_ENV=test` выбирает `.env.test`;
поддерживаются только `development` (по умолчанию) и `test`.
Все настройки перечислены в `.env.example`; в `.env.development` находятся
локальные значения, в `.env.test` — отдельная одноразовая тестовая БД и тестовый ключ.
Общие файлы коммитируются, личные секреты записывайте только в `.env*.local`.
Compose читает настройки PostgreSQL из файлов соответствующей среды и их `.local`.
При изменении учётных данных измените также URL; существующий development volume
сохраняет учётные данные, с которыми был создан. Порты Compose — 5432 и 55432.
Требуется Compose 2.24+ для необязательных `env_file`.

Приоритет: переменные процесса → `.env.<среда>.local` → `.env.local`
(только development) → `.env.<среда>` → `.env`.
Отсутствующий файл выбранной среды или ошибка синтаксиса останавливают запуск.
Загрузка не изменяет окружение процесса. Используется `godotenv` v1.5.1.
API не применяет миграции автоматически.

Следующий пример описывает HTTPS-параметры, а не отдельную production-среду:

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

Имя: 2–50 символов после trim; email: trim + нижний регистр, корректный формат,
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

### Go integration tests с PostgreSQL

Требуются Go 1.27+ и работающий Docker с Compose 2.24+.
Из корня репозитория выполните:

```sh
cd backend
docker compose --profile test up -d --wait postgres-test
go test -v -count=1 ./internal/migrations ./internal/storage/postgres
```

Compose запускает изолированную PostgreSQL на порту 55432 и ждёт готовности.
Тесты автоматически читают `TEST_DATABASE_URL` из `backend/.env.test`;
экспортировать переменные, запускать API/frontend или применять миграции вручную
не требуется. Каждый тест создаёт собственную временную БД, применяет миграции
и удаляет БД после завершения. Development-база не используется.
Не запускайте одновременно frontend E2E: они управляют тем же `postgres-test`.

`-v` показывает отдельные проверки, `-count=1` отключает кеш результатов Go.
Успешный результат — `PASS` и `ok` для обоих пакетов. `TestBrowserCrossSite`
по умолчанию имеет `SKIP`: это отдельная браузерная проверка, описанная ниже.
Остальные интеграционные тесты должны выполняться без пропусков.
Если они сообщают `TEST_DATABASE_URL is not set`, проверьте `.env.test` и уберите
пустой `TEST_DATABASE_URL` из окружения процесса: он имеет приоритет над файлом.

После проверки остановите тестовую PostgreSQL (из `backend/`):

```sh
docker compose --profile test stop postgres-test
```

Тестовые данные хранятся в tmpfs; остановка тестового сервиса не затрагивает
volume базы разработки. Для собственной тестовой БД задайте URL в
`.env.test.local`; её пользователь должен иметь CREATEDB.

### Полная проверка backend

Из `backend/`, с запущенной тестовой PostgreSQL:

```sh
sh scripts/check-sqlc.sh  # требует sqlc версии из .sqlc-version
go test ./... -count=1
go vet ./...
go build -o bin/api ./cmd/api
# Дополнительная проверка гонок:
go test -race ./... -count=1
```

### Отдельная браузерная проверка

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
записаны в [исходном плане](../docs/backend/plan/AUTH_PLAN.md) и
[плане cookie-контракта](../docs/backend/plan/COOKIE_AUTH_PLAN.md). Генерируемые сборки `bin/`, зависимости,
coverage, cookie-файлы и секреты не включайте в коммиты.

## Изменение имени профиля

`POST /api/v1/auth/profile` принимает JSON `{ "name": "Новое имя" }` и возвращает
`{ "user": { "id": "…", "name": "Новое имя", "email": "…" } }`. Нужен действующий
access token (cookie или Bearer) и `X-CSRF-Protection: 1`. Пробелы по краям
удаляются; длина имени — 2–50 Unicode-символов. Другие поля отклоняются.
Имя сохраняется только для владельца access token; email и пароль не меняются.

## Аватары профиля

`POST /api/v1/auth/profile` принимает JSON `{name}` для изменения имени или
`multipart/form-data` с полями `name` и одним файлом `avatar` для сохранения имени
и аватара вместе. Требуются access-cookie (или Bearer JWT) и
`X-CSRF-Protection: 1`. Ответ: `{user:{id,name,email,avatar_url?}}`.
Путь `avatar_url` также возвращается в `/me`, login и refresh; у пользователя
без аватара поле отсутствует. Клиент добавляет к пути origin API.

Поддерживаются JPEG, PNG, WebP и GIF. Сервер проверяет содержимое декодированием,
а не расширение или MIME-заголовок. Лимит файла — **500 КБ (512 000 байт)**,
дополнительно ограничены размеры изображения: не более 16 миллионов пикселей.
SVG и повреждённые изображения отклоняются. Невалидный файл возвращает `400`,
тело multipart больше лимита файла плюс 16 КБ служебных данных — `413`.

Файлы сохраняются под случайными именами в `uploads/avatars` относительно
рабочего каталога API (по умолчанию `backend/uploads/avatars` при запуске из
`backend/`). Переменная `AVATAR_DIR` меняет этот путь. Папка `backend/uploads/`
исключена из Git; для собственного пути настройте соответствующее исключение.
Каталог должен быть доступен процессу API для записи, сохраняться между перезапусками
и резервироваться вместе с БД. Все экземпляры API должны использовать общий каталог.
Изображения доступны публично по `GET /api/v1/avatars/<имя>`; URL изменяется
при замене, прежний файл удаляется после сохранения профиля. Листинг каталога
и произвольные файлы не выдаются. Ошибка сохранения профиля удаляет новый файл.

Перед запуском обновлённого API примените миграцию `00002_add_user_avatar.sql`:
`go run ./cmd/migrate up`. Для декодирования WebP используется `golang.org/x/image`.
Проверки форматов, ограничений, загрузки, замены и восстановления пользователя
включены в обычные `go test ./...` и PostgreSQL integration tests.

Для удаления аватара отправьте JSON `{name, remove_avatar: true}` на
`POST /api/v1/auth/profile`. Имя и удаление сохраняются вместе. После успешного
обновления БД файл удаляется из локального каталога, `avatar_url` исчезает из
ответов. Повторное удаление допустимо. При ошибке обновления прежний аватар сохраняется.

## Создание точки мониторинга

`POST /api/v1/monitors` сохраняет URL сайта и интервал будущего опроса для
авторизованного пользователя. Сам опрос сайтов пока не реализован.
Требуются access-cookie или `Authorization: Bearer <JWT>` и
`X-CSRF-Protection: 1`; действуют существующие правила Origin/CORS и приоритета
Authorization над cookie.

Запрос: `{"url":"https://example.com","interval_seconds":300}`.
Принимается только JSON-объект без неизвестных полей и дополнительных JSON-значений;
лимит тела — 16 KiB. Интервал — целое число от 60 до 86 400 секунд включительно.
URL после удаления пробелов по краям должен содержать схему HTTP(S), hostname
и корректный необязательный порт; credentials и fragment запрещены.
Длина URL — не более 2 048 байт. Пути, query, localhost и IP-адреса разрешены.
Сервер не обращается к сайту и не проверяет его доступность.

Ответ `201`: `{"id":"<uuid>","url":"https://example.com","interval_seconds":300,"created_at":"<UTC RFC3339 timestamp>","updated_at":"<UTC RFC3339 timestamp>"}`
с `Cache-Control: no-store`. Владелец берётся из access token, а не из тела запроса.
Повторный точный URL того же пользователя возвращает `409 monitor_conflict`,
даже при другом интервале. URL сохраняется без нормализации регистра, пути или
порта; разные пользователи могут сохранять одинаковые URL.
Остальные ошибки используют `{error:{code,message}}`: `400 invalid_request`,
`401 unauthorized`, `403` Origin/CSRF и `500 internal_error`.

```sh
curl -i -b /tmp/uptime-cookies.txt \
  -H 'Content-Type: application/json' -H 'X-CSRF-Protection: 1' \
  -d '{"url":"https://example.com","interval_seconds":300}' \
  http://localhost:8080/api/v1/monitors
```

Перед запуском обновлённого API примените миграции до `00004_add_monitor_updated_at.sql` включительно командой
`go run ./cmd/migrate up`. Точки хранятся в PostgreSQL и удаляются вместе с
пользователем. Создание, ограничения, конкурентные дубликаты и HTTP-контракт
проверяются существующими Go unit/integration командами; нового инструментария нет.
План: [MONITOR_ENDPOINT_PLAN.md](../docs/backend/plan/MONITOR_ENDPOINT_PLAN.md).

`GET /api/v1/monitors` возвращает `{ "monitors": [...] }` с теми же полями точек,
только для текущего пользователя, от новых к старым (при одинаковом времени —
по ID по убыванию). Пустой список — `[]`. Нужен access token; CSRF-заголовок
для GET не требуется. Ответы имеют `Cache-Control: no-store`.

Поле `updated_at` обязательно в ответах создания и списка. При создании оно
равно `created_at`; для записей, существовавших до миграции 00004, оно заполнено
значением `created_at`. Оба времени возвращаются в UTC.
