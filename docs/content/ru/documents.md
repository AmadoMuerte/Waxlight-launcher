---
title: Документы репозитория
description: Полный список важных документов проекта — политики, лицензия, технические и процессные документы.
order: 70
---

# Документы репозитория

Полный список важных документов проекта: политики, лицензия, технические и процессные документы.

## Политики

| Документ | Содержание |
| --- | --- |
| [PRIVACY.md](https://github.com/AmadoMuerte/Waxlight-launcher/blob/main/docs/PRIVACY.md) | Локальные данные, опциональная телеметрия (выключена по умолчанию), сторонние сервисы, контакты для запросов. |
| [SECURITY.md](https://github.com/AmadoMuerte/Waxlight-launcher/blob/main/docs/SECURITY.md) | Приватные отчёты об уязвимостях, область безопасности, модель угроз, хранение учётных данных, поддерживаемые версии. |
| [CODE_SIGNING_POLICY.md](https://github.com/AmadoMuerte/Waxlight-launcher/blob/main/docs/CODE_SIGNING_POLICY.md) | SignPath Foundation, роли и подтверждения, происхождение сборок, правила артефактов и метаданных. |

Те же политики в удобном для чтения виде: [Конфиденциальность](./policies/privacy.md), [Безопасность](./policies/security.md), [Подпись кода](./policies/code-signing.md).

## Лицензия и уведомления

| Документ | Содержание |
| --- | --- |
| [LICENSE](https://github.com/AmadoMuerte/Waxlight-launcher/blob/main/LICENSE) | GNU GPL v3.0: можно использовать, изучать, изменять и распространять на условиях GPLv3. |
| [NOTICE](https://github.com/AmadoMuerte/Waxlight-launcher/blob/main/NOTICE) | Авторские права (© 2026 AmadoMuerte), обязанности при распространении, уведомления о сторонних компонентах (go-keyring, dbus, wincred — MIT/permissive). |

## Для пользователей

| Документ | Содержание |
| --- | --- |
| [README.ru.md](https://github.com/AmadoMuerte/Waxlight-launcher/blob/main/docs/README.ru.md) | Обзор проекта, возможности, скачивание, первые шаги, сборка из исходников. |
| [AGENTS.md](https://github.com/AmadoMuerte/Waxlight-launcher/blob/main/AGENTS.md) | Правила проекта для агентов и контрибьюторов: workflow, структура слоёв, безопасность данных, ветвление (main/dev), требования к PR. |

## Технические документы

| Документ | Содержание |
| --- | --- |
| [authentication.md](https://github.com/AmadoMuerte/Waxlight-launcher/blob/main/docs/authentication.md) | Протокол аутентификации Vintage Story, сетевая граница, хранение сессий, транзакции, миграция legacy-секретов, жизненный цикл учётных данных при запуске игры. |
| [game-versions.md](https://github.com/AmadoMuerte/Waxlight-launcher/blob/main/docs/game-versions.md) | Официальная лента версий, выбор платформы, конвейер установки, MD5/SHA-256, известные ограничения. |
| [modpack.md](https://github.com/AmadoMuerte/Waxlight-launcher/blob/main/docs/modpack.md) | Библиотека анализа обновлений модов: контракт Catalog, статусы, выбор кандидата, совместимость, зависимости. |
| [operations-page.md](https://github.com/AmadoMuerte/Waxlight-launcher/blob/main/docs/operations-page.md) | Контракт страницы операций: статусы, отмена как откат, удаление и очистка истории, требуемые регрессионные тесты. |
| [windows-updater.md](https://github.com/AmadoMuerte/Waxlight-launcher/blob/main/docs/windows-updater.md) | Архитектура автообновления Windows: SHA256SUMS vs Authenticode, модель доверия, план подписи SignPath, режимы установки. |

## Для контрибьюторов

| Документ | Содержание |
| --- | --- |
| [CONTRIBUTING.md](https://github.com/AmadoMuerte/Waxlight-launcher/blob/main/docs/CONTRIBUTING.md) | Настройка окружения, обязательные проверки перед PR, локализация, архитектурные ожидания, workflow коммитов. |

Сводка для разработчиков — на странице [«Разработка»](./development.md).
