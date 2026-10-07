# Проверки backend

Все команды выполняются из `backend/`.

## Go integration tests с PostgreSQL

Требуются Go 1.27+ и работающий Docker с Compose 2.24+.
Команды:

```sh
docker compose --profile test up -d --wait postgres-test
go test -v -count=1 ./internal/migrations ./internal/storage/postgres
```

- Тесты читают `TEST_DATABASE_URL` из `.env.test` и создают временные БД;
  не применяйте миграции вручную и не используйте development-базу.
- Не запускайте одновременно frontend E2E: они управляют тем же `postgres-test`.
- Интеграционные тесты должны выполняться без пропусков. `TestBrowserCrossSite`
  пропускается без `AUTH_BROWSER_CHECK=1`; запускайте его отдельно при изменении cookies/CORS.
- При `TEST_DATABASE_URL is not set` проверьте `.env.test` и уберите пустую
  переменную из процесса: она переопределяет файл.
- Для собственной БД задайте URL в `.env.test.local`; пользователь должен иметь CREATEDB.

После проверки остановите тестовую PostgreSQL (из `backend/`):

```sh
docker compose --profile test stop postgres-test
```

## Полная проверка backend

Основные команды — в [README backend](../../backend/README.md#проверки).
Для установки закреплённой версии sqlc см. [миграции и SQL](migrations.md).
Дополнительно, с запущенной тестовой PostgreSQL, проверьте гонки:

```sh
go test -race ./... -count=1
```

## Отдельная браузерная проверка

Требуется запущенный `postgres-test` из раздела выше. Разрешайте сторонние cookies
и тестовые TLS-сертификаты только внутри тестового браузерного контекста.

```sh
npm install --prefix /tmp/uptime-auth-browser --no-save playwright@1.56.1
export PLAYWRIGHT_MODULE=/tmp/uptime-auth-browser/node_modules/playwright
# Если Chrome установлен, укажите его.
export BROWSER_EXECUTABLE=/usr/bin/google-chrome

# Альтернатива без системного Chrome:
# node /tmp/uptime-auth-browser/node_modules/playwright/cli.js install chromium
# unset BROWSER_EXECUTABLE
# При унаследованном HTTP-прокси исключите локальные тестовые домены:
export NO_PROXY=localhost,127.0.0.1,.auth-client.test,.auth-service.test
export no_proxy="$NO_PROXY"
AUTH_BROWSER_CHECK=1 go test -v ./internal/storage/postgres -run TestBrowserCrossSite -count=1
```
