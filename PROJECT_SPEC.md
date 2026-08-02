# PROJECT_SPEC.md

## 1. Общая информация
### 1.1 Название проекта

Официальное название продукта:

Waxlight Launcher

Краткое название:

Waxlight

Технические идентификаторы:

Executable: waxlight
Repository: waxlight-launcher
Application ID: com.waxlight.launcher

Application ID считается предварительным до проверки доступности домена, названия пакета и идентификаторов публикации.

### 1.1.1 Описание продукта

Waxlight Launcher — неофициальный кроссплатформенный лаунчер для Vintage Story, предназначенный для управления версиями игры, сборками, аккаунтами и модами.

Краткое описание:

A modern unofficial launcher for Vintage Story.

Расширенное описание:

Waxlight Launcher is a modern unofficial launcher for Vintage Story with support for multiple game versions, isolated instances, multiple accounts, mod management and playtime tracking.
### 1.1.2 Позиционирование

Waxlight должен сочетать:

простоту;
понятность;
удобство;
скорость;
современный внешний вид;
атмосферный и тёплый визуальный стиль.

По уровню удобства управления сборками продукт должен ориентироваться на подход Prism Launcher, но не копировать его:

название;
интерфейс;
структуру бренда;
логотип;
визуальные элементы.

Waxlight должен иметь собственную идентичность, основанную на образе свечного света.

### 1.1.3 Статус продукта

Waxlight Launcher является сторонним неофициальным проектом.

В интерфейсе, документации, репозитории и на сайте должно быть ясно указано:

Waxlight Launcher is not affiliated with or endorsed by the developers of Vintage Story.

Русский вариант:

Waxlight Launcher не связан с разработчиками Vintage Story и не является официальным лаунчером игры.
### 1.1.4 Основная идея бренда

Waxlight означает мягкий свет свечи.

Бренд должен передавать ощущение:

тёплого света в тёмном пространстве;
простого начала игры;
порядка среди множества сборок и модов;
спокойного и понятного инструмента;
уютной атмосферы без визуальной тяжести.

Главная продуктовая метафора:

Waxlight освещает путь от выбора сборки до запуска игры.

### 1.1.5 Возможные слоганы

Основной вариант:

Light your next world.

Дополнительные варианты:

Your worlds, clearly arranged.
A brighter way to launch.
Build. Mod. Play.
A warm light for every world.

Слоган не является обязательной частью логотипа и может быть изменён позднее.

### 1.2 Назначение документа

Этот документ описывает:

* функциональные требования;
* пользовательские сценарии;
* модели данных;
* поведение основных модулей;
* взаимодействие Go backend и React frontend;
* длительные операции;
* обработку ошибок;
* критерии готовности функций;
* границы MVP.

Архитектурные правила проекта описаны отдельно в `AGENTS.md`.

### 1.3 Основной стек

Backend:

* Go;
* Wails v2;
* SQLite.

Frontend:

* React;
* TypeScript;
* Vite.

Поддерживаемые платформы:

* Linux;
* Windows.

---

# 2. Цели продукта

Лаунчер должен позволять пользователю управлять Vintage Story из одного приложения.

Ключевые возможности:

1. авторизация в Vintage Story;
2. хранение нескольких аккаунтов;
3. переключение между аккаунтами;
4. установка нескольких версий игры;
5. создание независимых сборок;
6. использование разных наборов модов в разных сборках;
7. просмотр каталога модов;
8. установка и обновление модов;
9. запуск игры;
10. отслеживание игрового времени;
11. просмотр общей и детальной статистики;
12. работа на Linux и Windows.

---

# 3. Неосновные цели

На первом этапе проект не обязан поддерживать:

* macOS;
* мобильные платформы;
* облачную синхронизацию;
* социальную сеть;
* встроенный чат;
* серверную часть лаунчера;
* собственный CDN;
* собственный каталог модов;
* автоматическую синхронизацию сохранений;
* управление выделенными серверами Vintage Story;
* создание и редактирование самих модов;
* импорт сборок из других сторонних лаунчеров;
* публичную публикацию пользовательских сборок.

Эти возможности могут быть добавлены позже.

---

# 4. Основные понятия

## 4.1 Аккаунт

Аккаунт представляет авторизованного пользователя Vintage Story.

Аккаунт содержит:

* локальный идентификатор;
* удалённый идентификатор пользователя;
* отображаемое имя;
* данные авторизационной сессии;
* дату последней успешной авторизации;
* статус авторизации;
* признак аккаунта по умолчанию.

## 4.2 Версия игры

Версия игры представляет конкретный доступный релиз Vintage Story.

Версия может быть:

* доступна для загрузки;
* загружается;
* устанавливается;
* установлена;
* повреждена;
* удаляется;
* недоступна.

## 4.3 Сборка

Сборка, далее `Instance`, представляет независимую конфигурацию игры.

Каждая сборка имеет:

* собственное название;
* выбранную версию игры;
* собственный каталог данных;
* собственный набор модов;
* настройки запуска;
* аккаунт по умолчанию;
* статистику игрового времени;
* изображение или обложку;
* дату последнего запуска.

## 4.4 Мод

Мод представляет локально установленную или удалённо доступную модификацию Vintage Story.

Мод может иметь несколько релизов для разных версий игры.

## 4.5 Игровая сессия

Игровая сессия начинается после успешного запуска процесса игры и завершается после остановки процесса.

Игровая сессия используется для подсчёта игрового времени.

## 4.6 Операция

Операция представляет длительное действие:

* загрузка версии;
* установка версии;
* проверка версии;
* загрузка мода;
* установка мода;
* обновление мода;
* запуск игры;
* импорт сборки;
* экспорт сборки.

Операция имеет идентификатор, прогресс и статус.

---

# 5. Роли пользователей

В первой версии существует одна роль:

## Локальный пользователь лаунчера

Пользователь может:

* добавлять аккаунты;
* управлять сборками;
* устанавливать версии;
* управлять модами;
* запускать игру;
* просматривать статистику;
* изменять настройки.

Отдельная регистрация в самом лаунчере не требуется.

---

# 6. Основной пользовательский поток

Первый запуск:

1. пользователь запускает лаунчер;
2. лаунчер создаёт локальную структуру данных;
3. лаунчер открывает приветственный экран;
4. пользователь добавляет аккаунт;
5. пользователь создаёт первую сборку;
6. пользователь выбирает версию игры;
7. лаунчер предлагает установить выбранную версию;
8. версия загружается и устанавливается;
9. пользователь при необходимости устанавливает моды;
10. пользователь запускает игру;
11. лаунчер создаёт игровую сессию;
12. после завершения игры лаунчер сохраняет игровое время.

Повторный запуск:

1. пользователь открывает лаунчер;
2. лаунчер показывает библиотеку сборок;
3. пользователь выбирает сборку;
4. пользователь запускает игру;
5. лаунчер использует аккаунт и настройки сборки;
6. после завершения обновляется статистика.

---

# 7. Навигация приложения

Основные разделы:

```text
Библиотека
Каталог модов
Загрузки
Аккаунты
Статистика
Настройки
```

Дополнительные страницы:

```text
Создание сборки
Редактирование сборки
Страница сборки
Страница мода
Страница версии игры
Журнал запуска
Первоначальная настройка
```

---

# 8. Библиотека сборок

## 8.1 Экран библиотеки

Экран должен отображать все созданные сборки.

Для каждой сборки показываются:

