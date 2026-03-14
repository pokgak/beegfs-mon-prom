# beegfs-mon-prom

Prometheus exporter for [BeeGFS](https://www.beegfs.io/) 8.x metrics. Collects server stats, target capacity/health, and per-client operation counts by invoking the BeeGFS 8.x CLI (`/opt/beegfs/sbin/beegfs`) with JSON output.

> **Note:** v2.0 requires BeeGFS 8.x. For BeeGFS 7.x (using `beegfs-ctl`), use v1.x.

## Metrics

### Server metrics

Labels: `node_type` (meta/storage), `node_id`, `alias`

| Metric | Description |
|--------|-------------|
| `beegfs_server_requests` | Requests per second |
| `beegfs_server_queue_length` | Pending request queue length |
| `beegfs_server_busy_workers` | Busy worker threads |
| `beegfs_server_read_bytes` | Read bytes per second |
| `beegfs_server_written_bytes` | Written bytes per second |
| `beegfs_server_sent_bytes` | Network sent bytes per second |
| `beegfs_server_received_bytes` | Network received bytes per second |

### Target metrics

Labels: `node_type` (meta/storage), `target_id`, `node_id`, `alias`

| Metric | Description |
|--------|-------------|
| `beegfs_target_space_total_bytes` | Total disk space in bytes |
| `beegfs_target_space_free_bytes` | Free disk space in bytes |
| `beegfs_target_inodes_total` | Total inodes |
| `beegfs_target_inodes_free` | Free inodes |
| `beegfs_target_reachable` | Target reachability (1=online, 0=offline) |
| `beegfs_target_consistency_state` | Consistency state (0=good, 1=needs_resync, 2=bad) |

### Client metrics

Labels: `node_type` (meta/storage), `client`, `operation`

| Metric | Description |
|--------|-------------|
| `beegfs_client_ops_total` | Client operations since server start |

### Scrape metadata

| Metric | Description |
|--------|-------------|
| `beegfs_scrape_duration_seconds` | Time spent collecting metrics |
| `beegfs_scrape_success` | Whether the last scrape succeeded (1/0) |

## Install

### Ubuntu/Debian

Download the `.deb` from the [releases page](https://github.com/pokgak/beegfs-mon-prom/releases) and install:

```bash
sudo dpkg -i beegfs-mon-prom_*.deb
```

Edit `/etc/default/beegfs-mon-prom` to configure:

```
BEEGFS_MON_PROM_ARGS="--mgmtd-addr=10.0.0.1:8010 --auth-file=/etc/beegfs/connAuthFile --tls-disable --listen=:9100"
```

Then start:

```bash
sudo systemctl start beegfs-mon-prom
```

### Docker

```bash
docker run -d --name beegfs-mon-prom \
  --network host \
  ghcr.io/pokgak/beegfs-mon-prom:latest \
  --mgmtd-addr=10.0.0.1:8010 \
  --auth-file=/etc/beegfs/connAuthFile \
  --tls-disable
```

Note: the container needs the `beegfs` CLI binary available. Mount it from the host or set `--beegfs-path` to the correct location.

### From source

```bash
go install github.com/pokgak/beegfs-mon-prom@latest
```

## Usage

```
beegfs-mon-prom [flags]

Flags:
  --listen string         address to listen on (default ":9100")
  --beegfs-path string    path to beegfs CLI binary (default "/opt/beegfs/sbin/beegfs")
  --mgmtd-addr string     management daemon gRPC address (e.g. 10.0.0.1:8010)
  --auth-file string      path to BeeGFS auth file
  --tls-disable           disable TLS for gRPC communication
  --tls-cert-file string  path to TLS certificate file
  --interval duration     collection interval for cached metrics (default 30s)
```

## Releasing

Tag and push to trigger a release build:

```bash
git tag v2.0.0
git push --tags
```

GoReleaser builds Linux binaries (amd64/arm64), Docker images, and deb/rpm packages.
