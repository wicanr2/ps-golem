# 交叉 oracle 映像（`psgolem-oracle-mednafen`）：Debian bookworm 的 mednafen（GPL-2.0-or-later），
# 在 xvfb 下執行，以 xdotool 送鍵、mednafen 的截圖熱鍵存圖（docs/spec/014-cross-oracle.md）。
FROM debian:bookworm
RUN apt-get update && apt-get install -y --no-install-recommends \
      mednafen xvfb xauth xdotool libgl1-mesa-dri \
    && rm -rf /var/lib/apt/lists/* \
    && dpkg-query -W mednafen > /opt/mednafen-version.txt