* обложка;
* название;
* версия игры;
* число активных модов;
* общее игровое время;
* дата последнего запуска;
* выбранный аккаунт;
* статус;
* кнопка запуска;
* меню дополнительных действий.

Возможные статусы сборки:

```text
ready
preparing
running
updating
broken
missing_version
authentication_required
```

## 8.2 Пустое состояние

Если сборок нет, показывается:

* краткое описание;
* кнопка создания первой сборки;
* возможность импортировать сборку.

## 8.3 Действия со сборкой

Пользователь может:

* открыть;
* запустить;
* остановить игру, если поддерживается безопасная остановка;
* редактировать;
* клонировать;
* экспортировать;
* удалить;
* открыть директорию;
* открыть каталог модов.

## 8.4 Сортировка

Поддерживаемые варианты:

* по последнему запуску;
* по названию;
* по дате создания;
* по игровому времени.

## 8.5 Поиск

Поиск выполняется по названию сборки.

---

# 9. Создание сборки

## 9.1 Обязательные поля

* название сборки;
* версия игры.

## 9.2 Необязательные поля

* аккаунт по умолчанию;
* пользовательская директория;
* обложка;
* описание;
* дополнительные аргументы запуска.

## 9.3 Автоматические значения

Если пользователь не указал директорию, она создаётся автоматически.

Пример:

```text
instances/<instance-id>/
```

Если пользователь не выбрал аккаунт:

* используется глобальный аккаунт по умолчанию;
* если его нет, аккаунт выбирается перед запуском.

## 9.4 Проверки

Название:

* не должно быть пустым;
* должно содержать видимые символы;
* должно иметь ограниченную длину;
* не обязано быть уникальным.

Директория:

* не должна принадлежать другой сборке;
* должна быть доступна для записи;
* не должна находиться внутри системных директорий лаунчера, если это создаёт конфликт.

Версия:

* должна существовать в списке известных версий;
* может быть ещё не установлена.

## 9.5 Поведение после создания

После создания пользователь попадает на страницу сборки.

Если версия не установлена, интерфейс предлагает её установить.

---

# 10. Клонирование сборки

При клонировании пользователь выбирает:

* новое название;
* нужно ли копировать моды;
* нужно ли копировать настройки;
* нужно ли копировать пользовательские данные;
* версию игры.

По умолчанию:

* настройки копируются;
* список модов копируется;
* сохранения не копируются;
* игровая статистика не копируется.

Новая сборка получает:

* новый идентификатор;
* новую директорию;
* пустую историю игровых сессий;
* отдельные записи установленных модов.

---

# 11. Удаление сборки

Перед удалением требуется подтверждение.

Пользователь должен выбрать один из вариантов:

1. удалить только запись из лаунчера;
2. удалить запись и файлы сборки.

Перед удалением интерфейс показывает:

* название сборки;
* путь к директории;
* примерный размер;
* предупреждение о необратимости.

Сборку нельзя удалить во время активного процесса игры.

---

# 12. Страница сборки

## 12.1 Основные блоки

Страница содержит:

* заголовок;
* обложку;
* кнопку запуска;
* выбранный аккаунт;
* версию игры;
* число модов;
* игровое время;
* дату последнего запуска;
* текущий статус.

Вкладки:

```text
Обзор
Моды
Настройки
Логи
Статистика
```

## 12.2 Вкладка «Обзор»

Показывает:

* основную информацию;
* последние игровые сессии;
* последние установленные моды;
* наличие обновлений;
* возможные проблемы;
* быстрые действия.

## 12.3 Вкладка «Моды»

Показывает:

* установленные моды;
* состояние каждого мода;
* доступные обновления;
* совместимость;
* переключатель включения;
* кнопку удаления;
* переход на страницу мода.

## 12.4 Вкладка «Настройки»

Настройки:

* версия игры;
* аккаунт по умолчанию;
* рабочая директория;
* дополнительные аргументы;
* переменные окружения;
* настройки логирования;
* параметры окна игры, если они поддерживаются;
* поведение лаунчера после запуска игры.

## 12.5 Вкладка «Логи»

Показывает:

* последние логи запуска;
* stdout;
* stderr;
* код завершения;
* время запуска;
* продолжительность;
* возможность открыть файл лога;
* копирование лога.

## 12.6 Вкладка «Статистика»

Показывает:

* общее игровое время сборки;
* число запусков;
* среднюю длительность сессии;
* последнюю сессию;
* игровое время по дням;
* игровое время по аккаунтам.

---

# 13. Управление версиями игры

## 13.1 Список версий

Лаунчер должен получать список доступных версий из разрешённого источника.

Для версии отображаются:

* идентификатор;
* название;
* канал;
* дата выпуска;
* размер загрузки;
* статус установки;
* архитектура;
* поддерживаемая платформа.

Возможные каналы:

```text
stable
preview
unstable
unknown
```

Фактические названия каналов должны определяться внешним источником.

## 13.2 Установка версии

Пользователь выбирает версию и запускает установку.

Этапы:

1. проверка доступного места;
2. создание временной директории;
3. загрузка;
4. проверка целостности;
5. распаковка;
6. проверка структуры;
7. перенос в постоянную директорию;
8. регистрация версии в базе;
9. удаление временных файлов.

## 13.3 Атомарность установки

Незавершённая установка не должна считаться установленной версией.

До успешного завершения версия хранится во временной директории.

## 13.4 Проверка версии

Пользователь может запустить проверку файлов.

Результаты:

```text
valid
missing_files
modified_files
corrupted_files
unknown
```

## 13.5 Переустановка

При переустановке:

* пользовательские данные сборок не удаляются;
* текущая версия заменяется;
* перед заменой проверяется, не запущена ли игра;
* операция должна быть отменяемой до этапа финальной замены.

## 13.6 Удаление версии

Версию нельзя удалить, если:

* она используется запущенной сборкой;
* выполняется операция с этой версией.

Если версия используется существующими сборками, необходимо предупредить пользователя.

После удаления такие сборки получают статус:

```text
missing_version
```

---

# 14. Авторизация

## 14.1 Общие требования

Лаунчер должен использовать разрешённый механизм авторизации Vintage Story.

Точная реализация зависит от официально доступного протокола и должна быть подтверждена перед разработкой.

Запрещено:

* обходить защиту;
* сохранять пароль;
* логировать токены;
* отправлять данные на сторонние серверы проекта.

## 14.2 Добавление аккаунта

Поток:

1. пользователь нажимает «Добавить аккаунт»;
2. открывается форма или официальный процесс авторизации;
3. backend выполняет авторизацию;
4. backend получает данные сессии;
5. секретные данные сохраняются в `CredentialStore`;
6. публичные данные сохраняются в SQLite;
7. аккаунт появляется в списке.

## 14.3 Несколько аккаунтов

Лаунчер поддерживает несколько одновременно сохранённых аккаунтов.

Для каждого аккаунта хранится отдельная сессия.

## 14.4 Аккаунт по умолчанию

Может существовать:

* глобальный аккаунт по умолчанию;
* аккаунт по умолчанию для конкретной сборки.

Приоритет:

1. явно выбранный аккаунт перед запуском;
2. аккаунт сборки;
3. глобальный аккаунт;
4. запрос выбора аккаунта.

## 14.5 Истечение сессии

Если сессия истекла:

1. backend пытается её обновить;
2. если обновление невозможно, аккаунт получает статус `reauthentication_required`;
3. запуск блокируется;
4. пользователь получает предложение повторно войти.

