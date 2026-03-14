# beegfs-mon-prom

Prometheus exporter for [BeeGFS](https://www.beegfs.io/) metrics. Reimplements the monitoring logic from BeeGFS's built-in `beegfs-mon` daemon but exposes metrics via a Prometheus `/metrics` endpoint instead of writing to InfluxDB/Cassandra.

Collects data by shelling out to `beegfs-ctl`, so it requires the `beegfs-utils` package.

## Metrics

| Metric | Labels | Description |
|--------|--------|-------------|
| `beegfs_meta_responding` | node_id, hostname | Whether the meta node is responding (1/0) |
| `beegfs_meta_indirect_work_queue_size` | node_id, hostname | Meta node indirect work queue size |
| `beegfs_meta_direct_work_queue_size` | node_id, hostname | Meta node direct work queue size |
| `beegfs_meta_sessions` | node_id, hostname | Meta node session count |
| `beegfs_meta_work_requests_total` | node_id, hostname | Meta node work requests |
| `beegfs_meta_queued_requests` | node_id, hostname | Meta node queued requests |
| `beegfs_meta_net_send_bytes_total` | node_id, hostname | Meta node network bytes sent |
| `beegfs_meta_net_recv_bytes_total` | node_id, hostname | Meta node network bytes received |
| `beegfs_storage_responding` | node_id, hostname | Whether the storage node is responding (1/0) |
| `beegfs_storage_disk_space_total_bytes` | node_id, hostname | Storage node total disk space |
| `beegfs_storage_disk_space_free_bytes` | node_id, hostname | Storage node free disk space |
| `beegfs_storage_disk_write_bytes_total` | node_id, hostname | Storage node disk write bytes |
| `beegfs_storage_disk_read_bytes_total` | node_id, hostname | Storage node disk read bytes |
| `beegfs_storage_net_send_bytes_total` | node_id, hostname | Storage node network bytes sent |
| `beegfs_storage_net_recv_bytes_total` | node_id, hostname | Storage node network bytes received |
| `beegfs_target_disk_space_total_bytes` | node_id, target_id | Storage target total disk space |
| `beegfs_target_disk_space_free_bytes` | node_id, target_id | Storage target free disk space |
| `beegfs_target_inodes_total` | node_id, target_id | Storage target total inodes |
| `beegfs_target_inodes_free` | node_id, target_id | Storage target free inodes |
| `beegfs_target_consistency_state` | node_id, target_id | Target consistency (0=good, 1=needs_resync, 2=bad) |
| `beegfs_client_meta_ops_total` | client, operation | Client metadata operations |
| `beegfs_client_storage_ops_total` | client, operation | Client storage operations |

## Install

### Ubuntu/Debian

Download the `.deb` from the [releases page](https://github.com/pokgak/beegfs-mon-prom/releases) and install:

```bash
sudo dpkg -i beegfs-mon-prom_*.deb
```

Edit `/etc/default/beegfs-mon-prom` to configure:

```
BEEGFS_MON_PROM_ARGS="--mgmtd=10.0.0.1:8008 --listen=:9100"
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
  --mgmtd=10.0.0.1:8008
```

Note: the container needs `beegfs-ctl` available — mount it from the host or use `--network host` with a host-installed `beegfs-utils`.

### From source

```bash
go install github.com/pokgak/beegfs-mon-prom@latest
```

## Usage

```
beegfs-mon-prom [flags]

Flags:
  --listen string     address to listen on (default ":9100")
  --mgmtd string      management daemon host:port (e.g. 10.0.0.1:8008)
  --config string     path to beegfs client config file
  --interval duration collection interval for cached metrics (default 30s)
```

## Releasing

Tag and push to trigger a release build:

```bash
git tag v0.1.0
git push --tags
```

GoReleaser builds Linux binaries (amd64/arm64), Docker images, and deb/rpm packages.
