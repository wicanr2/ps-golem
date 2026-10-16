// Package early 在 Ebiten 初始化之前處理 -version。
//
// Ebiten 的 internal/ui 在套件 init 就開 GLFW，Linux 沒有 DISPLAY 時直接 panic，
// 輪不到 main 解析旗標。Go 依匯入路徑排序、相依先初始化（Go 1.21 起的規則）：
// internal/ui 相依 golang.org/x/sys 與 x/sync，排在本套件之後，所以本套件的 init 先跑。
// 只檢查 -version 一個旗標；其他旗標仍由 main 解析。
package early

import (
	"fmt"
	"os"
)

// Version 由 tools/build_dist.sh 以 -ldflags "-X github.com/wicanr2/ps-golem/cmd/psgolem/early.Version=..." 寫入。
var Version = "dev"

func init() {
	for _, a := range os.Args[1:] {
		switch a {
		case "-version", "--version", "-version=true", "--version=true":
			fmt.Println(Version)
			os.Exit(0)
		}
	}
}