## 14.6 Удаление аккаунта

При удалении:

* удаляется локальная запись;
* удаляются секреты;
* сборки сохраняются;
* ссылки на удалённый аккаунт очищаются;
* игровые сессии сохраняют историческую информацию.

---

# 15. Модель статусов аккаунта

```text
authenticated
refreshing
expired
reauthentication_required
offline
error
```

---

# 16. Каталог модов

## 16.1 Общие требования

Каталог получает данные из внешнего API модов Vintage Story либо другого разрешённого источника.

Frontend не обращается к внешнему API напрямую.

Все запросы выполняются через backend.

## 16.2 Список модов

Карточка мода содержит:

* название;
* краткое описание;
* автора;
* изображение;
* категории;
* количество загрузок;
* дату обновления;
* поддерживаемые версии игры;
* наличие установленной версии;
* наличие обновления.

## 16.3 Поиск

Поиск должен поддерживать:

* название;
* описание;
* автора;
* теги.

## 16.4 Фильтры

Минимальные фильтры:

* версия игры;
* категория;
* автор;
* только совместимые;
* только обновлённые;
* только установленные;
* только неустановленные.

## 16.5 Сортировка

Минимальные варианты:

```text
relevance
downloads
updated_desc
updated_asc
name_asc
name_desc
```

## 16.6 Пагинация

Backend должен возвращать:

* элементы;
* номер страницы;
* размер страницы;
* общее число элементов;
* наличие следующей страницы.

Frontend не должен вычислять число страниц по неполным данным.

## 16.7 Кеширование

Backend может кешировать:

* результаты поиска;
* страницы модов;
* изображения;
* информацию о релизах.

Кеш должен иметь срок жизни.

Пользователь должен иметь возможность обновить данные вручную.

---

# 17. Страница мода

Страница мода содержит:

* название;
* полное описание;
* автора;
* изображения;
* ссылки;
* категории;
* дату создания;
* дату обновления;
* список релизов;
* совместимость;
* зависимости;
* число загрузок;
* статус установки.

## 17.1 Выбор сборки

При установке пользователь выбирает одну или несколько сборок.

Для каждой выбранной сборки backend определяет подходящий релиз.

## 17.2 Выбор релиза

По умолчанию выбирается:

* последняя стабильная версия;
* совместимая с версией игры;
* совместимая с платформой, если это применимо.

Пользователь может вручную выбрать релиз.

## 17.3 Несовместимость

Если совместимый релиз отсутствует:

* установка по умолчанию блокируется;
* пользователь видит причину;
* принудительная установка допускается только как отдельное подтверждённое действие.

---

# 18. Установка модов

## 18.1 Установка из каталога

Этапы:

1. выбрать сборку;
2. выбрать релиз;
3. проверить совместимость;
4. проверить зависимости;
5. загрузить файл;
6. проверить файл;
7. поместить файл в директорию модов;
8. обновить локальную базу;
9. обновить интерфейс.

## 18.2 Установка локального файла

Пользователь выбирает файл.

Backend должен:

* проверить расширение;
* проверить, что файл существует;
* попытаться определить метаданные;
* проверить конфликт имён;
* скопировать файл;
* зарегистрировать мод.

Если метаданные определить невозможно, мод получает статус:

```text
unmanaged
```

## 18.3 Повторная установка

Если мод уже установлен:

* пользователь получает выбор обновить или заменить;
* предыдущая версия сохраняется до успешной установки новой;
* при ошибке выполняется откат.

## 18.4 Зависимости

Если API предоставляет зависимости, лаунчер должен:

* показать необходимые зависимости;
* предложить установить их автоматически;
* предупредить о конфликтах;
* не устанавливать необязательные зависимости без согласия.

---

# 19. Включение и отключение модов

Способ включения должен соответствовать фактическому механизму Vintage Story.

Допустимые реализации:

* перемещение в отдельную директорию;
* изменение расширения;
* изменение конфигурации;
* использование официального механизма игры.

Конкретная стратегия должна быть реализована через абстракцию.

```go
type ModStateManager interface {
    Enable(ctx context.Context, instanceID string, modID string) error
    Disable(ctx context.Context, instanceID string, modID string) error
}
```

Frontend не должен знать, как физически отключается мод.

---

# 20. Обновление модов

## 20.1 Проверка обновлений

Проверка может выполняться:

* вручную;
* при открытии сборки;
* периодически, если включено в настройках.

## 20.2 Условия обновления

Обновление доступно, если:

* найден более новый релиз;
* релиз совместим с версией игры;
* источник мода известен;
* мод не является локальным unmanaged-файлом.

## 20.3 Обновление

Во время обновления:

1. скачивается новая версия;
2. проверяется файл;
3. старая версия временно сохраняется;
4. новая версия устанавливается;
5. запись обновляется;
6. старая версия удаляется.

При ошибке должна быть восстановлена предыдущая версия.

---

# 21. Удаление модов

Перед удалением показывается подтверждение.

Backend должен удалить:

* файл мода;
* локальную запись;
* связанные кешированные данные, если они больше не нужны.

Если файл отсутствует, запись всё равно должна быть корректно обработана.

---

# 22. Конфликты модов

Минимальная версия может выявлять:

* одинаковые идентификаторы;
* одинаковые имена файлов;
* несовместимые версии;
* отсутствующие зависимости;
* дублирование одного мода.

Сложное автоматическое определение конфликтов не входит в MVP.

---

# 23. Запуск игры

## 23.1 Предварительные проверки

Перед запуском backend проверяет:

* сборка существует;
* сборка не запущена;
* версия установлена;
* executable существует;
* аккаунт выбран;
* авторизация действительна;
* рабочая директория существует;
* моды доступны;
* отсутствуют блокирующие операции.

## 23.2 Подготовка запуска

Backend формирует:

* путь к executable;
* аргументы;
* переменные окружения;
* рабочую директорию;
* пользовательские директории;
* данные аккаунта, если они требуются;
* конфигурацию логирования.

## 23.3 Запуск процесса

После успешного запуска:

* создаётся `PlaySession`;
* сборка получает статус `running`;
* frontend получает событие;
* stdout и stderr перенаправляются в лог;
* начинается учёт времени.

## 23.4 Ошибка запуска

Если процесс не был создан:

* игровая сессия не считается начатой;
* backend возвращает типизированную ошибку;
* сборка получает статус `error`;
* создаётся запись в журнале операций.

## 23.5 Завершение игры

После завершения:

* фиксируется время;
* сохраняется exit code;
* вычисляется продолжительность;
* обновляется статистика;
* сборка возвращается в `ready`;
* frontend получает событие.

## 23.6 Аварийное завершение

Сессия считается аварийной, если:

* exit code указывает на ошибку;
* процесс завершён сигналом;
* backend не смог корректно определить завершение;
* сессия была восстановлена как незавершённая.

---

# 24. Остановка игры

Лаунчер может предоставлять кнопку остановки.

Порядок:

1. попытка корректного завершения;
2. ожидание ограниченное время;
3. предложение принудительно завершить процесс;
4. принудительное завершение только после подтверждения.

Поведение должно учитывать Linux и Windows.

---

# 25. Игровое время

## 25.1 Общий принцип

Игровое время считается по фактическому времени существования процесса игры.

Frontend не является источником истины.

## 25.2 Значения

Должны вычисляться:

