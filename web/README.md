# Pika Web

`web/` 是官方管理后台前端（React/Vite），发布到 `/admin/assets/*`。

官方默认公开主题是独立项目 [`pika-monitor/pika-default-theme`](https://github.com/pika-monitor/pika-default-theme)。本地开发时，`Makefile` 默认从同级目录 `../pika-default-theme` 构建它。

## 开发

```bash
cd web && npm ci && npm run dev                  # 管理后台，http://localhost:5174/admin/
cd ../pika-default-theme && npm ci && npm run dev # 默认主题，http://localhost:5173/
```

管理后台开发服务器保留 `/admin/*`，并将其他路径代理到 `http://localhost:8080`。因此启动 Pika
后端后，可以通过 `http://localhost:5174/` 访问当前公开主题，通过
`http://localhost:5174/admin/` 访问管理后台。默认主题项目的开发服务器也会把 `/api/*`
代理到 Pika 后端，两者可以同时启动。

## 构建

`make build-web` 构建 `web/` 和独立的默认主题，将两者放到独立的发布目录：

- 管理后台：`web/dist/`；
- 默认主题：`themes/default/`。

如果主题源码不在默认位置，可以显式指定：

```bash
make DEFAULT_THEME_DIR=/path/to/pika-default-theme build-web
```

GitHub Actions 使用 `.github/default-theme.ref` 中的完整 Git Commit SHA 锁定默认主题，保证同一 Pika 版本的发布构建可复现。

## 运行路径

- 公开主题：`/`、`/servers/*`、`/monitors/*`；
- 管理后台：`/admin/*`，包括 `/admin/login` 和 OAuth/OIDC 回调；
- 管理后台资源：`/admin/assets/*`；
- 活动主题资源：`/t/*`。
