#!/bin/sh
set -e

systemctl stop beegfs-mon-prom.service || true
systemctl disable beegfs-mon-prom.service || true