* игровое время сессии;
* игровое время сборки;
* игровое время аккаунта;
* игровое время версии;
* общее игровое время;
* средняя длительность сессии;
* число запусков.

## 25.3 Ограничения

Сессии короче установленного порога могут помечаться как технический запуск.

Рекомендуемое значение по умолчанию:

```text
30 секунд
```

Такие сессии сохраняются, но могут не учитываться в основной статистике.

## 25.4 Незавершённые сессии

При старте приложения backend ищет сессии без `ended_at`.

Для каждой сессии:

* проверяется существование процесса, если это возможно;
* если процесс отсутствует, сессия закрывается;
* сессия помечается восстановленной;
* продолжительность оценивается по последнему известному времени.

---

# 26. Экран статистики

Экран показывает:

* общее игровое время;
* число запусков;
* среднее время сессии;
* наиболее используемую сборку;
* наиболее используемый аккаунт;
* время по дням;
* время по неделям;
* время по месяцам;
* последние сессии.

Фильтры:

* период;
* сборка;
* аккаунт;
* версия игры.

---

# 27. Центр загрузок

## 27.1 Назначение

Центр загрузок показывает все длительные операции.

Типы:

```text
game_version_download
game_version_install
game_version_verify
mod_download
mod_install
mod_update
instance_import
instance_export
launcher_update
```

## 27.2 Поля операции

* идентификатор;
* тип;
* название;
* статус;
* прогресс;
* загружено байт;
* всего байт;
* скорость;
* оставшееся время;
* время создания;
* ошибка;
* возможность отмены;
* возможность повторить.

## 27.3 Статусы операции

```text
queued
preparing
running
paused
completed
cancelled
failed
```

`paused` используется только если операция реально поддерживает паузу.

## 27.4 Очередь

Лаунчер должен ограничивать количество одновременных загрузок.

Настройка по умолчанию:

```text
3 параллельные загрузки
```

Точное значение может быть изменено.

---

# 28. Настройки приложения

## 28.1 Общие настройки

* язык;
* тема;
* запуск при старте системы;
* поведение при закрытии окна;
* сворачивание в трей;
* подтверждение удаления;
* автоматическая проверка обновлений.

## 28.2 Настройки загрузок

* количество параллельных загрузок;
* ограничение скорости;
* директория кеша;
* автоматическое удаление временных файлов;
* повтор при сетевой ошибке.

## 28.3 Настройки игры

* глобальные аргументы запуска;
* глобальные переменные окружения;
* поведение лаунчера после запуска;
* показывать ли логи;
* минимальная длительность учитываемой сессии.

## 28.4 Настройки каталога модов

* версия игры по умолчанию;
* скрывать несовместимые моды;
* автоматически проверять обновления;
* показывать нестабильные релизы.

## 28.5 Настройки приватности

* локальная аналитика;
* журналирование;
* автоматическая очистка логов;
* хранение истории игровых сессий.

Проект не должен отправлять аналитику без явного согласия.

---

# 29. Импорт и экспорт сборок

## 29.1 Экспорт

Экспорт может включать:

* метаданные сборки;
* версию игры;
* список модов;
* версии модов;
* настройки запуска;
* обложку.

По умолчанию экспорт не включает:

* токены;
* данные аккаунтов;
* сохранения;
* установленную игру;
* кеш;
* логи.

## 29.2 Формат

Рекомендуется архив:

```text
.zip
```

Внутри:

```text
manifest.json
mods/
assets/
```

## 29.3 Импорт

При импорте backend:

1. проверяет архив;
2. защищается от path traversal;
3. читает manifest;
4. проверяет формат;
5. показывает предварительный просмотр;
6. предлагает имя и директорию;
7. определяет отсутствующую версию;
8. определяет отсутствующие моды;
9. создаёт сборку;
10. при необходимости запускает загрузки.

---

# 30. Модель данных

## 30.1 Account

```go
type Account struct {
    ID                 string
    RemoteUserID       string
    Username           string
    DisplayName        string
    AvatarURL          *string
    Status             AccountStatus
    IsDefault          bool
    LastAuthenticated  *time.Time
    CreatedAt          time.Time
    UpdatedAt          time.Time
}
```

Секреты не входят в модель SQLite.

## 30.2 GameVersion

```go
type GameVersion struct {
    ID             string
    Name           string
    Channel        string
    Platform       string
    Architecture   string
    DownloadURL    string
    DownloadSize   int64
    Checksum       *string
    ReleasedAt     *time.Time
}
```

## 30.3 InstalledGameVersion

```go
type InstalledGameVersion struct {
    VersionID       string
    InstallationDir string
    ExecutablePath  string
    Status          InstalledVersionStatus
    InstalledAt     time.Time
    VerifiedAt      *time.Time
    SizeBytes       int64
}
```

## 30.4 Instance

```go
type Instance struct {
    ID               string
    Name             string
    Description      string
    GameVersionID    string
    DefaultAccountID *string
    Directory        string
    CoverPath        *string
    Status           InstanceStatus
    LastPlayedAt     *time.Time
    CreatedAt        time.Time
    UpdatedAt        time.Time
}
```

## 30.5 InstanceSettings

```go
type InstanceSettings struct {
    InstanceID          string
    LaunchArguments     []string
    Environment         map[string]string
    CloseLauncherOnPlay bool
    MinimizeOnPlay      bool
}
```

## 30.6 RemoteMod

```go
type RemoteMod struct {
    ID                   string
    Slug                 string
    Name                 string
    Summary              string
    Description          string
    AuthorID             string
    AuthorName           string
    IconURL              *string
    Categories           []string
    Downloads            int64
    CreatedAt            *time.Time
    UpdatedAt            *time.Time
}
```

## 30.7 ModRelease

```go
type ModRelease struct {
    ID                   string
    ModID                string
    Version              string
    FileName             string
    DownloadURL          string
    FileSize             int64
    Checksum             *string
    SupportedGameVersions []string
    Dependencies         []ModDependency
    ReleasedAt           *time.Time
}
```

## 30.8 InstalledMod

```go
type InstalledMod struct {
    ID              string
    InstanceID      string
    RemoteModID     *string
    ReleaseID       *string
    Name            string
    Version         string
    FileName        string
    FilePath        string
    Enabled         bool
    Managed         bool
    Source          string
    InstalledAt     time.Time
    UpdatedAt       time.Time
}
```

## 30.9 PlaySession

```go
type PlaySession struct {
    ID          string
    InstanceID  string
    AccountID   *string
    VersionID   string
    ProcessID   *int
    StartedAt   time.Time
    EndedAt     *time.Time
    DurationSec int64
    ExitCode    *int
    Crashed     bool
    Recovered   bool
}
```

## 30.10 Operation

```go
type Operation struct {
    ID             string
    Type           OperationType
    ResourceID     *string
    Title          string
    Status         OperationStatus
    Progress       float64
    CurrentBytes   int64
    TotalBytes     int64
    BytesPerSecond int64
    ErrorCode      *string
    ErrorMessage   *string
    CreatedAt      time.Time
    StartedAt      *time.Time
    FinishedAt     *time.Time
}
```

---

# 31. Схема базы данных

Минимальные таблицы:

```text
accounts
game_versions
installed_game_versions
instances
instance_settings
installed_mods
play_sessions
operations
app_settings
schema_migrations
```

Возможные дополнительные таблицы:

```text
mod_catalog_cache
mod_release_cache
instance_events
launch_logs
download_chunks
```

