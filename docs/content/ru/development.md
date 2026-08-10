---
title: Разработка
description: Сборка из исходников, архитектура, тесты и правила участия в проекте.
order: 80
---

# Разработка

Сборка из исходников, архитектура, тесты и правила участия в проекте.

## Стек

- **Бэкенд:** Go 1.25+, Wails 2.11, SQLite.
- **Фронтенд:** React 19, TypeScript (strict), Vite 8, Tailwind 4, TanStack Query v5, zustand v5, Radix UI, i18next.
- **Платформы:** Windows x64, Linux x64 (GTK3 + WebKitGTK 4.1).

## Сборка из исходников

Требования: Go 1.25+, Node.js 22+, Wails 2.11, C-компилятор и [платформенные зависимости Wails](https://wails.io/docs/gettingstarted/installation/).

```bash
git clone https://github.com/AmadoMuerte/Waxlight-launcher.git
cd Waxlight-launcher
npm ci --include=dev --prefix frontend
go install github.com/wailsapp/wails/v2/cmd/wails@v2.11.0
cd cmd/waxlight
wails dev
```

Продакшн-сборка из корня репозитория:

```bash
make wails-build
```

> [!NOTE] Только Wails-сборка
> `wails dev` запускается только из `cmd/waxlight`. Поддерживаемые десктопные артефакты собираются через `make wails-build`; голый `go build` без тегов Wails — неподдерживаемая GUI-сборка.

## Архитектура

Go-код разделён на слои:

| Слой | Назначение |
| --- | --- |
| `internal/domain` | Модели и ошибки |
| `internal/application` | Юзкейсы и порты |
| `internal/infrastructure` | Адаптеры: БД, загрузчик, credential store, файловая система, каталог модов и т.д. |
| `internal/presentation` | Wails-контроллеры и DTO |

`cmd/waxlight/main.go` запускает Wails и bootstrap; доменная и прикладная логика не зависят от Wails и React. Фронтенд организован по Feature-Sliced Design (`app/`, `pages/`, `features/`, `entities/`, `shared/`, `widgets/`); обращения к бэкенду — только через `frontend/src/shared/api`.

## Проверки и тесты

```bash
make test           # продакшн-сборка фронтенда (i18n + tsc), затем Go и frontend-тесты
make format         # gofmt + oxfmt
make lint           # статический анализ Go (Linux target) + oxlint
make vet            # go vet для текущей платформы
make security       # проверки запрещённых паттернов и уязвимостей
make release-check VERSION=X.Y.Z  # полная релизная валидация
```

Точечные Go-тесты: `go test ./path/to/package -run TestName`. Pre-commit hook запускает `make format-check lint` и блокирует коммит при ошибках.

## Ветвление и PR

- `main` — стабильная продакшн-ветка; `dev` — интеграционная.
- Рабочие ветки создаются от `dev` с префиксами `feat/`, `fix/`, `refactor/`, `chore/`, `docs/`, `test/`, `ci/`.
- Обычные PR целятся в `dev`; в `main` допустим только PR `dev → main` для релизной промоции.
- Прямые коммиты в `main` и `dev` запрещены; force-push запрещён.

## Правила безопасности для контрибьюторов

- Никогда не помещайте пароли, TOTP-коды, pre-login токены, ключи сессий и подписи в DTO, биндинги, логи, ошибки, фикстуры, URL, аргументы процессов, переменные окружения и экспорты.
- Продакшн-учётные данные — только нативное хранилище ОС; plaintext- и in-memory-fallback запрещены.
- Функции копирования/экспорта/диагностики инстансов обязаны удалять четыре аутентификационных свойства из `clientsettings.json`.
- Логирование — только через `internal/infrastructure/logging` (slog); stdlib `log` и вывод в stdout запрещены.

## Локализация

Каноническая локаль — `frontend/src/shared/i18n/locales/en.json`: переводятся только значения, ключи, интерполяции `{{...}}` и плюральные суффиксы сохраняются. Проверка: `npm run check:i18n --prefix frontend`. Новый язык требует регистрации в `languages.ts`, `i18n/index.ts` и бэкенд-валидации. Интерфейс уже доступен на 10 языках: English, Русский, Беларуская, Español, Français, Deutsch, Қазақша, Polski, Svenska, Português.

## Как помочь

Код, переводы, тестирование, документация, баг-репорты и сфокусированные предложения приветствуются. Перед PR прочитайте [CONTRIBUTING.md](https://github.com/AmadoMuerte/Waxlight-launcher/blob/main/docs/CONTRIBUTING.md). Об уязвимостях сообщайте приватно по [политике безопасности](./policies/security.md). Вопросы и обсуждения — в [Discord](https://discord.gg/CrRHvg9UVw).
