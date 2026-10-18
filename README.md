# prodi

A productivity dashboard built as a monorepo with three parts that share one single source of
truth:

| Part | Stack | Location |
| --- | --- | --- |
| Web dashboard | React 19, Vite 6, Zustand 5, react-rnd | `apps/web` |
| REST + WebSocket API | Go 1.23, chi, SQLite | `apps/api` |
| ESP32 firmware | ESP32, SSD1306 OLED, PlatformIO | `firmware/esp32-display` |

The whole app works in English and the code base carries no comments.

---

## The idea

You keep a to-do list where each task carries an **estimate of how many focus blocks it needs**.
Running the pomodoro timer credits every completed focus block to the task that is currently
active. When a task reaches its estimate it is automatically marked as done, a notification fires,
and the timer moves on to the next pending task.

The same state is mirrored on a small OLED display wired to an ESP32, which can also drive the
timer with three physical buttons.

Because the API owns the clock, the browser and the ESP32 never disagree: whoever connects last
sees the same countdown, and a control pressed on the device immediately moves the dashboard.

---

## Architecture

```
┌──────────────────────┐         ┌──────────────────────┐         ┌──────────────────────┐
│  apps/web            │  REST   │  apps/api            │  WS     │  firmware            │
│  React + Zustand     │◄───────►│  chi + timer engine  │◄───────►│  ESP32 + SSD1306     │
│  floating windows    │  /api   │  SQLite (embedded)   │  /ws    │  3 buttons           │
└──────────────────────┘         └───────────┬──────────┘         └──────────────────────┘
                                               │
                                    one authoritative timer
```

**The API is the single source of truth.** It keeps the pomodoro state machine, ticks once per
second and broadcasts the result to every connected client. The web app and the firmware are both
pure views that can send commands back.

### Repository layout

```
prodi/
├── Makefile                        Orchestration for every target
├── package.json                    npm workspaces root
├── .editorconfig                   Formatting rules for Go, TS, CSS and C++
│
├── apps/
│   ├── api/                        Go API
│   │   ├── cmd/prodi-api/main.go   Entry point and wiring
│   │   └── internal/
│   │       ├── config/             Environment configuration
│   │       ├── domain/             Task, TimerState, Settings, Session, Snapshot
│   │       ├── store/              SQLite, migrations and repositories
│   │       ├── timer/              Authoritative pomodoro engine
│   │       ├── hub/                WebSocket registry and broadcast
│   │       ├── httpapi/            Router, handlers, middleware, WS endpoint
│   │       └── protocol/           Wire format shared by server and clients
│   └── web/                        React dashboard
│       └── src/
│           ├── api/                REST client and realtime client
│           ├── components/
│           │   ├── common/         Button, Icon, Progress, Stepper, Switch
│           │   ├── desktop/        MenuBar, Launcher, NotificationStack
│           │   ├── pomodoro/       TimerRing, PomodoroWindow
│           │   ├── stats/          StatsWindow
│           │   ├── todo/           TodoWindow, TaskItem
│           │   └── window/         WindowFrame, WindowLayer
│           ├── hooks/              Hotkeys, clock helpers
│           ├── store/              Zustand stores (window layout, app state)
│           ├── styles/             Design tokens and per-area stylesheets
│           └── types/              Shared TypeScript types
│
└── firmware/esp32-display/
    ├── platformio.ini
    ├── include/secrets.example.h   Template, copy to secrets.h
    └── src/
        ├── config.h                Pins, timings
        ├── main.cpp                Setup and loop
        ├── display/oled.*          U8g2 rendering
        ├── input/buttons.*         Debounced button handling
        ├── net/ws_client.*         WebSocket client and JSON parsing
        └── state/model.*           Device-side timer view
```

---

## Prerequisites