---

# 32. Контракты Go ↔ React

## 32.1 Общие правила

Frontend работает только с DTO.

Все DTO должны:

* иметь JSON-теги;
* быть совместимыми с генерацией Wails bindings;
* не содержать `time.Duration`;
* использовать секунды или миллисекунды;
* использовать ISO 8601 для дат;
* не содержать Go-специфичные типы;
* не содержать секреты.

## 32.2 Формат дат

Даты передаются как строки:

```text
2026-08-02T18:52:00+03:00
```

## 32.3 Продолжительность

Продолжительность передаётся в секундах:

```text
durationSeconds
```

## 32.4 Размеры

Размеры передаются в байтах:

```text
sizeBytes
downloadedBytes
totalBytes
```

## 32.5 Прогресс

Прогресс:

```text
0.0 ... 1.0
```

Frontend отвечает за отображение процентов.

---

# 33. Wails-контроллеры

Предлагаемые контроллеры:

```text
AppController
AccountController
InstanceController
GameVersionController
ModCatalogController
ModManagerController
LaunchController
StatisticsController
OperationController
SettingsController
```

---

# 34. AccountController

## 34.1 Методы

```go
ListAccounts() ([]AccountDTO, error)

GetAccount(accountID string) (AccountDTO, error)

BeginAuthentication(
    request BeginAuthenticationRequest,
) (AuthenticationFlowDTO, error)

CompleteAuthentication(
    request CompleteAuthenticationRequest,
) (AccountDTO, error)

RefreshAccount(accountID string) (AccountDTO, error)

SetDefaultAccount(accountID string) error

RemoveAccount(accountID string) error

LogoutAccount(accountID string) error
```

## 34.2 AccountDTO

```go
type AccountDTO struct {
    ID                string  `json:"id"`
    Username          string  `json:"username"`
    DisplayName       string  `json:"displayName"`
    AvatarURL         *string `json:"avatarUrl,omitempty"`
    Status            string  `json:"status"`
    IsDefault         bool    `json:"isDefault"`
    LastAuthenticated *string `json:"lastAuthenticated,omitempty"`
}
```

---

# 35. InstanceController

## 35.1 Методы

```go
ListInstances() ([]InstanceSummaryDTO, error)

GetInstance(instanceID string) (InstanceDetailsDTO, error)

CreateInstance(
    request CreateInstanceRequest,
) (InstanceDetailsDTO, error)

UpdateInstance(
    request UpdateInstanceRequest,
) (InstanceDetailsDTO, error)

CloneInstance(
    request CloneInstanceRequest,
) (InstanceDetailsDTO, error)

DeleteInstance(
    request DeleteInstanceRequest,
) error

OpenInstanceDirectory(instanceID string) error

OpenInstanceModsDirectory(instanceID string) error

ExportInstance(
    request ExportInstanceRequest,
) (OperationDTO, error)

ImportInstance(
    request ImportInstanceRequest,
) (OperationDTO, error)
```

## 35.2 CreateInstanceRequest

```go
type CreateInstanceRequest struct {
    Name             string  `json:"name"`
    Description      string  `json:"description"`
    GameVersionID    string  `json:"gameVersionId"`
    DefaultAccountID *string `json:"defaultAccountId,omitempty"`
    Directory        *string `json:"directory,omitempty"`
    CoverPath        *string `json:"coverPath,omitempty"`
}
```

## 35.3 InstanceSummaryDTO

```go
type InstanceSummaryDTO struct {
    ID                 string  `json:"id"`
    Name               string  `json:"name"`
    Description        string  `json:"description"`
    GameVersionID      string  `json:"gameVersionId"`
    GameVersionName    string  `json:"gameVersionName"`
    DefaultAccountID   *string `json:"defaultAccountId,omitempty"`
    DefaultAccountName *string `json:"defaultAccountName,omitempty"`
    CoverURL           *string `json:"coverUrl,omitempty"`
    Status             string  `json:"status"`
    EnabledModCount    int     `json:"enabledModCount"`
    TotalModCount      int     `json:"totalModCount"`
    PlaytimeSeconds    int64   `json:"playtimeSeconds"`
    LastPlayedAt       *string `json:"lastPlayedAt,omitempty"`
}
```

---

# 36. GameVersionController

## 36.1 Методы

```go
ListAvailableVersions(
    request ListVersionsRequest,
) (GameVersionPageDTO, error)

ListInstalledVersions() ([]InstalledGameVersionDTO, error)

GetVersion(versionID string) (GameVersionDetailsDTO, error)

InstallVersion(versionID string) (OperationDTO, error)

VerifyVersion(versionID string) (OperationDTO, error)

RepairVersion(versionID string) (OperationDTO, error)

RemoveVersion(
    request RemoveGameVersionRequest,
) error
```

## 36.2 GameVersionDTO

```go
type GameVersionDTO struct {
    ID             string  `json:"id"`
    Name           string  `json:"name"`
    Channel        string  `json:"channel"`
    Platform       string  `json:"platform"`
    Architecture   string  `json:"architecture"`
    DownloadSize   int64   `json:"downloadSize"`
    ReleasedAt     *string `json:"releasedAt,omitempty"`
    Installed      bool    `json:"installed"`
    InstallStatus  *string `json:"installStatus,omitempty"`
}
```

---

# 37. ModCatalogController

## 37.1 Методы

```go
SearchMods(
    request SearchModsRequest,
) (ModSearchResultDTO, error)

GetMod(modID string) (ModDetailsDTO, error)

ListModReleases(
    request ListModReleasesRequest,
) ([]ModReleaseDTO, error)

RefreshModCatalog() error

GetCategories() ([]ModCategoryDTO, error)
```

## 37.2 SearchModsRequest

```go
type SearchModsRequest struct {
    Query             string   `json:"query"`
    GameVersionID     *string  `json:"gameVersionId,omitempty"`
    Categories        []string `json:"categories"`
    Author            *string  `json:"author,omitempty"`
    CompatibleOnly    bool     `json:"compatibleOnly"`
    InstalledOnly     bool     `json:"installedOnly"`
    Sort              string   `json:"sort"`
    Page              int      `json:"page"`
    PageSize          int      `json:"pageSize"`
}
```

## 37.3 ModSearchResultDTO

```go
type ModSearchResultDTO struct {
    Items      []ModCardDTO `json:"items"`
    Page       int          `json:"page"`
    PageSize   int          `json:"pageSize"`
    TotalItems int          `json:"totalItems"`
    TotalPages int          `json:"totalPages"`
    HasNext    bool         `json:"hasNext"`
}
```

---

# 38. ModManagerController

## 38.1 Методы

```go
ListInstalledMods(
    instanceID string,
) ([]InstalledModDTO, error)

InstallMod(
    request InstallModRequest,
) (OperationDTO, error)

InstallModFile(
    request InstallModFileRequest,
) (OperationDTO, error)

UpdateMod(
    request UpdateModRequest,
) (OperationDTO, error)

RemoveMod(
    request RemoveModRequest,
) error

SetModEnabled(
    request SetModEnabledRequest,
) (InstalledModDTO, error)

CheckModUpdates(
    instanceID string,
) ([]ModUpdateDTO, error)
```

## 38.2 InstallModRequest

```go
type InstallModRequest struct {
    InstanceIDs []string `json:"instanceIds"`
    ModID       string   `json:"modId"`
    ReleaseID   *string  `json:"releaseId,omitempty"`
    Force       bool     `json:"force"`
}
```

