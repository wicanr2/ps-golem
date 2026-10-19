# 執行期驗證映像（psgolem-wine-verify:noble-r2），用 tools/build_image.sh wine 建。
# r2 替代 noble-r1，增加隔離的硬體 GPU 視窗與有節拍音訊驗證；不需主機顯示器。
#
# 在 xvfb 上用 Wine 跑 psgolem.exe，確認交叉編出的執行檔開得起來、畫面與 Linux 版相同。
# Wine 不等於 Windows：通過只代表執行檔結構與 Ebiten 的 Windows 路徑在 Wine 下可用。
# 內容沿用 ~/cht/kol 的 docker/Dockerfile.verify-wine。
FROM ubuntu:24.04@sha256:534baea6a22c03a63003dbc8dbe78fe34bc0d7e595d9a9dc9834884ff530eb55
LABEL psgolem.replaces="psgolem-wine-verify:noble-r1"
RUN apt-get update && apt-get install -y --no-install-recommends \
      wine64 wine xvfb xauth libgl1-mesa-dri mesa-vulkan-drivers ca-certificates \
      weston xwayland pulseaudio libasound2-plugins ffmpeg mesa-utils \
    && dpkg-query -W > /opt/psgolem-toolchain-packages.txt \
    && rm -rf /var/lib/apt/lists/*
