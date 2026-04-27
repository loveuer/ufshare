# ufshare

> A simple HTTP file server, just like `python -m http.server`.

## Installation

Download the latest binary from the [releases page](../../releases), or pull the Docker image:

```bash
docker pull <your-username>/ufshare:latest
```

## Usage

### Basic

```bash
# Serve current directory on default port 8000
./ufshare

# Specify host, port, and directory
./ufshare -host 0.0.0.0 -port 9000 -dir /path/to/share

# Show hidden files (files starting with .)
./ufshare -hidden
```

### Daemon mode

Run ufshare as a background daemon process:

```bash
# Start as daemon (default pid/log files: ufshare.pid, ufshare.log)
./ufshare -daemon

# Start as daemon with custom pid and log file paths
./ufshare -daemon -pidfile /var/run/ufshare.pid -logfile /var/log/ufshare.log

# Stop the daemon
kill $(cat ufshare.pid)
```

### Docker

```bash
# Serve current directory on port 8000
docker run -d \
  -p 8000:8000 \
  -v $(pwd):/data \
  <your-username>/ufshare:latest

# Serve a specific directory on a custom port
docker run -d \
  -p 9000:9000 \
  -v /path/to/share:/data \
  <your-username>/ufshare:latest \
  -host 0.0.0.0 -port 9000 -dir /data

# Show hidden files
docker run -d \
  -p 8000:8000 \
  -v $(pwd):/data \
  <your-username>/ufshare:latest \
  -hidden
```

## Flags

| Flag        | Default       | Description                    |
|-------------|---------------|--------------------------------|
| `-host`     | `0.0.0.0`     | Listen host                    |
| `-port`     | `8000`        | Listen port                    |
| `-dir`      | `.`           | Directory to serve             |
| `-hidden`   | `false`       | Show hidden files (`.` prefix) |
| `-daemon`   | `false`       | Run as background daemon       |
| `-pidfile`  | `ufshare.pid` | PID file path (daemon mode)    |
| `-logfile`  | `ufshare.log` | Log file path (daemon mode)    |