## 38.3 InstalledModDTO

```go
type InstalledModDTO struct {
    ID                string  `json:"id"`
    InstanceID        string  `json:"instanceId"`
    RemoteModID       *string `json:"remoteModId,omitempty"`
    Name              string  `json:"name"`
    Version           string  `json:"version"`
    AuthorName        *string `json:"authorName,omitempty"`
    FileName          string  `json:"fileName"`
    Enabled           bool    `json:"enabled"`
    Managed           bool    `json:"managed"`
    Compatible        *bool   `json:"compatible,omitempty"`
    UpdateAvailable   bool    `json:"updateAvailable"`
    AvailableVersion  *string `json:"availableVersion,omitempty"`
    InstalledAt       string  `json:"installedAt"`
}
```

---

# 39. LaunchController

## 39.1 Методы

```go
ValidateLaunch(
    request ValidateLaunchRequest,
) (LaunchValidationDTO, error)

LaunchInstance(
    request LaunchInstanceRequest,
) (LaunchResultDTO, error)

StopInstance(
    request StopInstanceRequest,
) error

ForceStopInstance(instanceID string) error

GetRunningInstances() ([]RunningInstanceDTO, error)

GetLaunchLog(
    request GetLaunchLogRequest,
) (LaunchLogDTO, error)
```

## 39.2 LaunchInstanceRequest

```go
type LaunchInstanceRequest struct {
    InstanceID string  `json:"instanceId"`
    AccountID  *string `json:"accountId,omitempty"`
}
```

## 39.3 LaunchValidationDTO

```go
type LaunchValidationDTO struct {
    Valid    bool                  `json:"valid"`
    Issues   []LaunchIssueDTO      `json:"issues"`
    Warnings []LaunchWarningDTO    `json:"warnings"`
}
```

## 39.4 LaunchResultDTO

```go
type LaunchResultDTO struct {
    InstanceID string `json:"instanceId"`
    SessionID  string `json:"sessionId"`
    ProcessID  int    `json:"processId"`
    StartedAt  string `json:"startedAt"`
}
```

---

# 40. StatisticsController

## 40.1 Методы

```go
GetOverviewStatistics(
    request StatisticsFilterRequest,
) (StatisticsOverviewDTO, error)

GetPlaytimeSeries(
    request PlaytimeSeriesRequest,
) (PlaytimeSeriesDTO, error)

ListPlaySessions(
    request ListPlaySessionsRequest,
) (PlaySessionPageDTO, error)

GetInstanceStatistics(
    instanceID string,
) (InstanceStatisticsDTO, error)

GetAccountStatistics(
    accountID string,
) (AccountStatisticsDTO, error)
```

## 40.2 StatisticsFilterRequest

```go
type StatisticsFilterRequest struct {
    DateFrom    *string `json:"dateFrom,omitempty"`
    DateTo      *string `json:"dateTo,omitempty"`
    InstanceID  *string `json:"instanceId,omitempty"`
    AccountID   *string `json:"accountId,omitempty"`
    VersionID   *string `json:"versionId,omitempty"`
}
```

---

# 41. OperationController

## 41.1 Методы

```go
ListOperations(
    request ListOperationsRequest,
) ([]OperationDTO, error)

GetOperation(operationID string) (OperationDTO, error)

CancelOperation(operationID string) error

RetryOperation(operationID string) (OperationDTO, error)

DeleteOperation(operationID string) error

ClearOperationHistory() (int64, error)
```

`DeleteOperation` и `ClearOperationHistory` удаляют только завершённые операции
со статусами `completed`, `failed` или `cancelled`. Активные операции `queued` и
`running` защищены на уровне persistence. Успешная явная отмена загрузки удаляет
временные файлы и саму запись операции, поэтому новая запись `cancelled` в
истории не создаётся.

## 41.2 OperationDTO

```go
type OperationDTO struct {
    ID              string  `json:"id"`
    Type            string  `json:"type"`
    ResourceID      *string `json:"resourceId,omitempty"`
    Title           string  `json:"title"`
    Status          string  `json:"status"`
    Progress        float64 `json:"progress"`
    CurrentBytes    int64   `json:"currentBytes"`
    TotalBytes      int64   `json:"totalBytes"`
    BytesPerSecond  int64   `json:"bytesPerSecond"`
    ErrorCode       *string `json:"errorCode,omitempty"`
    ErrorMessage    *string `json:"errorMessage,omitempty"`
    CanCancel       bool    `json:"canCancel"`
    CanRetry        bool    `json:"canRetry"`
    CreatedAt       string  `json:"createdAt"`
    StartedAt       *string `json:"startedAt,omitempty"`
    FinishedAt      *string `json:"finishedAt,omitempty"`
}
```

---

# 42. SettingsController

## 42.1 Методы

```go
GetSettings() (AppSettingsDTO, error)

UpdateSettings(
    request UpdateSettingsRequest,
) (AppSettingsDTO, error)

ResetSettings() (AppSettingsDTO, error)

SelectDirectory(
    request SelectDirectoryRequest,
) (*string, error)

SelectFile(
    request SelectFileRequest,
) (*string, error)

OpenDirectory(path string) error
```

---

# 43. События Wails

## 43.1 Общий формат

Все события должны иметь:

* название;
* версию payload;
* время;
* идентификатор ресурса.

## 43.2 События операций

```text
operation:created
operation:updated
operation:completed
operation:failed
operation:cancelled
```

Payload:

```go
type OperationEventDTO struct {
    Version   int          `json:"version"`
    Timestamp string       `json:"timestamp"`
    Operation OperationDTO `json:"operation"`
}
```

## 43.3 События сборки

```text
instance:created
instance:updated
instance:deleted
instance:status_changed
```

## 43.4 События игры

```text
game:starting
game:started
game:output
game:exited
game:crashed
```

## 43.5 События аккаунта

```text
account:added
account:updated
account:removed
account:authentication_required
```

## 43.6 События модов

```text
mod:installed
mod:updated
mod:removed
mod:enabled
mod:disabled
```

---

# 44. Передача игровых логов

Событие `game:output` не должно отправляться для каждой отдельной буквы или байта.

Логи следует:

* буферизовать;
* отправлять строками или небольшими пачками;
* ограничивать по частоте;
* хранить на диске;
* не держать всю историю в памяти frontend.

Пример:

```go
type GameOutputEventDTO struct {
    InstanceID string   `json:"instanceId"`
    SessionID  string   `json:"sessionId"`
    Stream     string   `json:"stream"`
    Lines      []string `json:"lines"`
    Timestamp  string   `json:"timestamp"`
}
```

---

# 45. Структура frontend API

Рекомендуемая структура:

```text
frontend/src/shared/api/
├── accounts.ts
├── instances.ts
├── gameVersions.ts
├── modCatalog.ts
├── mods.ts
├── launcher.ts
├── statistics.ts
├── operations.ts
├── settings.ts
└── errors.ts
```

Пример:

```ts
export const instancesApi = {
    list: () => InstanceController.ListInstances(),

    get: (instanceId: string) =>
        InstanceController.GetInstance(instanceId),

    create: (request: CreateInstanceRequest) =>
        InstanceController.CreateInstance(request),

    remove: (request: DeleteInstanceRequest) =>
        InstanceController.DeleteInstance(request),
}
```

React-компоненты не импортируют `wailsjs` напрямую.

---

# 46. Состояние frontend

