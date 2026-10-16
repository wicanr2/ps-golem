#!/usr/bin/env bash
# macOS 交叉編產物的結構驗收。
#
#   tools/verify_macos.sh <binary>...
#
# Linux 上執行不了 Mach-O，全過只代表不會因結構問題開不起來，不代表功能正常。
# 四道（沿用 ~/cht/kol 的 tools/verify_macos.sh）：
#   1. 雙弧（arm64 ＋ x86_64）。
#   2. arm64 有 LC_CODE_SIGNATURE；少了它 Apple Silicon 上會 `Killed: 9`。
#   3. 最低系統版本讀得出來。
#   4. 動態相依只落在 /usr/lib/ 與 /System/Library/。
set -euo pipefail
[ $# -ge 1 ] || { echo "用法：verify_macos.sh <binary>..." >&2; exit 2; }
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE="${PSGOLEM_MAC_IMAGE:-psgolem-osxcross:1.24-15.5-r1}"
RELS=()
for b in "$@"; do
  a="$(cd "$(dirname "$b")" && pwd)/$(basename "$b")"
  case "$a" in "$ROOT"/*) RELS+=("${a#"$ROOT"/}") ;; *) echo "$b 要在 repo 內" >&2; exit 2 ;; esac
done

exec timeout 300 docker run --rm --network none \
  --memory 2g --cpus 2 --pids-limit 128 \
  --log-opt max-size=10m --log-opt max-file=3 \
  -u "$(id -u):$(id -g)" -e HOME=/tmp \
  -v "$ROOT:/src:ro" -w /src \
  --tmpfs "/tmp:exec,uid=$(id -u),gid=$(id -g),size=512m" \
  "$IMAGE" bash -c '
    set -euo pipefail
    eval "$(osxcross-conf)"
    T=x86_64-apple-$OSXCROSS_TARGET
    for B in "$@"; do
      info=$($T-lipo -info "$B")
      echo "$info"
      for a in arm64 x86_64; do
        case "$info" in *" $a"*) ;; *) echo "$B 缺少架構 $a" >&2; exit 1 ;; esac
      done
      for a in arm64 x86_64; do
        $T-lipo -thin $a "$B" -output /tmp/$a
        # 整份收下再比，不用 `otool | grep -q`：grep -q 提早結束會讓 otool 收到 SIGPIPE，pipefail 判失敗。
        load=$($T-otool -l /tmp/$a)
        if [ "$a" = arm64 ]; then
          case "$load" in *LC_CODE_SIGNATURE*) ;; *) echo "$B arm64 沒有 LC_CODE_SIGNATURE" >&2; exit 1 ;; esac
        fi
        echo "-- $a $(printf "%s\n" "$load" | grep -m1 minos || echo "（沒有 minos）")"
        libs=$($T-otool -L /tmp/$a)
        bad=$(printf "%s\n" "$libs" | tail -n +2 | awk "{print \$1}" | grep -vE "^(/usr/lib/|/System/Library/)" || true)
        if [ -n "$bad" ]; then echo "$B $a 連到系統目錄以外的動態庫：" >&2; echo "$bad" >&2; exit 1; fi
      done
    done
    echo "四道全過（結構驗收；不等於功能驗收）"
  ' _ "${RELS[@]}"
