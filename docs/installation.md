# Installation

File Browser Next is a single binary and can be used as a standalone executable. It is also available as a [Docker](https://www.docker.com) image. Installation is straightforward on any platform.

## Quick Install (Automated)

The quickest and easiest way to install File Browser Next is to use our official installation script, which automatically detects your operating system and CPU architecture, downloads the latest binary, and installs it to your system PATH.

### Linux & macOS

```sh
curl -fsSL https://raw.githubusercontent.com/FilebrowserNext/get/main/get.sh | bash
```

Or using `wget`:

```sh
wget -qO- https://raw.githubusercontent.com/FilebrowserNext/get/main/get.sh | bash
```

Once installed, launch File Browser Next with:

```sh
filebrowser -r /path/to/your/files
```

### Windows (PowerShell)

Run PowerShell as Administrator:

```powershell
iwr -useb https://raw.githubusercontent.com/FilebrowserNext/get/main/get.ps1 | iex
```

Once installed, launch File Browser Next in a new terminal:

```powershell
filebrowser -r C:\path\to\your\files
```

### Homebrew (macOS)

Install directly with one command:

```sh
brew install FilebrowserNext/tap/filebrowser
```

Or tap the repository first:

```sh
brew tap FilebrowserNext/tap
brew install filebrowser
```

## Manual Binary Download

If you prefer downloading and extracting the binary manually, pre-compiled archives are available on our [releases page](https://github.com/FilebrowserNext/filebrowserNEXT/releases).

### Linux (amd64)

```sh
curl -L https://github.com/FilebrowserNext/filebrowserNEXT/releases/latest/download/filebrowser-linux-amd64.tar.gz | tar xz
chmod +x filebrowser
./filebrowser -r /path/to/your/files
```

### Linux (arm64 / Raspberry Pi)

```sh
curl -L https://github.com/FilebrowserNext/filebrowserNEXT/releases/latest/download/filebrowser-linux-arm64.tar.gz | tar xz
chmod +x filebrowser
./filebrowser -r /path/to/your/files
```

### macOS (Apple Silicon)

```sh
curl -L https://github.com/FilebrowserNext/filebrowserNEXT/releases/latest/download/filebrowser-darwin-arm64.tar.gz | tar xz
chmod +x filebrowser
./filebrowser -r /path/to/your/files
```

### macOS (Intel)

```sh
curl -L https://github.com/FilebrowserNext/filebrowserNEXT/releases/latest/download/filebrowser-darwin-amd64.tar.gz | tar xz
chmod +x filebrowser
./filebrowser -r /path/to/your/files
```

### Windows (amd64)

Download the archive and extract it:

```
https://github.com/FilebrowserNext/filebrowserNEXT/releases/latest/download/filebrowser-windows-amd64.zip
```

Then run:

```powershell
.\filebrowser.exe -r C:\path\to\your\files
```

## Running as a Background Service

### Linux

#### Quick Background Run (Nohup)

Run File Browser Next detached from your terminal:

```sh
nohup filebrowser -r /path/to/your/files > filebrowser.log 2>&1 &
```

To stop it:

```sh
pkill filebrowser
```

#### Production Service (Systemd)

Create and enable a systemd service so File Browser Next starts automatically on boot:

```sh
sudo tee /etc/systemd/system/filebrowser.service > /dev/null <<EOF
[Unit]
Description=File Browser Next
After=network.target

[Service]
ExecStart=/usr/local/bin/filebrowser -r /path/to/your/files
Restart=on-failure
User=nobody

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable --now filebrowser
```

To stop or view logs:

```sh
sudo systemctl status filebrowser
sudo systemctl stop filebrowser
```

---

### macOS

#### Quick Background Run (Nohup)

```sh
nohup filebrowser -r /path/to/your/files > filebrowser.log 2>&1 &
```

To stop it:

```sh
pkill filebrowser
```

#### Launchd Agent (Auto-start on login)

Create `~/Library/LaunchAgents/com.filebrowsernext.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.filebrowsernext</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/filebrowser</string>
        <string>-r</string>
        <string>/Users/your-username/files</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
</dict>
</plist>
```

Load the service:

```sh
launchctl load ~/Library/LaunchAgents/com.filebrowsernext.plist
```

To stop it:

```sh
launchctl unload ~/Library/LaunchAgents/com.filebrowsernext.plist
```

---

### Windows

#### Quick Background Run (PowerShell)

Launch File Browser Next silently without an open command window:

```powershell
Start-Process filebrowser -ArgumentList "-r C:\path\to\your\files" -WindowStyle Hidden
```

To stop the background process:

```powershell
Stop-Process -Name filebrowser
```

#### Windows Task Scheduler (Auto-start on boot)

Create a scheduled task to run File Browser Next automatically at logon:

```powershell
schtasks /create /tn "FileBrowserNext" /tr "filebrowser.exe -r C:\path\to\your\files" /sc onlogon /rl highest
```

To start or delete the task:

```powershell
schtasks /run /tn "FileBrowserNext"
schtasks /delete /tn "FileBrowserNext" /f
```

## Docker

File Browser Next is designed to run seamlessly in Docker as a single, ultra-lightweight container (approx. 26 MB).

### Method 1: Docker Compose (Recommended)

Docker Compose is the cleanest way to run and manage File Browser Next on a server, NAS, or homelab.

Create a `docker-compose.yml` file:

```yaml
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
