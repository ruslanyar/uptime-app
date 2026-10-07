# Конфигурация

Переменные и примеры значений — в [`.env.example`](../../backend/.env.example).
Значения по умолчанию и валидация — в
[`config.go`](../../backend/internal/config/config.go);
порядок загрузки — в [`environment.go`](../../backend/internal/config/environment.go).

## Окружение

- Запускайте API и мигратор из `backend/` или вложенного каталога:
  загрузчик ищет корень по `go.mod`.
- `APP_ENV` допускает только `development` (по умолчанию) и `test`.
  Не задавайте `production`. Файл `.env.<среда>` обязателен даже при
  передаче всех значений через окружение процесса.
- Сохраняйте приоритет: процесс → `.env.<среда>.local` → `.env.local`
  (только development) → `.env.<среда>` → `.env`.
  Пустая переменная процесса тоже переопределяет файл.
- Личные значения и секреты записывайте в игнорируемые `.env*.local`;
  в общих файлах оставляйте только локальные development/test значения.
- Загрузчик не меняет окружение процесса: в коде используйте возвращённые
  настройки, не рассчитывайте получить загруженные значения через `os.Getenv`.
- Compose читает `.env.development` / `.env.test` и соответствующий `.local`;
  не рассчитывайте на поддержку `.env.local` для настройки контейнеров.
  Требуется Compose 2.24+.
- При смене PostgreSQL credentials согласуйте `POSTGRES_*` с URL подключения.
  Существующий development volume сохраняет прежние credentials;
  изменение env не переинициализирует БД.

## HTTPS

Для несвязанных frontend/API origins используйте HTTPS на обоих сайтах,
точный frontend origin в `ALLOWED_ORIGINS`, `COOKIE_SECURE=true`, `COOKIE_SAME_SITE=none`.
Не используйте wildcard и не рассчитывайте на автоматическое разрешение поддоменов:
схема и порт входят в origin. `none` несовместим с `COOKIE_SECURE=false`.

Завершайте TLS на reverse proxy перед API. Для production задавайте отдельные
credentials и PostgreSQL URL с защищённым соединением (например, `sslmode=verify-full`).
