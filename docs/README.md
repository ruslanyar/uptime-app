# Документация

Начните с [README репозитория](../README.md) для совместного запуска,
[README backend](../backend/README.md) для API и
[README frontend](../frontend/README.md) для браузерного клиента.
Инструкции для агентов — в [AGENTS.md](../AGENTS.md) и файлах AGENTS.md проектов.

## Структура

```text
docs/
├── guides/       # как выполнить задачу
├── rules/        # действующие правила и контракты
├── templates/    # заготовки документов
├── decisions/    # архитектурные решения (ADR), в Git
└── tmp/          # локальные рабочие файлы, вне Git
    ├── plans/    # планы, разработанные в Plan mode
    └── tasks/    # файлы задач
```

Документы группируются по назначению; внутри разделов допустимы каталоги
`backend/` и `frontend/`. Правила выбора раздела и обновления документации —
в [rules/documentation.md](rules/documentation.md).
Каталоги `tmp/` создавайте локально по необходимости.

## Гайды

- [Миграции и генерация SQL](guides/backend/migrations.md).
- [Проверки backend и браузерная проверка cookies/CORS](guides/backend/testing.md).
- Запуск и проверки frontend — в [README frontend](../frontend/README.md).

## Правила и контракты

- [Документация](rules/documentation.md).
- [Комментарии в коде](rules/code-comments.md).
- [GitHub Flow](rules/github-flow.md).
- [Коммиты и pull requests](rules/commits-pr.md).
- [Размещение изменений backend](rules/backend/architecture.md).
- [Конфигурация backend](rules/backend/configuration.md).
- [Авторизация](rules/backend/authentication.md).
- [Профиль и аватары](rules/backend/profile.md).
- [Мониторы](rules/backend/monitors.md).
- HTTP-контракт — в [README backend](../backend/README.md#http-контракт).
- Правила клиента и UI — в [README frontend](../frontend/README.md#правила-изменений).

## Шаблоны

- [Задача](templates/task.md) → локальный файл в `tmp/tasks/`.
- [План](templates/plan.md) → локальный файл в `tmp/plans/`, только для Plan mode.
- [Архитектурное решение](templates/adr.md) → документ в `decisions/`.

## Решения

Порядок ведения ADR — в [decisions/README.md](decisions/README.md).
Текущие ограничения ищите в правилах; ADR объясняют контекст и последствия решения.
