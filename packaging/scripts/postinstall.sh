#!/bin/sh
set -e

if ! getent group beegfs-mon-prom >/dev/null 2>&1; then
    groupadd --system beegfs-mon-prom
fi

if ! getent passwd beegfs-mon-prom >/dev/null 2>&1; then
    useradd --system --no-create-home --shell /usr/sbin/nologin -g beegfs-mon-prom beegfs-mon-prom
fi

systemctl daemon-reload
systemctl enable beegfs-mon-prom.service
