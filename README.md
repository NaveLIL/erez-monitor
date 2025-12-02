# EREZMonitor

Легковесная утилита мониторинга системных ресурсов для Windows с игровым оверлеем.

## Возможности

- **CPU мониторинг**: общая нагрузка, нагрузка по ядрам
- **RAM мониторинг**: использование памяти (использовано/всего)
- **GPU мониторинг**: нагрузка GPU, температура, VRAM (поддержка AMD и NVIDIA через Windows PDH API)
- **Диск мониторинг**: скорость чтения/записи (MB/s), использование дисков
- **Сеть мониторинг**: входящий/исходящий трафик (KB/s, MB/s)
- **Пинг мониторинг**: задержка до популярных серверов (Cloudflare, Google, Steam EU, Riot EU)
- **Процессы**: топ процессов по CPU и памяти
- **Игровой оверлей**: полупрозрачное окно поверх игр с drag-and-drop позиционированием
- **Системный трей**: иконка с цветовой индикацией нагрузки
- **Окно настроек**: нативное Windows GUI для настройки всех параметров
- **Алерты**: всплывающие уведомления при превышении порогов (с поддержкой звука)
- **Логирование**: запись логов и экспорт метрик в CSV
- **Горячие клавиши**: глобальные комбинации клавиш
- **Автозагрузка**: запуск с Windows

## Требования

- Windows 10/11
- Go 1.21+ (для сборки из исходного кода)
- Дискретная видеокарта AMD или NVIDIA (опционально, для GPU мониторинга)

## Установка

### Из исходного кода

```powershell
# Клонировать репозиторий
git clone https://github.com/NaveLIL/erez-monitor.git
cd erez-monitor

# Установить зависимости и собрать
.\build.ps1

# Или для релизной сборки (без консольного окна)
.\build.ps1 -Release
```

### Готовый бинарник

