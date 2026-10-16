#!/usr/bin/env bash
# 在 docker 的 xvfb 裡用 mednafen（Debian bookworm 1.29.0）跑光碟，依時間表送鍵與截圖，當交叉 oracle
# （docs/spec/014-cross-oracle.md）。mednafen 以實際時間執行，時間表的秒數只是大約位置。
#
#   tools/oracle/mednafen.sh <BIOS 檔> <bin 檔> <時間表> <輸出目錄> [<1P 記憶卡檔>]
#
# 給記憶卡時放到 mednafen 的 sav/game0.mcr（以 -filesys.fname_sav 固定檔名）再執行；
# 結束後 mednafen 的記憶卡存到 <輸出目錄>/card.mcr。
#
# 時間表每行「秒數 動作 [參數]」，秒數從 mednafen 啟動起算：
#   12.0 press START 0.2     按住 0.2 秒（按鍵名同 psexe -press，可用 + 合併）
#   30   snap                截圖（mednafen F9，存到 <輸出目錄>/snaps/）
#   90   quit
# mednafen 的預設鍵位：方向 W/A/S/D，✕○□△ 為數字鍵盤 2/6/4/8，L1 R1 L2 R2 為 1/3/7/9，START Enter，SELECT Tab。
set -euo pipefail
BIOS="${1:?BIOS}"; BIN="${2:?bin}"; PLAN="${3:?時間表}"; OUT="${4:?輸出目錄}"; CARD="${5:-}"
for f in "$BIOS" "$BIN" "$PLAN"; do [ -f "$f" ] || { echo "$f 不存在" >&2; exit 2; }; done
CUE="$(dirname "$BIN")/$(basename "$BIN" .bin).cue"
[ -f "$CUE" ] || { echo "找不到 $CUE" >&2; exit 2; }
mkdir -p "$OUT"
cp "$PLAN" "$OUT/plan.txt"
if [ -n "$CARD" ]; then
  [ -f "$CARD" ] || { echo "$CARD 不存在" >&2; exit 2; }
  cp "$CARD" "$OUT/card-in.mcd"
fi
exec timeout "${PSGOLEM_ORACLE_TIMEOUT:-3600}" docker run --rm --init --network none \
  --memory 3g --cpus 2 --pids-limit 128 --log-opt max-size=10m --log-opt max-file=3 \
  -u "$(id -u):$(id -g)" --tmpfs "/tmp:exec,uid=$(id -u),gid=$(id -g),size=256m" \
  -v "$(cd "$(dirname "$BIOS")" && pwd):/bios:ro" -v "$(cd "$(dirname "$BIN")" && pwd):/disc:ro" \
  -v "$(cd "$OUT" && pwd):/out" \
  -e "BIOSF=$(basename "$BIOS")" -e "BINF=$(basename "$BIN")" -e "CUEF=$(basename "$CUE")" \
  "${PSGOLEM_MEDNAFEN_IMAGE:-psgolem-oracle-mednafen:bookworm-r1}" \
  xvfb-run -a -s "-screen 0 1024x768x24" bash -c '
    set -u
    export HOME=/tmp
    mkdir -p /tmp/disc
    ln -s "/disc/$BINF" /tmp/disc/game.bin
    { echo "FILE \"game.bin\" BINARY"; grep -av "^FILE" "/disc/$CUEF"; } > /tmp/disc/game.cue
    declare -A K=([UP]=w [DOWN]=s [LEFT]=a [RIGHT]=d [CROSS]=KP_2 [CIRCLE]=KP_6 [SQUARE]=KP_4
      [TRIANGLE]=KP_8 [L1]=KP_1 [R1]=KP_3 [L2]=KP_7 [R2]=KP_9 [START]=Return [SELECT]=Tab)
    # 記憶卡檔名固定為 game0.mcr（預設檔名含遊戲 MD5，事先不知道）。
    MED=(/usr/games/mednafen -psx.bios_jp "/bios/$BIOSF" -sound 0 -video.driver softfb -video.fs 0
      -psx.xscale 1 -psx.yscale 1 -filesys.fname_sav "%f%x" /tmp/disc/game.cue)
    if [ -f /out/card-in.mcd ]; then
      mkdir -p /tmp/.mednafen/sav
      cp /out/card-in.mcd /tmp/.mednafen/sav/game0.mcr
    fi
    "${MED[@]}" > /out/mednafen.log 2>&1 &
    P=$!
    W=""
    for i in $(seq 100); do W=$(xdotool search --pid $P 2>/dev/null | head -1); [ -n "$W" ] && break; sleep 0.1; done
    T0=$(date +%s.%N)
    while read -r t act arg dur; do
      case "$t" in ""|"#"*) continue ;; esac
      now=$(date +%s.%N); wait_s=$(awk -v t="$t" -v n="$now" -v z="$T0" "BEGIN{w=t-(n-z); print (w>0)?w:0}")
      sleep "$wait_s"
      case "$act" in
        press)
          keys=(); IFS=+ read -ra bs <<<"$arg"; for b in "${bs[@]}"; do keys+=("${K[$b]}"); done
          xdotool keydown --window "$W" "${keys[@]}"; sleep "${dur:-0.1}"; xdotool keyup --window "$W" "${keys[@]}" ;;
        snap) xdotool key --window "$W" F9 ;;
        quit) break ;;
      esac
    done < /out/plan.txt
    sleep 1; kill $P; wait $P 2>/dev/null
    cp -r /tmp/.mednafen/snaps /out/ 2>/dev/null || true
    cp /tmp/.mednafen/sav/game0.mcr /out/card.mcr 2>/dev/null || true
  '