| Tool | Version used | Notes |
| --- | --- | --- |
| Node.js | 22.11 | npm 10.9 |
| Go | 1.23.4 | Not required to be on `PATH`, the Makefile auto-detects `C:\Program Files\Go\bin` |
| Python | 3.14 | Only used to install PlatformIO |
| GNU Make | 4.3 | Chocolatey or MSYS2 |
| PlatformIO Core | 6.2.0 | Installed by `make install-firmware` |

> **Go module pins.** `modernc.org/sqlite` is pinned to `v1.34.5` on purpose. It is a pure-Go,
> CGO-free SQLite, and it is the newest release that still builds with Go 1.23. Versions from
> `v1.60` onwards require Go 1.26 and would force a second toolchain download. The API is built
> with `CGO_ENABLED=0`, so the result is a single static binary.

> **Windows shells.** The Makefile points `SHELL` at Git Bash (`C:/Program Files/Git/bin/sh.exe`),
> which keeps the recipes identical on Windows and Linux.

---

## Getting started

```bash
git clone <your-repo-url> prodi
cd prodi

make install          # Go modules, npm dependencies and PlatformIO Core
```

`make install-firmware` installs PlatformIO with `pip install --user`. On Windows the scripts land
in `%APPDATA%\Python\Python3XX\Scripts`, which is usually not on `PATH`; the Makefile finds it
automatically.

### Configure the firmware

```bash
cp firmware/esp32-display/include/secrets.example.h firmware/esp32-display/include/secrets.h
```

Then edit `secrets.h`:

| Define | Meaning |
| --- | --- |
| `WIFI_SSID` | 2.4 GHz network name |
| `WIFI_PASSWORD` | Network password |
| `WS_HOST` | LAN IP of the machine running the API |
| `WS_PORT` | API port, `8080` by default |
| `WS_PATH` | `/ws?kind=device&name=esp32-display` |
| `WS_TOKEN` | Optional, must match `PRODI_DEVICE_TOKEN` on the API |

`secrets.h` is git-ignored and never committed.

---

## Running

```bash
make dev
```

This starts both dev servers and waits for them:

| Service | URL |
| --- | --- |
| Web dashboard | http://localhost:5173 |
| API | http://127.0.0.1:8080 |
| API health | http://127.0.0.1:8080/api/health |

The Vite dev server proxies `/api` and `/ws` to the Go API, so the browser only ever talks to one
origin and there is nothing to configure in the frontend.

To run the parts separately:

```bash
make dev-api
make dev-web
```

---

## The dashboard

The desktop is a macOS dark-mode workspace with draggable, resizable floating windows.

**Opening windows.** Use the launcher (the Dock at the bottom), the menu bar, or the hotkeys
below. Any window type can be opened more than once; new instances cascade. The layout, including
position and size, is persisted in `localStorage` and restored on reload.

**Window behaviour**

- Drag from the title bar, resize from all eight edges and corners
- Click anywhere in a window to bring it to the front, the focused window gets a deeper shadow
- Double-click the title bar, or click the green traffic light, to maximize and restore
- Red and yellow traffic lights close the window
- The chevron on the title bar collapses To Do and Insights down to a compact view
- Windows snap back inside the viewport, they can never be dragged out of reach

**Visual style.** Colour and spacing come from a single token sheet in `src/styles/tokens.css`:
translucent vibrancy backgrounds with `backdrop-filter`, traffic-light chrome, SF Pro typography
with sensible fallbacks, layered shadows and rounded corners. The wallpaper is a layered radial
gradient with a noise grain and a vignette, so no image assets are needed.

**Windows available**

| Window | Contents |
| --- | --- |
| **To Do** | Filter, add, rename, reorder-free estimate stepping, complete, delete, set as active |
| **Pomodoro** | Animated timer ring, play/pause/reset/skip, cycle dots, active task progress, inline settings |
| **Insights** | Total focus time, block count, day streak, completion rate, recent session history |

### Keyboard shortcuts

