# Размещение изменений

- HTTP-разбор и ответы меняйте в `internal/controllers/auth` или
  `internal/controllers/monitor`; бизнес-правила — в `internal/auth` и `internal/monitor`.
- Маршруты и общую защиту CORS/CSRF меняйте в `internal/httpapi`.
- Подключение зависимостей и жизненный цикл API меняйте в `internal/app`;
  `cmd/api` оставляйте точкой входа.
- Проверку, хранение и выдачу файлов меняйте в `internal/avatar`.
- SQL меняйте в `internal/storage/postgres/queries`, схему — новыми файлами
  в `migrations`. Не редактируйте `internal/storage/postgres/sqlc` вручную;
  см. [миграции и генерацию](migrations.md).
- Для PostgreSQL-тестов используйте `internal/testdb`, не development-базу;
  см. [проверки](testing.md).
