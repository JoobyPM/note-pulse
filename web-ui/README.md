# NotePulse

Lightweight, keyboard‑friendly note‑taking SPA built with vanilla JavaScript,
Vite and Tailwind CSS.

## Features

- **Instant search & virtual scrolling** - handle thousands of notes without
  slowdowns
- **Offline‑first** state with WebSocket live sync
- **Dark mode** with local storage persistence
- **Centralised store** and pub‑sub updates
- **Full keyboard support & ARIA markup**
- **End‑to‑end tests** powered by Playwright

## Tech Stack

| Layer        | Choice                   |
| ------------ | ------------------------ |
| Build        | Vite + pnpm              |
| Styling      | Tailwind CSS             |
| State        | Plain JS store + pub‑sub |
| Networking   | Fetch API + WebSocket    |
| Virtual list | clusterize.js            |
| Tests        | Playwright               |

## Folder Layout

```text
package.json          - pnpm scripts & deps
vite.config.js        - build config
src/
  main.js             - bootstrap & router
  store.js            - global state
  api.js              - HTTP, tokens, WebSocket
  ui.js               - DOM helpers & screens
  templates/          - string templates
  utils/              - colour, time, sanitizer …
  generated/          - compiled CSS (dev only)
  tailwind.src.css    - Tailwind source
  index.html          - app shell
  tests/              - Playwright specs
```

## Quick Start

```bash
pnpm install           # fetch deps
pnpm dev               # dev server + CSS watch
```

[http://localhost:5173](http://localhost:5173) opens automatically.

### Production build

```bash
pnpm build             # bundle app & CSS
pnpm preview           # serve built files
```

### Other scripts

| Command         | Purpose             |
| --------------- | ------------------- |
| pnpm dev\:css   | watch Tailwind only |
| pnpm build\:css | one‑off CSS build   |
| pnpm lint       | run ESLint          |
| pnpm test       | Playwright e2e run  |

## Accessibility & Testing

All interactive elements include proper roles, labels and focus states.
Playwright covers happy paths, dark mode and screen reader semantics.

## Requirements

- Node 21+
- pnpm 9+

## License

MIT
