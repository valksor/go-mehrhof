# Deployment

Configurations for running kvelmo in production, from local service supervision to containerized orchestration.

| Subdirectory | Use when |
|---|---|
| `docker/` | Running kvelmo in a single container or with Compose |
| `kubernetes/` | Deploying kvelmo to a Kubernetes cluster |
| `systemd/` | Running kvelmo as a managed daemon on Linux |
| `launchd/` | Running kvelmo as a managed agent on macOS |

## Docker

```bash
docker compose -f deploy/docker/docker-compose.yaml up -d
```

The Dockerfile lives at `deploy/docker/Dockerfile`. See the compose file for environment variables and volume mounts.

## Kubernetes

```bash
kubectl apply -k deploy/kubernetes/
```

Kustomize entry point is `deploy/kubernetes/kustomization.yaml`. Adjust `configmap.yaml` for your environment before applying.

## systemd (Linux)

```bash
# Install the service file
sudo cp deploy/systemd/kvelmo.service /etc/systemd/system/

# Create the kvelmo user (if needed)
sudo useradd -r -s /bin/false -m kvelmo

# Reload systemd and enable
sudo systemctl daemon-reload
sudo systemctl enable kvelmo
sudo systemctl start kvelmo

# Check status
sudo systemctl status kvelmo
journalctl -u kvelmo -f
```

## launchd (macOS)

```bash
# Install the plist
cp deploy/launchd/com.valksor.kvelmo.plist ~/Library/LaunchAgents/

# Create log directory
mkdir -p /usr/local/var/log/kvelmo

# Load the service
launchctl load ~/Library/LaunchAgents/com.valksor.kvelmo.plist

# Check status
launchctl list | grep kvelmo
```

## Configuration notes

Both the systemd unit and the launchd plist use `--log-format json` for structured logging and set `KVELMO_ENVIRONMENT=prod` for production guardrails. Adjust the binary path (`/usr/local/bin/kvelmo` by default) to match your install location.
