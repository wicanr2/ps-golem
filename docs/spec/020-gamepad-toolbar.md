# 020 自動隱藏工具列與手把設定

狀態：`READY`（實作與隔離驗收完成，實體手把待真機）。只處理作業系統已連線的 USB／藍芽手把。

## 1. 使用者決定與原型

2026-10-18 使用者要求在工具列加入藍芽與 USB 手把設定，並確認：

- 設定已連線手把；藍芽配對由系統處理。
- 沿用展示的手把設定面板，工具列改為滑鼠移到上緣或按 F1 才顯示。
- 面板含裝置選擇、按鍵重設、搖桿死區、即時測試、恢復預設、取消與儲存。
- 設定面板開啟時暫停遊戲，工具列單獨顯示時繼續執行。

定版原型在 `docs/ui/gamepad-settings-prototype.png`。原型顯示工具列展開狀態，
正式版本平常隱藏；除此之外維持原型的面板分區、配色與控制位置。
原型資料為自製介面與示例裝置，不是實體手把驗收收據。

## 2. 已核實介面與限制

固定版本 Ebitengine v2.9.9，本機來源為 `workplace/gomodcache/github.com/hajimehoshi/ebiten/v2@v2.9.9/input.go`：

- AppendGamepadIDs、GamepadName、GamepadSDLID 可列舉已連線裝置及模型識別。
- 標準配置、標準按鍵與軸，以及原始按鍵／軸可供取樣。
- SDL GUID 不是單台手把的永久序號；同型裝置以目前列舉順序區分。
- 沒有本規格使用的跨平台藍芽配對或傳輸型態查詢。不憑名稱猜測 USB／藍芽。

舊版 `frontend/ebiten/input.go` 將鍵盤與所有標準手把按鍵取聯集。
預設仍為全部手把加鍵盤；標準手把額外允許左搖桿控制方向，死區預設 20%。
未具標準配置的手把不猜測按鍵／軸用途，需在面板逐項指派。

## 3. 互動與狀態

- 滑鼠在視窗頂部 32 像素內顯示工具列，移開即隱藏；F1 可固定顯示／取消固定。
- 工具列覆蓋遊戲畫面，不改原有 viewport、視窗尺寸、4:3／16:9 比例或正常截圖幾何。
- 點選「手把設定」才開面板；面板開啟期間保持工具列可見。
- 設定時模擬 goroutine 在 field 邊界停止推進並清除送入的按鍵；不由 UI 執行緒讀寫機器。
- 關閉、取消與 ESC 放棄草稿並恢復遊戲，原按鍵需先放開；儲存成功才套用草稿。寫檔失敗留在面板並顯示錯誤。
- 退出程式仍須停止 goroutine、寫回記憶卡並關閉 session，不能因暫停卡住。
- 原有 -press 腳本仍取代實體輸入，沒有進入面板時正常重播不變。

## 4. 裝置與按鍵

### 4.1 裝置選擇

- 選項包含「自動（全部手把）」、「僅鍵盤」及目前每台已連線手把。
- 裝置識別用 SDL GUID，沒有 GUID 時以名稱作替代；同型裝置附第幾台的索引。
- 選定裝置離線時保留選擇與設定，提示未連線；不偷偷改用其他手把。鍵盤仍可操作遊戲。
- 模型配置依識別鍵保存，USB 與藍芽若回報不同 GUID 可分開設定。

### 4.2 指派與即時測試

- 14 個 PS1 動作：上、下、左、右、×、○、□、△、L1、R1、L2、R2、START、SELECT。
- 點選動作後，使用下一次新按下的手把按鈕或明顯移動的軸；先記錄起始狀態，不能把原本按住的輸入當成新指派。
- 標準配置優先保存標準按鍵；沒有可用標準按鍵才保存原始按鍵或軸與方向。
- 軸指派要求相對起始值變化至少 0.6，且目前絕對值至少 0.6，避免觸發器靜止值與雜訊誤指派。
- 等待指派時可按 Delete 或 Backspace 清除目前動作，方便交換兩個既有按鍵。
- 重複使用同一個明確綁定時提示衝突，不自動刪除另一個動作。
- 死區調整範圍 5%–90%，步進 5%；影響左搖桿方向與自訂軸綁定。
- 即時測試顯示原始按鈕／軸與套用草稿後的 PS1 動作，這些測試輸入不送進暫停中的遊戲。
- 自動模式可測試全部手把；按鍵重設需明確選定單台裝置。

## 5. 設定保存

沿用 `012`／`005` 的 settings.json 位置與原子寫檔，新增可選 gamepad 欄位。
缺欄位時載入預設配置；BIOS、光碟、HD、PGXP、寬螢幕與濾鏡欄位保留。

```json
{
  "gamepad": {
    "device": "auto",
    "dead_zone": 0.2,
    "profiles": {
      "<模型識別鍵>": {
        "bindings": [
          {"kind": "standard", "index": 0, "direction": 0}
        ]
      }
    }
  }
}
```

範例縮短 bindings，實際每份配置必須恰有 14 項，順序固定為 §4.2。
kind 為 none、standard、button、axis；只有 axis 的 direction 為 -1／1。
數值範圍及重複綁定在載入與儲存時驗證，壞設定回預設並回報原因。
選定裝置以模型鍵與序號編碼，顯示名稱不作輸入映射規則。
UI 草稿與生效配置需深複製，取消不能污染生效配置。

