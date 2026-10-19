#!/usr/bin/env bash
# 在 Linux 上交叉編出 macOS 版（arm64 ＋ x86_64，lipo 成 universal）。
#
#   tools/build_macos.sh <輸出目錄>      # 產出 <輸出目錄>/psgolem、<輸出目錄>/psexe
#
# Ebiten 的 macOS 後端要 cgo，CC 指到 osxcross 的 clang（映像見 tools/docker/osxcross-go.Dockerfile）。
# 工具鏈前綴帶 SDK 次版號（SDK 15.5 → darwin24.5），從 osxcross-conf 讀，不寫死。
# 做法沿用 ~/cht/kol 的 tools/build_macos.sh（docs/spec/005-cross-platform-delivery.md）。
#
# Linux 上執行不了 Mach-O：這裡只做得到 tools/verify_macos.sh 的結構驗收，不等於功能正常。
set -euo pipefail
OUT="${1:?用法：build_macos.sh <輸出目錄>}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE="${PSGOLEM_MAC_IMAGE:-psgolem-osxcross:1.24-15.5-r1}"
MIN="${PSGOLEM_MACOS_MIN:-11.0}"
LDFLAGS="${PSGOLEM_LDFLAGS:-}"
mkdir -p "$OUT"
ABS="$(cd "$OUT" && pwd)"
case "$ABS" in "$ROOT"/*) REL="${ABS#"$ROOT"/}" ;; *) echo "輸出目錄要在 repo 內" >&2; exit 2 ;; esac
mkdir -p "$ROOT/workplace/gomodcache"

CPUSET=()
[[ -n "${PSGOLEM_CPUSET:-}" ]] && CPUSET=(--cpuset-cpus "$PSGOLEM_CPUSET")

exec timeout "${PSGOLEM_MAC_TIMEOUT:-1800}" docker run --rm --network none \
  --memory 6g --cpus "${PSGOLEM_CPUS:-4}" --pids-limit 512 "${CPUSET[@]}" \
  --log-opt max-size=10m --log-opt max-file=3 \
  -u "$(id -u):$(id -g)" \
  -e HOME=/tmp -e "MIN=$MIN" -e "OUTREL=$REL" -e "LDF=$LDFLAGS" \
  -v "$ROOT:/src" -v "$ROOT/workplace/gomodcache:/gomodcache:ro" -w /src \
  --tmpfs "/tmp:exec,uid=$(id -u),gid=$(id -g),size=3g" \
  "$IMAGE" bash -c '
    set -euo pipefail
    eval "$(osxcross-conf)"
    export GOMODCACHE=/gomodcache GOCACHE=/tmp/gocache GOPATH=/tmp/gopath GOMAXPROCS=2
    export GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=mod
    export CGO_ENABLED=1 GOOS=darwin MACOSX_DEPLOYMENT_TARGET=$MIN
    for cmd in psgolem psexe; do
      for arch in arm64 amd64; do
        case $arch in arm64) pre=arm64-apple-$OSXCROSS_TARGET ;; amd64) pre=x86_64-apple-$OSXCROSS_TARGET ;; esac
        echo "== $cmd $arch（$pre）"
        env GOARCH=$arch CC=$pre-clang CXX=$pre-clang++ \
            CGO_CFLAGS="-mmacosx-version-min=$MIN" CGO_LDFLAGS="-mmacosx-version-min=$MIN" \
          go build -p 2 -trimpath -ldflags "$LDF" -o /tmp/$cmd-$arch ./cmd/$cmd
      done
      x86_64-apple-$OSXCROSS_TARGET-lipo -create /tmp/$cmd-arm64 /tmp/$cmd-amd64 -output "/src/$OUTREL/$cmd"
      x86_64-apple-$OSXCROSS_TARGET-lipo -info "/src/$OUTREL/$cmd"
    done
  '
