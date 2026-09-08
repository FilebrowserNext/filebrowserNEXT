# Installation

File Browser is a single binary and can be used as standalone executable. However, it is also available as a [Docker](https://www.docker.com) image. The installation and first time setup is quite straightforward independently of which system you use.

## Binary

The quickest and easiest way to install File Browser is to use a package manager, or our download script, which automatically fetches the latest version of File Browser for your platform. Alternatively, you can manually download the binary from the [releases page](https://github.com/filebrowser/filebrowser/releases).

### Brew

```sh
brew tap filebrowser/tap
brew install filebrowser
filebrowser -r /path/to/your/files
```

### Unix

```sh
curl -fsSL https://raw.githubusercontent.com/filebrowser/get/master/get.sh | bash
filebrowser -r /path/to/your/files
```

### Windows

```sh
iwr -useb https://raw.githubusercontent.com/filebrowser/get/master/get.ps1 | iex
filebrowser -r /path/to/your/files
```

File Browser is now up and running. Read the ["First Boot"](#first-boot) section for more information.

## Docker

File Browser Next is designed to run seamlessly in Docker as a single, ultra-lightweight container (approx. 26 MB).

### Method 1: Docker Compose (Recommended)

Docker Compose is the cleanest way to run and manage File Browser Next on a server, NAS, or homelab.

Create a `docker-compose.yml` file:

```yaml
version: '3.8'

services:
  filebrowser:
    image: ghcr.io/filebrowsernext/filebrowsernext:latest
    container_name: filebrowser
    restart: unless-stopped
    ports:
      - "8080:80"
    volumes:
      - /path/to/your/files:/srv
      - /path/to/database:/database
      - /path/to/config:/config
    environment:
      - PUID=1000
      - PGID=1000
```

Start the container in the background:

```bash
docker compose up -d
```

### Method 2: Docker Run (Single Command)

You can also launch File Browser Next with a single `docker run` command:

```bash
docker run -d \
  --name filebrowser \
  --restart unless-stopped \
  -p 8080:80 \
  -v /path/to/your/files:/srv \
  -v /path/to/database:/database \
  -v /path/to/config:/config \
  ghcr.io/filebrowsernext/filebrowsernext:latest

```

### Understanding Docker Volumes

| Container Path | Purpose | Description |
|---|---|---|
| `/srv` | **Your Files** | The root directory of the files you want to manage. Mount any directory from your host here. |
| `/database` | **User Database** | Contains `filebrowser.db` (users, passwords, permissions). Persisting this volume ensures your accounts are preserved across container updates and restarts. |
| `/config` | **Settings (Optional)** | Contains optional custom `settings.json`. Automatically initialized if empty. |

> [!NOTE]
> **Permissions**: The container runs under non-root user `UID 1000` / `GID 1000`. Ensure that your host directories mounted into `/srv` and `/database` are readable and writable by your user (`chown -R 1000:1000 /path/to/database`).

File Browser Next is now up and running. Read the ["First Boot"](#first-boot) section below for first login details.


## First Boot

Your instance is now up and running. By default, the web interface is accessible in your browser at:

```
http://127.0.0.1:8080
```

If hosted on a remote server or VPS, replace `127.0.0.1` with your server's public or local IP address (e.g. `http://192.168.1.100:8080` or `http://<your-server-ip>:8080`).

File Browser Next automatically initializes its database upon startup. The default administrator credentials are `admin` / `admin` (or the randomly generated password printed to the console on first launch if configured with random credentials).


Although this is the fastest way to bootstrap an instance, we recommend you to take a look at other possible options, by checking [`config init`](cli/filebrowser-config-init.md) and [`config set`](cli/filebrowser-config-set.md), to make the installation as safe and customized as it can be.

If your goal is to have a public-facing deployment, we recommend taking a look at the [deployment](deployment.md) page for more information on how you can secure your installation.
