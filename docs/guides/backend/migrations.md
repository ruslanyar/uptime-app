# Миграции и SQL

Все команды выполняются из `backend/`. API не применяет миграции:
выполняйте их отдельно перед запуском обновлённой версии.

```sh
go run ./cmd/migrate up      # до последней версии
go run ./cmd/migrate status  # current=N target=M
go run ./cmd/migrate down    # откат одной миграции; только development/test
```

- Не редактируйте применённые миграции; исправляйте схему новой миграцией.
  Конфликты номеров между ветками разрешайте до применения.
- Используйте следующий последовательный номер шириной пять цифр:
  `<номер>_<описание>.sql`. SQL применения и отката разделяйте строкой
  `---- create above / drop below ----`.
- Не используйте шаблоны, Go-миграции или отключение транзакций.
  Учитывайте потерю данных при `down`.
- Схему для sqlc берите из SQL применения в `migrations`;
  не создавайте отдельный `schema.sql`.

Установите sqlc версии из [`.sqlc-version`](../../../backend/.sqlc-version):

```sh
go install "github.com/sqlc-dev/sqlc/cmd/sqlc@v$(cat .sqlc-version)"
export PATH="$(go env GOPATH)/bin:$PATH"
sqlc compile
sqlc generate
sh scripts/check-sqlc.sh
```

После изменения SQL выполните генерацию, проверку и включите
`internal/storage/postgres/sqlc` в коммит. Генерируемый код вручную не редактируйте.
Версию Tern меняйте через `go.mod`, sqlc — через `.sqlc-version`;
не обновляйте генератор отдельно от проверки и сгенерированного пакета.
