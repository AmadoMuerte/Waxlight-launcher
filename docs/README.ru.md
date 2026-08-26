<div align="center">
  <img src="./waxlight.png" alt="Waxlight Launcher" width="180">

# Waxlight Launcher

**Современный и лёгкий лаунчер для Vintage Story.**

[English](../README.md) · **Русский**

[![CI](https://github.com/AmadoMuerte/Waxlight-launcher/actions/workflows/ci.yml/badge.svg)](https://github.com/AmadoMuerte/Waxlight-launcher/actions/workflows/ci.yml)
[![Последний релиз](https://img.shields.io/github/v/release/AmadoMuerte/Waxlight-launcher)](https://github.com/AmadoMuerte/Waxlight-launcher/releases/latest)
[![Лицензия: GPL-3.0](https://img.shields.io/badge/license-GPL--3.0-blue.svg)](../LICENSE)
[![Поддержать разработку](https://img.shields.io/badge/Поддержать-разработку-8A2BE2)](https://hipolink.net/amadomuerte)

[Скачать](https://github.com/AmadoMuerte/Waxlight-launcher/releases/latest) · [Discord](https://discord.gg/CrRHvg9UVw) · [Политика конфиденциальности](PRIVACY.md) · [Code Signing Policy](CODE_SIGNING_POLICY.md) · [Issues](https://github.com/AmadoMuerte/Waxlight-launcher/issues) · [Поддержать](https://hipolink.net/amadomuerte)
</div>

Waxlight — независимый open-source лаунчер, который объединяет аккаунты Vintage Story, версии игры, изолированные сборки, моды, обновления и игровую статистику в одном приложении для **Windows и Linux**.

Проект поддерживается [AmadoMuerte](https://github.com/AmadoMuerte) при помощи участников сообщества. Waxlight не связан с разработчиками Vintage Story, не одобрен ими, не распространяет игру и не обходит её лицензирование.

## Возможности

- **Управление сборками** — изолированные сборки, клонирование, импорт существующих установок, импорт и экспорт `.waxlight`, обложки и отдельные параметры запуска.
- **Версии игры** — установка и одновременное хранение нескольких версий Vintage Story.
- **Моды** — браузер ModDB, локальные моды, выбор версий, управление зависимостями и политиками обновлений для каждой сборки.
- **Резервные копии и восстановление** — ручные и автоматические снимки, настраиваемое хранение и восстановление Last Known Good.
- **Аккаунты и клиенты** — несколько аккаунтов Vintage Story и поддержка Optimum.
- **Серверы и ссылки** — каталог публичных серверов, избранное и глубокие ссылки на страницы модов и серверов.
- **Активность** — статистика игрового времени, логи запуска, загрузки и история фоновых операций.
- **Новости и обновления** — официальные новости Vintage Story и обновления лаунчера через Stable и Prerelease с проверкой контрольных сумм.
- **Поддержка ОС** — нативные пакеты для Windows и Linux и переносимая папка данных лаунчера.
- **Переводы сообщества** — английский, русский, немецкий, французский, испанский, португальский и многие другие переводы сообщества.

## Скачивание

Последняя версия всегда доступна в [GitHub Releases](https://github.com/AmadoMuerte/Waxlight-launcher/releases/latest).

| Платформа | Пакет |
| --- | --- |
| Windows x64 | Установщик `.exe` или портативный `.zip` |
| Debian / Ubuntu x64 | `.deb` |
| Fedora / RPM x64 | `.rpm` |
| Другие Linux x64 | Портативный `.tar.gz` |

Каждый релиз содержит `SHA256SUMS` для проверки целостности файлов.

> В Windows ранние неподписанные сборки могут вызывать предупреждение Microsoft Defender SmartScreen. Скачивайте Waxlight только со страницы Releases этого репозитория.

## Первый запуск

1. Войдите в аккаунт в разделе **Аккаунты**.
2. Установите версию игры в **Версии игры**.
3. Создайте сборку в **Библиотеке** и выберите аккаунт и версию игры.
4. При необходимости установите моды через **Моды**.
5. Нажмите **Запустить**.

Для работы нужен действующий аккаунт Vintage Story с доступом к игре.

## Данные и конфиденциальность

Папки данных по умолчанию:

- Linux: `~/.config/waxlight/`
- Windows: `%AppData%\waxlight\`

Основную папку данных можно перенести через **Настройки → Папка данных**. Учётные данные аккаунтов остаются в системном хранилище учётных данных ОС.

Телеметрия необязательна и по умолчанию отключена для новых установок. Она включается только с явного согласия пользователя; при включении отправляет псевдонимный идентификатор установки, версию лаунчера, ОС, архитектуру и ограниченные числовые или разрешённые операционные данные. Её можно изменить или отключить в **Настройки → Конфиденциальность и телеметрия**. Полный актуальный список сетевых передач приведён в [Политике конфиденциальности](PRIVACY.md).

Подробнее: [SECURITY.md](SECURITY.md) и [документация по авторизации](authentication.md).

## Сборка из исходного кода

Требования: **Go 1.25+**, **Node.js 22+**, **Wails 2.11**, C-компилятор и необходимые [системные зависимости Wails](https://wails.io/docs/gettingstarted/installation/).

```bash
git clone https://github.com/AmadoMuerte/Waxlight-launcher.git
cd Waxlight-launcher
npm ci --include=dev --prefix frontend
go install github.com/wailsapp/wails/v2/cmd/wails@v2.11.0
cd cmd/waxlight
wails dev
```

Production-сборка из корня репозитория:

```bash
make wails-build
```

## Документация

Документация backend API генерируется из транспортного слоя Wails и GoDoc с помощью закреплённой зависимости WailsDoc:

```bash
make api-docs       # сгенерировать схему, Markdown и проверяемый API-инвентарь
make api-docs-dev   # сгенерировать документацию и запустить VitePress dev-сервер
make api-docs-build # сгенерировать и собрать production-сайт VitePress
```

Сгенерированные схема, Markdown и сборка VitePress намеренно не хранятся в Git. Чистый checkout воспроизводит их из `internal/transport/wails` и `wailsdoc.yaml`, а `docs/wails-api-inventory.json` остаётся проверяемым контрактом публичного API.

Контрибьюторы могут посмотреть актуальную документацию backend API на <https://docs.waxlight.by>.

## Участие в разработке

Приветствуется помощь кодом, переводами, тестированием, документацией, сообщениями об ошибках и конкретными предложениями новых функций.

Перед pull request прочитайте [CONTRIBUTING.md](CONTRIBUTING.md) и выполните:

```bash
make release-check
```

Об уязвимостях сообщайте по инструкции в [SECURITY.md](SECURITY.md), а не через публичный issue.

## Участники проекта

<a href="https://github.com/AmadoMuerte/Waxlight-launcher/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=AmadoMuerte/Waxlight-launcher" alt="Участники Waxlight Launcher">
</a>

## Поддержать разработку

Waxlight бесплатный и open source. Если проект вам полезен и вы хотите поддержать дальнейшую разработку:

[![Поддержать разработку](https://img.shields.io/badge/Поддержать-разработку-8A2BE2?style=for-the-badge)](https://hipolink.net/amadomuerte)

## Лицензия

Waxlight Launcher распространяется по лицензии [GNU General Public License v3.0](../LICENSE). Информация о сторонних компонентах находится в [NOTICE](../NOTICE).