## 46.1 Глобальное состояние

Допустимо хранить:

* тему;
* активный instance;
* активный аккаунт;
* состояние боковой панели;
* пользовательские UI-настройки.

## 46.2 Серверное состояние

Через query-слой:

* аккаунты;
* сборки;
* моды;
* версии;
* статистика;
* операции.

## 46.3 Событийное обновление

После события backend frontend должен:

* обновить соответствующий кеш;
* не выполнять полную перезагрузку приложения;
* не дублировать одну и ту же запись.

---

# 47. Ошибки

## 47.1 Формат ошибки

```go
type AppErrorDTO struct {
    Code      string            `json:"code"`
    Message   string            `json:"message"`
    Details   map[string]string `json:"details,omitempty"`
    Retryable bool              `json:"retryable"`
}
```

## 47.2 Основные коды

```text
VALIDATION_ERROR
ACCOUNT_NOT_FOUND
ACCOUNT_AUTHENTICATION_REQUIRED
AUTHENTICATION_FAILED
INSTANCE_NOT_FOUND
INSTANCE_ALREADY_RUNNING
INSTANCE_DIRECTORY_CONFLICT
GAME_VERSION_NOT_FOUND
GAME_VERSION_NOT_INSTALLED
GAME_VERSION_CORRUPTED
MOD_NOT_FOUND
MOD_RELEASE_NOT_FOUND
MOD_INCOMPATIBLE
MOD_DEPENDENCY_MISSING
MOD_ALREADY_INSTALLED
DOWNLOAD_FAILED
CHECKSUM_MISMATCH
ARCHIVE_INVALID
INSUFFICIENT_DISK_SPACE
FILE_PERMISSION_DENIED
NETWORK_UNAVAILABLE
OPERATION_NOT_FOUND
OPERATION_NOT_CANCELLABLE
PROCESS_START_FAILED
PROCESS_STOP_FAILED
DATABASE_ERROR
INTERNAL_ERROR
```

## 47.3 Отображение

Frontend должен показывать:

* понятное сообщение;
* рекомендуемое действие;
* кнопку повтора, если `retryable`;
* технические детали в раскрываемом блоке;
* код ошибки для поддержки.

---

# 48. Сетевое поведение

## 48.1 Тайм-ауты

Все HTTP-запросы должны иметь тайм-аут.

## 48.2 Повторы

Автоматический повтор допустим для:

* временных сетевых ошибок;
* HTTP 429;
* HTTP 5xx;
* прерванных загрузок.

Не повторять автоматически:

* ошибки авторизации;
* несовместимость;
* ошибки валидации;
* checksum mismatch без ограничения числа попыток.

## 48.3 Offline-режим

Без сети пользователь должен иметь возможность:

* открыть приложение;
* видеть локальные сборки;
* видеть установленные моды;
* просматривать локальную статистику;
* запускать игру, если авторизация и игра позволяют это.

Недоступны:

* каталог модов;
* новые загрузки;
* обновление аккаунта;
* получение новых версий.

---

# 49. Требования безопасности

Обязательно:

* использовать HTTPS;
* не хранить пароли;
* хранить токены в защищённом хранилище;
* не передавать токены во frontend;
* проверять checksum;
* защищать распаковку архивов;
* валидировать пользовательские пути;
* ограничивать операции разрешёнными директориями;
* не запускать команды из метаданных модов;
* маскировать секреты в логах;
* проверять права доступа;
* не доверять ответам внешнего API;
* использовать временные файлы для атомарных обновлений.

---

# 50. Требования к интерфейсу

## 50.1 Стиль

* минималистичный;
* современный;
* тёмная тема по умолчанию;
* аккуратная типографика;
* небольшое количество акцентных цветов;
* плавные, короткие анимации;
* единая система отступов;
* отсутствие перегруженных экранов.

## 50.2 Состояния

Каждый асинхронный блок должен иметь:

* loading;
* empty;
* success;
* error;
* stale;
* disabled.

## 50.3 Доступность

Минимальные требования:

* управление клавиатурой;
* видимый focus;
* подписи кнопок;
* нормальный контраст;
* недопустимость передачи смысла только цветом;
* корректные состояния disabled.

## 50.4 Производительность

* виртуализация длинных списков;
* lazy loading изображений;
* debounce поиска;
* отмена старых запросов;
* отсутствие полного rerender приложения при обновлении прогресса;
* ограничение частоты событий.

---

# 51. Поведение окна

Поддержать:

* изменение размера;
* минимальный размер;
* полноэкранный режим не обязателен;
* кастомный titlebar допустим;
* системные кнопки должны работать корректно;
* сохранение последнего размера и позиции окна;
* отдельное поведение закрытия во время игры.

Варианты закрытия:

```text
close
minimize_to_tray
ask
```

---

# 52. Трей

Поддержка системного трея желательна, но может не входить в первый MVP.

Действия:

* открыть лаунчер;
* запустить последнюю сборку;
* показать активные загрузки;
* выйти;
* показать статус запущенной игры.

---

# 53. Автообновление лаунчера

Автообновление не входит в самый ранний MVP, но архитектура должна учитывать его.

Требования:

* проверка версии;
* загрузка пакета;
* проверка подписи или checksum;
* запрос подтверждения;
* безопасная установка;
* восстановление при ошибке;
* отдельная реализация для Linux и Windows.

---

# 54. Логирование

## 54.1 Логи приложения

Хранить:

* запуск приложения;
* миграции;
* операции;
* ошибки API;
* ошибки файловой системы;
* запуск игры;
* завершение игры.

## 54.2 Ротация

Логи должны иметь:

* максимальный размер;
* ограниченное количество файлов;
* автоматическое удаление старых логов.

## 54.3 Режимы

```text
normal
verbose
debug
```

`debug` не должен быть включён по умолчанию.

---

# 55. Производительность

Целевые требования:

* запуск интерфейса без длительной блокировки;
* операции с базой не блокируют UI;
* загрузки выполняются в фоне;
* хеширование не блокирует основные обработчики;
* UI остаётся отзывчивым во время установки;
* каталог с тысячами модов прокручивается плавно;
* события прогресса ограничены разумной частотой.

Рекомендуемая частота прогресса:

```text
не чаще 5–10 событий в секунду на операцию
```

---

# 56. Надёжность

Лаунчер должен корректно переживать:

* потерю сети;
* недостаток места;
* закрытие во время загрузки;
* повреждённый архив;
* ошибку checksum;
* удаление файла вручную;
* удаление директории сборки;
* истечение токена;
* аварийное завершение игры;
* аварийное завершение самого лаунчера;
* недоступность каталога модов.

---

# 57. Миграции данных

При изменении схемы SQLite:

* создаётся новая миграция;
* миграции выполняются последовательно;
* миграции имеют версии;
* ошибки миграций блокируют основной запуск;
* перед опасной миграцией допустимо создать резервную копию.

---

# 58. Тестовые сценарии backend

Обязательные тесты:

1. создание сборки;
2. конфликт директорий;
3. клонирование сборки;
4. удаление сборки без файлов;
5. удаление сборки с файлами;
6. установка версии;
7. отмена загрузки;
8. checksum mismatch;
9. безопасная распаковка;
10. установка совместимого мода;
11. блокировка несовместимого мода;
12. обновление с откатом;
13. включение и отключение мода;
14. выбор аккаунта для запуска;
15. запуск без авторизации;
16. запуск без версии;
17. создание игровой сессии;
18. завершение игровой сессии;
19. восстановление незавершённой сессии;
20. расчёт общей статистики.