Скачайте последний релиз из [Releases](https://github.com/NaveLIL/erez-monitor/releases).

## Использование

```powershell
# Обычный запуск
.\EREZMonitor.exe

# Режим отладки
.\EREZMonitor.exe --debug

# Запуск только в трей
.\EREZMonitor.exe --tray-only

# С пользовательским конфигом
.\EREZMonitor.exe --config path/to/config.yaml

# Показать версию
.\EREZMonitor.exe --version
```

## Настройка

Конфигурационный файл создается автоматически в `%APPDATA%\EREZMonitor\config.yaml`:

```yaml
monitoring:
  update_interval: 1s      # Интервал обновления метрик (минимум 100ms)
  history_duration: 60s    # Длительность хранения истории
  enable_gpu: true         # Включить GPU мониторинг
  enable_processes: true   # Включить мониторинг процессов
  top_process_count: 10    # Количество топ процессов (1-50)

alerts:
  enabled: true
  cpu_threshold: 80        # Порог CPU для алерта (%)
  ram_threshold: 85        # Порог RAM (%)
  gpu_threshold: 85        # Порог GPU (%)
  gpu_temp_threshold: 85   # Порог температуры GPU (C)
  disk_threshold: 90       # Порог заполнения диска (%)
  cooldown: 30s            # Минимальный интервал между алертами
  sound_enabled: true      # Звуковое уведомление

ui:
  tray_enabled: true
  autostart: false         # Запуск с Windows
  hotkey: "Ctrl+Shift+M"   # Горячая клавиша для показа деталей
  theme: "dark"            # Тема: dark или light
  language: "en"           # Язык интерфейса
  window_width: 800
  window_height: 600
  refresh_rate: 500ms      # Частота обновления UI

overlay:
  enabled: false           # Включить оверлей по умолчанию
  position: "top-right"    # Позиция: top-right, top-left, bottom-right, bottom-left, custom
  custom_x: 0              # X координата при position: custom
  custom_y: 0              # Y координата при position: custom
  opacity: 0.8             # Прозрачность (0.0 - 1.0)
  font_size: 16            # Размер шрифта (8-72)
  show_fps: true           # Показывать FPS
  show_cpu: true           # Показывать CPU
  show_ram: true           # Показывать RAM
  show_gpu: true           # Показывать GPU
  show_net: true           # Показывать сеть и пинг
  show_disk: true          # Показывать диск (только при активности)
  background_color: "#000000"  # Цвет фона
  text_color: "#FFFFFF"        # Цвет текста
  hotkey: "Ctrl+Shift+O"       # Горячая клавиша включения оверлея
  move_hotkey: "Ctrl+Shift+P"  # Горячая клавиша режима перемещения

logging:
  level: "info"            # Уровень: debug, info, warn, error
  to_file: true            # Записывать логи в файл
  file_path: "logs/erez-monitor.log"  # Путь к файлу логов
  csv_export: true         # Экспортировать метрики в CSV
  csv_path: "logs/metrics.csv"        # Путь к CSV файлу
  max_file_size: "10MB"    # Максимальный размер файла
  rotation: "daily"        # Ротация: daily, size, both
  max_age: 7               # Максимальный возраст логов (дни)
  max_backups: 5           # Количество резервных копий
```

## Горячие клавиши

| Комбинация | Действие |
|------------|----------|
| `Ctrl+Shift+M` | Показать детали в консоли |
| `Ctrl+Shift+O` | Включить/выключить оверлей |
| `Ctrl+Shift+P` | Режим перемещения оверлея (drag-and-drop) |

## Игровой оверлей

Оверлей отображает в реальном времени:
- **CPU** - нагрузка с цветовым индикатором (зеленый/желтый/оранжевый/красный)
- **RAM** - использование памяти в ГБ
- **GPU** - нагрузка, VRAM и температура
- **NET** - скорость загрузки/отдачи + пинг до серверов
- **DISK** - скорость чтения/записи (показывается только при активности)

### Перемещение оверлея
1. Нажмите `Ctrl+Shift+P` или выберите "Move Overlay" в меню трея
2. Перетащите оверлей в нужное место
3. Снова нажмите `Ctrl+Shift+P` чтобы зафиксировать позицию

Позиция сохраняется в конфиге автоматически.

## Иконка в трее

Цвет иконки меняется в зависимости от нагрузки:
-  **Зеленая**: CPU < 50%, RAM < 70%
-  **Желтая**: CPU 50-80%, RAM 70-85%
-  **Красная**: CPU > 80% или RAM > 85%

## Меню трея

- **Show Details** - показать детальную статистику
- **Toggle Overlay** - включить/выключить оверлей
- **Move Overlay** - режим перемещения оверлея
- **Settings** - открыть окно настроек
- **Export Logs** - экспортировать метрики в CSV (сохраняется в Документы)
- **Start with Windows** - включить/выключить автозагрузку
- **Exit** - выход

## Окно настроек

Нативное Windows окно с настройками:
- **Оверлей**: включение, позиция, прозрачность
- **Алерты**: включение, пороги CPU/RAM/GPU/Disk
- **Общие**: автозагрузка с Windows

## Пинг мониторинг

Автоматически пингуются серверы:
- **Cloudflare** (1.1.1.1)
- **Google** (8.8.8.8)
- **Steam EU** (155.133.248.34)
- **Riot EU** (185.40.64.65)

Отображается лучший (минимальный) пинг. Цветовая индикация:
-  < 30 ms
-  30-60 ms
-  60-100 ms
-  > 100 ms

## Разработка

### Структура проекта

```
erez-monitor/
 main.go                 # Точка входа, инициализация компонентов
 build.ps1               # PowerShell скрипт сборки
 Makefile                # Make targets
 config/
    config.go           # Менеджер конфигурации
    config.yaml         # Дефолтный конфиг (embedded)
 collector/
    collector.go        # Главный сборщик метрик
    cpu.go              # CPU метрики
    memory.go           # RAM метрики
    gpu.go              # GPU метрики (основной)
    gpu_pdh.go          # GPU через Windows PDH API
    gpu_d3dkmt.go       # GPU через D3DKMT API
    disk.go             # Диск I/O
    network.go          # Сетевые метрики
    ping.go             # Пинг до серверов
    fps.go              # FPS через DWM API
    processes.go        # Топ процессов
 storage/
    ringbuffer.go       # Кольцевой буфер для истории
    ringbuffer_test.go  # Тесты
 alerter/
    alerter.go          # Система алертов
 ui/
    tray.go             # Системный трей
    overlay.go          # Игровой оверлей (WinAPI)
    settings.go         # Окно настроек (WinAPI)
 logger/
    logger.go           # Логирование и CSV экспорт
 hotkeys/
    hotkeys.go          # Глобальные горячие клавиши
 autostart/
    autostart.go        # Автозагрузка Windows
 models/
    metrics.go          # Структуры данных
 utils/
     format.go           # Форматирование значений
     windows.go          # Windows API утилиты
```

### Запуск тестов

```powershell
go test -v ./...

# С покрытием
go test -v -cover ./...

# С race detector
go test -v -race ./...

# Только storage
go test -v ./storage/...
```

### Сборка

```powershell
# Debug сборка (с консолью)
.\build.ps1

# Release сборка (без консоли)
.\build.ps1 -Release

# С UPX сжатием
.\build.ps1 -Release -Compress

# Очистка и сборка
.\build.ps1 -Clean -Release

# Тесты + сборка
.\build.ps1 -Test -Release

# Запустить после сборки
.\build.ps1 -Run
```

### Встраивание ресурсов (иконка)

Для встраивания иконки в exe требуется [go-winres](https://github.com/tc-hib/go-winres):

```powershell
go install github.com/tc-hib/go-winres@latest
```

## Зависимости

| Библиотека | Назначение |
|------------|------------|
| [shirou/gopsutil](https://github.com/shirou/gopsutil) | Сбор системных метрик (CPU, RAM, Disk, Network) |
| [getlantern/systray](https://github.com/getlantern/systray) | Системный трей |
| [spf13/viper](https://github.com/spf13/viper) | Конфигурация (YAML) |
| [sirupsen/logrus](https://github.com/sirupsen/logrus) | Структурированное логирование |
| [natefinch/lumberjack](https://github.com/natefinch/lumberjack) | Ротация логов |
| [golang.org/x/sys/windows](https://pkg.go.dev/golang.org/x/sys/windows) | Windows API |

## Технические особенности

- **Lock-free метрики**: использование atomic операций для доступа к метрикам из оверлея
- **Параллельный сбор**: все метрики собираются параллельно с таймаутом 800ms
- **Non-blocking оверлей**: отдельный поток с WinAPI message loop
- **Graceful shutdown**: корректное завершение всех горутин с таймаутом
- **Embedded config**: дефолтный конфиг встроен в бинарник

## Лицензия

MIT License
