# macOS 交叉編譯映像（`psgolem-osxcross`）。用 tools/build_image.sh osxcross 建。
#
# Ebiten 的 macOS 後端走 Cocoa／Metal，要 cgo，所以 CC 要指到 osxcross 的 clang。
# `crazymax/osxcross:15.5-debian` 的執行檔要 glibc 2.38，`psgolem-go-ebiten` 是 Debian 12
# （glibc 2.36）跑不動，所以以 Ubuntu 24.04 起底，再把同一份 Go 工具鏈搬過來。
# 做法沿用 ~/cht/kol 的 docker/Dockerfile.osxcross-go（docs/spec/005-cross-platform-delivery.md）。
#
# macOS SDK 的授權只允許在 Apple 硬體上使用：這個映像只留本機，不推送、不散布。
FROM psgolem-go-ebiten:1.24-r1 AS gobase
FROM crazymax/osxcross:15.5-debian AS osxcross

FROM ubuntu:24.04
RUN apt-get update && apt-get install -y --no-install-recommends \
      clang lld llvm libxml2 zlib1g liblzma5 libssl3 file ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY --from=osxcross /osxcross /osxcross
# 少這一行 ld64 起不來，clang 只會轉述成「找不到指令」。
RUN echo /osxcross/lib > /etc/ld.so.conf.d/osxcross.conf && ldconfig
COPY --from=gobase /usr/local/go /usr/local/go
ENV PATH="/osxcross/bin:/usr/local/go/bin:${PATH}" GOTOOLCHAIN=local