---

# 59. Тестовые сценарии frontend

Обязательные сценарии:

1. пустая библиотека;
2. создание сборки;
3. ошибка создания;
4. список сборок;
5. выбор аккаунта;
6. установка версии;
7. отображение прогресса;
8. отмена операции;
9. каталог модов;
10. поиск;
11. фильтры;
12. страница мода;
13. установка мода;
14. отображение несовместимости;
15. запуск сборки;
16. состояние запущенной игры;
17. ошибка запуска;
18. экран статистики;
19. offline-состояние;
20. повтор операции.

---

# 60. MVP

## 60.1 Входит в MVP

* Wails v2;
* React;
* TypeScript;
* Linux;
* Windows;
* SQLite;
* создание сборок;
* редактирование сборок;
* удаление сборок;
* установка нескольких версий игры;
* один общий загрузчик;
* добавление нескольких аккаунтов;
* выбор аккаунта;
* запуск игры;
* игровые сессии;
* общее игровое время;
* время по сборкам;
* локальный список модов;
* установка мода из файла;
* включение и отключение мода;
* удаление мода;
* базовый центр операций;
* базовые настройки;
* современный тёмный интерфейс.

## 60.2 Не входит в первый MVP

* полноценный онлайн-каталог модов;
* автоматические зависимости;
* автообновление лаунчера;
* экспорт и импорт;
* системный трей;
* облачная синхронизация;
* расширенные графики;
* автоматическое исправление конфликтов модов.

---

# 61. Этап после MVP

После стабильного MVP добавить:

1. API каталога модов;
2. поиск;
3. фильтры;
4. страницы модов;
5. установка из каталога;
6. обновления;
7. зависимости;
8. импорт и экспорт сборок;
9. автообновление;
10. расширенную статистику.

---

# 62. Definition of Done

Функция считается завершённой, если:

* выполнены функциональные требования;
* соблюдены границы архитектуры;
* добавлена обработка ошибок;
* добавлены необходимые тесты;
* нет дублирования существующей логики;
* длительная операция поддерживает context;
* frontend имеет loading, empty и error состояния;
* функция работает на Linux;
* функция работает на Windows либо имеет явную платформенную реализацию;
* нет секретов в логах;
* DTO типизированы;
* документация обновлена.

---

# 63. Открытые вопросы

До реализации необходимо подтвердить:

1. официальный механизм авторизации Vintage Story;
2. допустимый способ хранения сессии;
3. официальный источник списка версий;
4. формат игровых дистрибутивов;
5. механизм запуска на Linux;
6. механизм запуска на Windows;
7. структура пользовательских данных игры;
8. механизм изоляции сборок;
9. механизм включения и отключения модов;
10. API каталога модов;
11. правила совместимости модов;
12. формат метаданных модов;
13. возможность offline-запуска;
14. лицензионные ограничения на загрузку игры;
15. допустимость стороннего лаунчера согласно правилам Vintage Story.

До подтверждения этих пунктов агент не должен придумывать несуществующие API или обходные механизмы.

---

# 64. Правило реализации неизвестных интеграций

Если внешний механизм ещё не подтверждён:

* создать интерфейс;
* создать mock или stub;
* не жёстко связывать application layer с предположением;
* отметить реализацию как незавершённую;
* не использовать фиктивные production URL;
* не хранить секреты в коде;
* не обходить официальные ограничения.

Пример:

```go
type AuthenticationProvider interface {
    BeginAuthentication(
        ctx context.Context,
        request BeginAuthenticationCommand,
    ) (AuthenticationFlow, error)

    CompleteAuthentication(
        ctx context.Context,
        request CompleteAuthenticationCommand,
    ) (AuthenticatedAccount, error)

    RefreshSession(
        ctx context.Context,
        accountID string,
    ) (AuthenticatedAccount, error)
}
```

---

# 65. Главный принцип продукта

Лаунчер должен быть удобным интерфейсом над независимым Go-ядром.

Пользователь должен иметь возможность:

* создать сборку;
* выбрать версию;
* выбрать аккаунт;
* установить моды;
* нажать «Играть»;

без необходимости вручную управлять директориями, архивами и конфигурационными файлами.

Сложность внутренней реализации не должна переноситься в интерфейс.

---

# 66. Реализованный протокол авторизации Vintage Story

Waxlight изолирует неофициально документированный протокол Vintage Story в
`internal/auth`. Первичный вход и продолжение с TOTP используют `POST
https://auth3.vintagestory.at/v2/gamelogin` с form-urlencoded параметрами.
Проверка сессии использует `POST
https://auth3.vintagestory.at/clientvalidate` с form-urlencoded `uid` и
`sessionkey`; этот формат подтверждён безопасной проверкой реального endpoint с
фиктивными значениями 2 августа 2026 года и зафиксирован unit-тестом HTTP
контракта. API остаётся публично не документированным и может измениться.

Пароли, TOTP-коды и `prelogintoken` не сохраняются и не передаются во frontend.
Постоянные session key/signature отделены от SQLite metadata и временно хранятся
в атомарно записываемом `account-secrets.json` с правами `0600` на POSIX.
Системные Secret Service и Windows Credential Manager остаются обязательной
последующей миграцией перед полной защитой секретов at rest.

Перед запуском Waxlight разрешает аккаунт в порядке: явный выбор, аккаунт
сборки, глобально выбранный аккаунт. Выбранная сессия валидируется, после чего
только четыре поля авторизации точечно и атомарно обновляются в
`clientsettings.json`. Истёкшая сессия и ошибка patching блокируют запуск;
сетевая ошибка не помечает сессию истёкшей. Подробности и границы проверки
описаны в `docs/authentication.md`.

---

# 67. Реализованный источник и установка версий Vintage Story

Waxlight получает список релизов из официального endpoint:

```text
GET https://api.vintagestory.at/stable-unstable.json
```

Контракт проверен по реальному ответу 2 августа 2026 года. Endpoint возвращает
объект, ключами которого являются версии игры. Для каждой версии опубликованы
платформенные дистрибутивы, имя файла, отображаемый размер, MD5, официальный CDN
URL и признак `latest`. Источник не предоставляет дату выпуска; Waxlight не
подставляет вымышленное значение.

Для MVP поддерживаются опубликованные клиентские пакеты:

* `linux` для Linux x64 в формате `tar.gz`;
* `windows` для Windows x64 в формате Inno Setup `exe`.

Windows-дистрибутив версии 1.22.6 проверен по сигнатуре и использует Inno Setup
6.4.3. Установка выполняется отдельной Windows-реализацией с документированными
параметрами Inno Setup, без shell-интерпретации аргументов. Linux использует
защищённую распаковку архива.

Разрешены только HTTPS URL официального хоста `cdn.vintagestory.at`. Имя файла
обязано совпадать с basename URL. Загрузка поддерживает `.partial`, HTTP Range,
прогресс, скорость, отмену, общую очередь до трёх одновременных передач и
проверку официального MD5. Регистрация версии в SQLite происходит только после
успешной установки и обнаружения исполняемого файла.

MD5 используется исключительно как доступная в официальном feed проверка
соответствия файла опубликованным метаданным и не считается криптографической
подписью. При ручном импорте локального архива остаётся доступна SHA-256.

Детали реализации и актуальные ограничения описаны в
`docs/game-versions.md`.
