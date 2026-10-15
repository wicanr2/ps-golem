#!/usr/bin/env bash
# 抓 JaCzekanski/ps1-tests（MIT）到 testdata/ps1-tests/。不進版控。
#
#   tools/fetch_ps1tests.sh
#
# 釘住 commit 與 release，避免語料在背後換掉：
#   - gte-fuzz 的硬體紀錄（輸入／輸出暫存器），可直接轉成 GTE 單元測試
#   - release 的 tests.zip（預編的 PS-EXE 與 psx.log），要整台機器才能跑
# 下載在 docker 裡做（需要網路），輸出以目前 UID/GID 寫入。
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMMIT="${PS1TESTS_COMMIT:-f727802fead11f1daa7549285548392ef87749cb}"
RELEASE="${PS1TESTS_RELEASE:-build-158}"
OUT="$ROOT/testdata/ps1-tests"
mkdir -p "$OUT"
exec timeout 15m docker run --rm --init --network bridge \
  --memory 1g --cpus 1 --pids-limit 64 \
  --log-opt max-size=10m --log-opt max-file=3 \
  -u "$(id -u):$(id -g)" -e HOME=/tmp \
  -v "$OUT:/out" -w /out "${PSGOLEM_GO_IMAGE:-psgolem-go-ebiten:1.24-r1}" bash -c '
    set -euo pipefail
    base=https://raw.githubusercontent.com/JaCzekanski/ps1-tests/'"$COMMIT"'
    for f in gte-fuzz/gte_valid_0xc0ffee_50.log gte/test-all/psx.log LICENSE; do
      mkdir -p "$(dirname "$f")"
      curl -fsSL "$base/$f" -o "$f"
    done
    curl -fsSL -o tests.zip \
      https://github.com/JaCzekanski/ps1-tests/releases/download/'"$RELEASE"'/tests.zip
    echo "'"$COMMIT $RELEASE"'" > SOURCE
    sha256sum gte-fuzz/*.log tests.zip > SHA256SUMS
    cat SHA256SUMS
  '
