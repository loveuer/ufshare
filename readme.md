# ufshare

> A simple HTTP file server, just like `python -m http.server`.

## Installation

Download the latest binary from the [releases page](../../releases), or pull the Docker image:

```bash
docker pull loveuer/ufshare:latest
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

### Upload with curl

Uploads are disabled by default. Set `UFSHARE_TOKEN` before starting the server
to enable API-only uploads. The token must be at least 32 characters long.
Uploads are limited to 1 GB by default. Set `UFSHARE_MAX_UPLOAD_SIZE` to a byte
size to override the limit.

```bash
export UFSHARE_TOKEN="12345678901234567890123456789012"
export UFSHARE_MAX_UPLOAD_SIZE="1073741824"
./ufshare -dir /path/to/share

curl -T ./local-file.txt \
  -H "Authorization: Bearer ${UFSHARE_TOKEN}" \
  --location "http://127.0.0.1:8000/uploaded-file.txt"
```

Successful uploads return JSON:

```json
{
  "status": 200,
  "action": "uploaded",
  "file": {
    "name": "uploaded-file.txt",
    "path": "uploaded-file.txt",
    "url": "http://127.0.0.1:8000/uploaded-file.txt",
    "size": 123,
    "mod_time": "2026-04-29T02:45:00-07:00"
  }
}
```

`action` is `uploaded` for new files and `updated` when replacing an existing
file. `file.url` can be used directly with `wget`.

Uploading to a nested path creates missing parent directories. Uploading to an
existing file replaces it.

Upload API errors return JSON:

```json
{
  "status": 401,
  "error": "unauthorized"
}
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
  loveuer/ufshare:latest

# Serve a specific directory on a custom port
docker run -d \
  -p 9000:9000 \
  -v /path/to/share:/data \
  loveuer/ufshare:latest \
  -host 0.0.0.0 -port 9000 -dir /data

# Show hidden files
docker run -d \
  -p 8000:8000 \
  -v $(pwd):/data \
  loveuer/ufshare:latest \
  -hidden

# Enable curl uploads
docker run -d \
  -p 8000:8000 \
  -v $(pwd):/data \
  -e UFSHARE_TOKEN="12345678901234567890123456789012" \
  -e UFSHARE_MAX_UPLOAD_SIZE="1073741824" \
  loveuer/ufshare:latest
```

## HTTP methods

| Method | Description                                      |
|--------|--------------------------------------------------|
| `GET`  | Browse directories, download files, and preview  |
| `HEAD` | Check file metadata without downloading the body |
| `PUT`  | Upload files when `UFSHARE_TOKEN` is configured  |

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
