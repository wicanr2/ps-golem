# Windows 版的冒煙測試映像（`psgolem-wine-verify`）。用 tools/build_image.sh wine 建。
#
# 在 xvfb 上用 Wine 跑 psgolem.exe，確認交叉編出的執行檔開得起來、畫面與 Linux 版相同。
# Wine 不等於 Windows：通過只代表執行檔結構與 Ebiten 的 Windows 路徑在 Wine 下可用。
# 內容沿用 ~/cht/kol 的 docker/Dockerfile.verify-wine。
FROM ubuntu:24.04
RUN apt-get update && apt-get install -y --no-install-recommends \
      wine64 wine xvfb xauth libgl1-mesa-dri mesa-vulkan-drivers ca-certificates \
    && rm -rf /var/lib/apt/lists/*