## 6. 介面與字型

- 面板以原型 960×752 的比例設計，視窗較小時等比縮放；文字與點擊區共同使用單一幾何來源。
- 欄位有固定文字安全區，動態裝置名稱、錯誤與輸入值超寬時依字型實測加省略號；不以畫布裁切代替排版。
- 使用自製位圖字圖集支援繁體中文與 ASCII，不修改原版遊戲字型。
- 字圖集由本機 Noto Sans CJK TC Regular Sans2.004 生成，輸入 TTC SHA-256
  `b76b0433203017ca80401b2ee0dd69350349871c4b19d504c34dbdd80541690a`。
- 字型依 [Noto CJK OFL-1.1](https://github.com/notofonts/noto-cjk/blob/main/Sans/LICENSE) 使用，
  原始著作權與完整授權保存於 `frontend/ebiten/ui-font-OFL.txt`；位圖字圖集維持該授權。
  生成工具 `tools/make_ui_font.py`、輸出 `ui-font.png`／`ui-font.json` 均在本 repo，
  不把主機字型或 Python 掛進正式 runtime。
  固定輸入快照保存在本機 `workplace/hdtex/ui-font-source.ttc` 與 `ui-font-source-copyright.txt`。
  一般封包附 `UI-FONT-LICENSE.txt`，macOS .app 與 AppImage 內也保存授權附件。

## 7. 驗收

| 層 | 判準 |
|---|---|
| 輸入 | 標準與原始按鍵、軸正負、死區邊界、鍵盤聯集、多裝置、指定與斷線，使用注入的裝置快照 |
| 設定 | 缺欄位預設、完整往返、配置深複製、非法數值／綁定、取消不生效、寫檔失敗不套用 |
| 互動 | 懸停、F1、ESC、指派只收新輸入、按住與觸發器靜止值不誤綁、重複衝突 |
| 暫停 | 面板開啟後在 field 邊界停住、關閉繼續、按鍵不洩漏、暫停時退出仍正常存卡 |
| GUI | Linux Docker／xvfb 的實際工具列與面板截圖，標準／非標準／未連線及長中文字串；與定版原型比較 |
| 幾何 | 工具列隱藏時維持原 viewport；面板可讀、文字不越界、點擊區與顯示一致 |
| 整合 | 既有前端測試與 HD／機器測試通過；Linux／Windows／macOS 交叉編譯沿用 005 |

注入的手把快照與 GUI fixture 只證明程式路徑，不能當成實體 USB／藍芽試玩。
實體手把連線、斷線重連與各平台驅動相容性需玩家真機確認，不虛構通過紀錄。
原型、研究輸出留本機 workplace；定版原型與可散布的 UI 字圖集可入版控。

本規格入口為 `000-index.md`，玩家說明更新 `docs/play.md`，歷程追加唯一 `WORKLOG.md`。

## 8. 驗收結果（2026-10-18）

| 範圍 | 實際結果與本機收據 |
|---|---|
| 輸入與設定 | 標準／原始映射、死區、多裝置、離線、草稿隔離、清除與交換按鍵、衝突、寫檔失敗與往返保留其他欄位通過；`gamepad-tests.log`、`gamepad-final-tests.log` |
| 暫停／恢復 | 實際 BIOS 執行到 field 邊界，暫停期間畫面發布計數不增加，恢復後繼續；關閉後輸入需先放開；`gamepad-bios-pause.log`、`gamepad-regression.log` |
| 正常視窗操作 | 最終 Linux 執行檔經滑鼠／鍵盤操作確認 F1、懸停、裝置、死區、儲存、持久設定與正常關窗；`gamepad-player-receipt.log`、`gamepad-player-inputs.sha256` |
| 畫面與幾何 | 實際 Ebiten 擷取隱藏、工具列、標準／原始／離線裝置與長中文名稱；已目視比對原型，文字範圍、兩行即時診斷的實際墨跡與固定字串字圖覆蓋測試通過；`gamepad-*.png`、`gamepad-window-captures.sha256` |
| 回歸與競態 | `xvfb-run -a go test -p 2 ./...` 與前端／設定 `-race` 通過；`gamepad-regression.log`、`gamepad-race.log` |
| 三平台 | Linux、Windows 與 macOS universal 編譯通過；macOS 四道結構檢查通過；`gamepad-linux-windows-build.log`、`gamepad-macos-build.log`、`gamepad-macos-verify.log` |

收據均在 `workplace/hdtex/`，總入口為 `implementation-verification.json`，
重跑命令為 `implementation-commands.txt`，程式與產物雜湊為 `implementation-evidence.sha256`。
Linux 開發執行檔 `bin/psgolem` 已更新；既有 `local-full/v0.5.0/` 未重新組裝。
封包腳本已加字型授權附件，但本輪沒有建立或發布新封包。

手把輸入來自注入快照；正常視窗測試使用滑鼠與鍵盤，實際 BIOS 測試另驗證暫停機制。
沒有實體 USB／藍芽連線與重連測試，沒有 Windows／macOS 真機功能驗收，不據此標 CONFORMED。
