#!/usr/bin/env bash
# Builds gym-server for vps-he, installs it over /opt/gym/gym-server and
# restarts gym.service, putting the previous binary back if the service does
# not come up. The unit, its user and /etc/gym.env are set up once by hand
# (deploy/gym.service); this only ships the binary.
set -euo pipefail

host=${GYM_DEPLOY_HOST:-vps-he}
remote_bin=/opt/gym/gym-server
cd "$(dirname "$0")/.."

arch=$(ssh "$host" uname -m)
case $arch in
  x86_64) goarch=amd64 ;;
  aarch64) goarch=arm64 ;;
  *) echo "deploy: unknown arch $arch" >&2; exit 1 ;;
esac

version=$(git describe --always --dirty)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
CGO_ENABLED=0 GOOS=linux GOARCH=$goarch go build -trimpath \
  -ldflags "-s -w" -o "$tmp/gym-server" ./cmd/gym-server

remote_tmp=$(ssh "$host" mktemp -d)
scp -q "$tmp/gym-server" "$host:$remote_tmp/gym-server"
ssh "$host" bash -s -- "$remote_tmp" "$remote_bin" <<'REMOTE'
set -euo pipefail
tmp=$1 bin=$2
backup=$bin.prev
[[ -f $bin ]] && sudo cp -p "$bin" "$backup"
sudo install -d -m 0755 "$(dirname "$bin")"
sudo install -m 0755 "$tmp/gym-server" "$bin"
rm -rf "$tmp"
sudo systemctl restart gym.service
sleep 2
if ! systemctl is-active --quiet gym.service; then
  echo "deploy: gym.service did not start, putting the previous binary back" >&2
  [[ -f $backup ]] && sudo install -m 0755 "$backup" "$bin" && sudo systemctl restart gym.service
  exit 1
fi
echo "deploy: gym.service active"
REMOTE
echo "deployed $version to $host"
