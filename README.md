# Pika

<div align="center">

Lightweight probe monitoring — Go + PostgreSQL/SQLite + VictoriaMetrics

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](.) [![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?logo=docker)](.) [![License](https://img.shields.io/badge/license-Apache--2.0-green)](./LICENSE) [![Stars](https://img.shields.io/github/stars/pika-monitor/pika?style=social)](https://github.com/pika-monitor/pika)

[English](./README.md) | [简体中文](./README.zh-CN.md) · [Website](https://pika.termark.app) · [Docs](./docs/features.md)

</div>

## Overview

Pika is a lightweight probe monitoring system. Probes push metrics to the server over WebSocket; VictoriaMetrics stores time-series while PostgreSQL/SQLite stores business data. Beyond monitoring, it provides Linux incident response and baseline checks to surface security risks early.

## Features

- **📊 Real-time metrics** — CPU / Memory / Disk / Network / GPU / Temperature, with multi-range history
- **🔍 Service checks** — HTTP(S) / TCP / ICMP, including cert expiry detection
- **🛡️ Tamper protection** — fsnotify watch, immutable attribute patrol, and alerting
- **🔒 Security audit** — asset inventory, risk grading (Critical/High/Medium/Low), and audit history
- **🔐 Auth** — Basic Auth (bcrypt) / OIDC / GitHub OAuth
- **📦 One-command deploy** — Docker Compose, SQLite or PostgreSQL

See [Features](./docs/features.md) for details.

## Screenshots

| Public | Security | Tamper |
| --- | --- | --- |
| ![public1](screenshots/public1.png) | ![sec1](screenshots/sec1.png) | ![tamper](screenshots/tamper.png) |
| ![public2](screenshots/public2.png) | ![sec2](screenshots/sec2.png) | ![setting](screenshots/setting.png) |

## Quick Start

### SQLite

```bash
curl -O https://raw.githubusercontent.com/pika-monitor/pika/main/docker-compose.sqlite.yml
curl -o config.yaml https://raw.githubusercontent.com/pika-monitor/pika/main/config.sqlite.yaml
# Edit config.yaml: change JWT secret and admin password
docker compose -f docker-compose.sqlite.yml up -d
# Open http://localhost:8080  — default admin / admin123
```

See [SQLite guide](./docs/deployment-sqlite.md).

### PostgreSQL

```bash
curl -O https://raw.githubusercontent.com/pika-monitor/pika/main/docker-compose.postgresql.yml
curl -o config.yaml https://raw.githubusercontent.com/pika-monitor/pika/main/config.postgresql.yaml
# Edit config.yaml: change database password, JWT secret and admin password
docker compose -f docker-compose.postgresql.yml up -d
# Open http://localhost:8080  — default admin / admin123
```

See [PostgreSQL guide](./docs/deployment-postgresql.md).

## Docs

- [Features](./docs/features.md)
- [SQLite deployment](./docs/deployment-sqlite.md)
- [PostgreSQL deployment](./docs/deployment-postgresql.md)
- [Common config](./docs/common-config.md)

## Requirements

- Docker 20.10+
- Docker Compose 1.29+ (or `docker compose` v2)

## Community

- Community: See https://pika.termark.app for community channels.


---

<div align="center">

**Pika — keep every probe visible.**

</div>
