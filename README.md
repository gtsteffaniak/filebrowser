<div align="center">
  <h2>AcornDrive</h2>
  <p>Secure cloud file storage with blockchain-backed protection — part of the <a href="https://acorn.tools">Acorn Tools</a> platform.</p>
</div>

---

## About

AcornDrive is a fork of [FileBrowser Quantum](https://github.com/gtsteffaniak/filebrowser) extended with:

- **Azure AD B2C authentication** — SSO login via ChainFS / Nansen identity
- **ChainFS integration** — right-click → Protect uploads files to the blockchain for tamper-evident storage
- **Subscription-aware quota** — per-user storage quota pulled live from the Acorn.Tools billing system
- **Acorn.Tools theme** — teal branding consistent with the broader Acorn Tools suite

## Environments

| Branch | Environment | ChainFS API |
|--------|-------------|-------------|
| `dev`  | DEV | https://nansendev.azurewebsites.net |
| `uat`  | UAT | https://nansenuat.azurewebsites.net |
| `main` | PROD | https://nansenprod.azurewebsites.net |

Pushing to any of these branches triggers an automatic Azure Container Apps deployment via GitHub Actions.

## Quick Start (local dev)

```bash
# Build frontend
cd frontend && npm install && npm run build

# Build & run backend against DEV ChainFS
cd backend && go build -o filebrowser && ./filebrowser -c config.dev.yaml
```

Open `http://localhost:8080` — login with the **ChainFS Login** button (Azure AD B2C).

Use `--chainfs-bypass` to skip ChainFS and run with local password auth:

```bash
./filebrowser --chainfs-bypass -c config.dev.yaml
```

## Documentation

| File | Purpose |
|------|---------|
| [BUILD.md](BUILD.md) | Full build, run, config, and Azure deployment guide |
| [Fork.md](Fork.md) | Changelog from the upstream FileBrowser Quantum project |
| [Todo.md](Todo.md) | Current tasks and planned work |
| [THEME_UPDATES_FINAL.md](THEME_UPDATES_FINAL.md) | Theme colour specifications |

## Architecture

```
Azure Front Door (SSL / CDN / WAF)
        │
Azure Container App  ←── ACR (acorntoolsregistry)
  Go binary + Vue SPA
        │
        ├── acorndrive-srv  (Azure Files NFS, 100 GiB) → /srv  user files
        └── acorndrive-data (Azure Files NFS, 32 GiB)  → /data database + config
```

Quota values are fetched live from the [Acorn.Tools landing page API](https://acorn.tools/api/internal/acorn-drive/access) and cached per-user for 10 minutes. The sidebar progress bar reflects each user's subscription quota (default 4 GiB for subscribers).

## Related Projects

- [FileBrowser Quantum](https://github.com/gtsteffaniak/filebrowser) — upstream project
- [Acorn.Tools Landing Page](https://acorn.tools) — subscription and billing
- ChainFS API — `C:\git\azure-blockchain-workbench-app\NasenAPI`