| Shortcut | Action |
| --- | --- |
| `Space` | Start or pause the timer |
| `Cmd/Ctrl + T` | Open a To Do window |
| `Cmd/Ctrl + P` | Open a Pomodoro window |
| `Cmd/Ctrl + S` | Open an Insights window |
| `Cmd/Ctrl + W` | Close the focused window |
| `Cmd/Ctrl + \`` | Close every window |
| `Alt + T` / `P` / `S` | Close that window type |
| `Shift + Enter` | Maximize the focused window |
| `Esc` | Minimize the focused window |
| `R` | Reset the current phase |
| `N` | Skip to the next phase |

Shortcuts are ignored while a text field has focus.

---

## The API

Base URL `http://127.0.0.1:8080`. All bodies and responses are JSON.

### Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `PRODI_ADDR` | `:8080` | Listen address |
| `PRODI_DB` | `apps/api/data/prodi.db` | SQLite file, `:memory:` is accepted for tests |
| `PRODI_CORS_ORIGINS` | Vite dev origins | Comma separated allow list |
| `PRODI_WS_PATH` | `/ws` | WebSocket endpoint path |
| `PRODI_DEVICE_TOKEN` | empty | When set, device clients must present it as `?token=` |

### REST endpoints

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/health` | Status, uptime and connected client counts |
| `GET` | `/api/clients` | Connected web and device counts |
| `GET` | `/api/state` | Full snapshot: tasks, timer, settings, stats, sessions |
| `GET` | `/api/stats` | Aggregated focus statistics |
| `GET` | `/api/tasks` | All tasks, pending first |
| `POST` | `/api/tasks` | Create a task |
| `PATCH` | `/api/tasks/{id}` | Update title, estimate, progress or status |
| `DELETE` | `/api/tasks/{id}` | Delete a task, detaching its sessions |
| `POST` | `/api/tasks/reorder` | Set an explicit order by id |
| `GET` | `/api/sessions?limit=25` | Recent focus and break sessions |
| `GET` | `/api/settings` | Timer settings |
| `PUT` / `PATCH` | `/api/settings` | Replace the timer settings |
| `POST` | `/api/timer/start` | Start or resume |
| `POST` | `/api/timer/pause` | Pause, freezing the remaining time |
| `POST` | `/api/timer/reset` | Reset the phase, recording an abandoned session |
| `POST` | `/api/timer/skip` | Jump to the next phase without crediting a task |
| `POST` | `/api/timer/active-task` | Set the task that receives focus credit |

Errors always use the same envelope:

```json
{
  "error": {
    "code": "validation_error",
    "message": "title: must not be empty",
    "field": "title"
  }
}
```

### WebSocket protocol

Connect to `/ws`. Browser clients are treated as `web`, devices must ask for `kind=device`.

On connect the server sends `hello` and then a complete `snapshot`, so a client never needs a
separate REST round trip to start.

**Server to client**

| Type | Payload |
| --- | --- |
| `hello` | Client identity, kind and server clock |
| `snapshot` | Full `Snapshot`, identical to `GET /api/state` |
| `tick` | Emitted every second: phase, status, remaining, progress, active task counters |
| `tasks` | Full task list after any change |
| `settings` | Timer settings after a change |
| `task.completed` | Id, title and completion time |
| `phase.changed` | From, to, reason and whether it auto-started |
| `notify` | Level, title and message for the notification stack |
| `error` | Code and message for a rejected command |

**Client to server**

| Type | Payload |
| --- | --- |
| `timer.start` / `timer.pause` / `timer.reset` / `timer.skip` | none |
| `task.create` | `{ "title": string, "estimatedPomodoros": number }` |
| `task.setActive` / `task.complete` | `{ "id": string }` |
| `settings.update` | `{ "settings": Settings }` |
| `ping` | none, answered with `hello` |

---

## The timer engine

The engine in `apps/api/internal/timer` owns the clock. On boot it loads the persisted state,
and if it finds a running timer it converts it to paused at the correct remaining time, so
restarting the API never loses or double counts a block.

During a phase:

- A completed focus block records a session, increments `completedPomodoros` on the active task
  and, if the estimate is reached, marks the task done and selects the next pending task
- The next phase is a short break, or a long break every `longBreakEveryCycles` focus blocks
- With `autoStartBreaks` on, breaks begin on their own; focus always waits for you
- Skipping never credits a task; resetting records an abandoned session so the history stays honest
- Changing a duration while the timer runs does not truncate the current block
- Every change is persisted to SQLite immediately, so state survives a crash

---

## The firmware

### Wiring

| Display | ESP32 |
| --- | --- |
| SSD1306 `SDA` | GPIO 21 |
| SSD1306 `SCL` | GPIO 22 |
| Buttons to `GND` | GPIO 27, 26, 32 |

The OLED is a 128x64 I2C module at address `0x3C`. Inputs use the internal pull-ups and are
active low, so a button is just a wire to ground. Pins are defined in `src/config.h`.

### Buttons

| Button | GPIO | Action |
| --- | --- | --- |
| Start / Pause | 27 | `timer.start`, or `timer.pause` while running |
| Reset | 26 | `timer.reset` |
| Skip | 32 | `timer.skip` |

Presses are debounced and support long-press and double-click detection.

### What the screen shows

```
FOCUS              ● LINK
----------------------------------
      2 4 : 3 7
   [###################     ]
2/5 Write the firmware
```

A top bar with the phase and the link state, the remaining time in a large font, a progress bar
for the phase, and the active task with its `done/estimate` counter. The title is truncated to
fit. If the API stops sending ticks the link dot turns hollow and the display shows it.

### Flashing

```bash
make firmware-serial     # list the available ports
make flash               # build and upload
make monitor             # open the serial monitor at 115200
```

If the port is not detected automatically:

```bash
make flash UPLOAD_PORT=COM5
```

---

## Make targets

| Target | What it does |
| --- | --- |
| `make help` | List every target |
| `make install` | Install Go modules, npm dependencies and PlatformIO |
| `make dev` | Run the API and the web dev server together |
| `make dev-api` / `make dev-web` | Run one of them |
| `make build` | Build the API binary, the web bundle and the firmware |
| `make test` | Go tests plus the web typecheck |
| `make lint` | `go vet` plus ESLint |
| `make format` | `go fmt` plus Prettier |
| `make run-api` | Build and run the API |
| `make preview` | Serve the production web build |
| `make flash` | Upload the firmware |
| `make monitor` | Open the serial monitor |
| `make firmware-serial` | List serial devices |
| `make db-reset` | Delete the local SQLite file |
| `make clean` | Remove build artifacts |

Variables: `API_ADDR`, `UPLOAD_PORT`, `GO`, `PYTHON`, `PIO`.

---

## Tests

```bash
make test-api     # go test ./...
make test-web     # tsc
make lint
```

The Go suite covers the repositories, the engine and the HTTP surface, and passes under
`go test -race ./...`. It covers the task lifecycle, foreign-key behaviour when a task is deleted,
pomodoro credit, phase transitions, long break scheduling, abandoned sessions, restoring a
running timer after a restart, REST validation, and live WebSocket flows including a device
client controlling the timer.

---

## Current status

**Done**

- Monorepo scaffolding, Makefile orchestration, npm workspaces
- Go API with chi, embedded SQLite, migrations, validation and a CORS-aware error envelope
- Authoritative pomodoro engine with per-task focus credit and automatic task completion
- WebSocket hub with per-client write pumps, heartbeat, backpressure handling and broadcast
- React dashboard with floating macOS-style windows, dock, menu bar, notifications, hotkeys
- To Do, Pomodoro and Insights windows
- ESP32 firmware with OLED rendering, buttons, Wi-Fi and WebSocket client
- Full test suite for the backend

**Next**

- Verify the firmware end to end on real hardware, since the toolchain download had not
  finished at the time of writing
- A settings window beyond the inline pomodoro controls
- Task reordering by drag and drop, the endpoint already exists
- Sound or haptic feedback when a block completes
- Multiple users or workspaces, currently the app is single user by design

---

## License

Private project, no license granted.
