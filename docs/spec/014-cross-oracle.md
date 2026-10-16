# 014 交叉 oracle

狀態：`READY`

## 1. 為什麼

自己畫出的畫面要拿成熟模擬器比對過才算數（`AGENTS.md`「原版與交叉 oracle」）。
ps-golem 的週期模型與其他模擬器不同，同一份按鍵腳本在兩邊不會落在同一個 field，
所以比對只在畫面靜止、輸入粗略的場景做，比的是「遊戲走到同一個狀態時畫面是否一致」。

## 2. oracle

| 項目 | 內容 |
|---|---|
| 模擬器 | mednafen 1.29.0（Debian bookworm 套件 `mednafen 1.29.0+dfsg-1+b1`，GPL-2.0-or-later） |
| 映像 | `psgolem-oracle-mednafen`（`tools/docker/oracle-mednafen.Dockerfile`，`tools/build_image.sh mednafen`） |
| 執行 | `tools/oracle/mednafen.sh <BIOS> <bin> <時間表> <輸出目錄>`：xvfb 內以實際時間執行，xdotool 依時間表送鍵，F9 截圖 |
| 輸出 | 原生解析度（`-psx.xscale 1 -psx.yscale 1`）PNG，存在 `<輸出目錄>/snaps/` |

mednafen 以實際時間執行，時間表的秒數只是大約位置，結果不可逐位元重現；截圖的場景以人工核對。

beetle-psx-libretro 的 libretro core 配上自寫的無頭前端，這片光碟停在 PS 標誌畫面不開機
（commit `e57860d` 從原始碼建；Debian 的 2015 年版則在載入時丟 Input/output error），原因未查明，不用作 oracle。

## 3. 比對方法

- 兩邊用同一份按鍵腳本：psexe 的 `-press`（以 field 計）換算成秒數（約 60 field／秒，起點對齊標題畫面）給 mednafen。
- ps-golem 以 `psexe -shots <目錄> -shot-every 10` 連續截圖。
- `tools/oracle/match.go`：對每張 mednafen 截圖，在指定的 field 範圍內找最接近的一張 ps-golem 截圖。
  mednafen 的截圖含水平邊框（顯示寬 256 時為 280、320 類為 364→400、512 為 560），
  所以在 mednafen 圖內水平平移找位置，只比 ps-golem 的顯示範圍；各色比高 5 位元（PS1 是 15 位元色）。
- 判準：靜止場景 0 像素不同；有動畫的場景，不同的像素只能落在動畫元素（游標、燭火、捲動背景、角色動作）上，
  以三欄對照圖（oracle｜ps-golem｜差異）人工核對。發現其他不一致先懷疑 ps-golem，記入 §5。

## 4. 限制

- 只看畫面。mednafen 不提供 RAM 觀測，狀態比對不在本規格範圍。
- 無法驗證聲音。

## 5. 結果

2026-10-16，SCPH5500，mednafen 1.29.0。同一份腳本在 mednafen 也走到序章王座廳開戰。

| 場景 | ps-golem field | 不同像素 | 差異所在 |
|---|---|---|---|
| 標題畫面（512 寬） | 4320 | 0 / 122880 | |
| 選單 SELECT YOUR DESTINY（364 寬） | 4870 | 0 / 87360 | |
| 命名畫面 | 5940 | 31 / 87360 | 游標框動畫 |
| Now Loading | 10550 | 333 / 61440 | 讀取動畫 |
| 序章樓梯下 | 11140 | 3424 / 61440 | 捲動中的雲、背景草叢、Richter 動作格 |
| 王座廳對話（中文） | 13350 | 0 / 61440 | |
| 王座廳對話（中文） | 13650、13890、14480 | 36、36、43 | 心數閃爍、燭火 |
| 王座廳對話（中文） | 14190 | 103 | 同上 |
| 王座廳入口 | 12610、12620 | 2527、2496 | 走動中，兩邊位置不同 |
| 王座廳開戰 | 14730 | 1257 | 德古拉現身動畫 |

沒有發現動畫元素以外的差異。
