// psexe 在真 BIOS 上側載一個 PS-EXE，跑固定週期後印出 TTY（docs/spec/006-minimal-machine.md §8）。
//
//	psexe -bios /orig/bios/SCPH5500.BIN -exe test.exe -cycles 200000000
package main

import (
	"flag"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"runtime/pprof"

	"github.com/wicanr2/ps-golem/internal/disc"
	"github.com/wicanr2/ps-golem/machine"
)

func main() {
	biosPath := flag.String("bios", "", "BIOS 檔（512 KB）")
	exePath := flag.String("exe", "", "PS-EXE 檔")
	cycles := flag.Uint64("cycles", 200_000_000, "側載之後執行的 CPU 週期數")
	boot := flag.Uint64("boot-cycles", 400_000_000, "等 BIOS 進入 shell 的週期上限")
	vramOut := flag.String("vram", "", "結束時把 VRAM 存成 PNG（docs/spec/007 §2.1 的展開方式）")
	screenFlag := flag.String("screen", "", "結束時把目前顯示的畫面存成 PNG（007 §8）")
	shotsFlag := flag.String("shots", "", "每隔 -shot-every 個 field 把畫面存到這個目錄")
	shotEvery := flag.Uint64("shot-every", 60, "-shots 的間隔（field 數）")
	exact := flag.Bool("exact", false, "每條指令都同步週邊（等價性比對用，較慢）")
	digest := flag.Bool("digest", false, "結束時印出機器狀態摘要")
	cpuProf := flag.String("cpuprofile", "", "把 CPU profile 寫到這個檔案（效能分析用）")
	stopField := flag.Uint64("fields", 0, "大於 0 時跑到第這麼多次 VBlank 就停（不超過 -cycles）")
	state := flag.Bool("state", false, "結束時印出 PC、GPUSTAT 與部分暫存器到 stderr")
	ramOut := flag.String("ram", "", "結束時把 2 MB 主記憶體存成檔案（除錯用）")
	cuePath := flag.String("cue", "", "光碟映像的 cue 檔（docs/spec/009）")
	cardPath := flag.String("card", "", "1P 記憶卡檔案（128 KB）；不存在就用空白卡，結束時有寫入就存回")
	pressSpec := flag.String("press", "", "按鍵腳本 FIELD:BTN[+BTN][:DUR],...（FIELD 為 VBlank 次數）")
	flag.Parse()
	if *cpuProf != "" {
		f, err := os.Create(*cpuProf)
		if err != nil {
			fmt.Fprintln(os.Stderr, "psexe:", err)
			os.Exit(1)
		}
		_ = pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	ramPath = *ramOut
	cue = *cuePath
	card = *cardPath
	screenOut, shotsDir, shotStep, fieldLimit = *screenFlag, *shotsFlag, *shotEvery, *stopField
	exactTiming, printDigest = *exact, *digest
	var err error
	if presses, err = parsePresses(*pressSpec); err != nil {
		fmt.Fprintln(os.Stderr, "psexe:", err)
		os.Exit(2)
	}
	if err := run(*biosPath, *exePath, *boot, *cycles, *vramOut, *state); err != nil {
		fmt.Fprintln(os.Stderr, "psexe:", err)
		os.Exit(1)
	}
}

func run(biosPath, exePath string, boot, cycles uint64, vramOut string, state bool) error {
	bios, err := os.ReadFile(biosPath)
	if err != nil {
		return err
	}
	m, err := machine.New(bios)
	if err != nil {
		return err
	}
	m.ExactTiming = exactTiming
	if cue != "" {
		d, err := disc.Open(cue)
		if err != nil {
			return err
		}
		if d.Fallback != "" {
			fmt.Fprintln(os.Stderr, "psexe：", d.Fallback)
		}
		fmt.Fprintf(os.Stderr, "psexe：光碟 %d 軌，lead-out LBA %d，區域 %q\n", len(d.Tracks), d.LeadOut(), d.Region())
		m.InsertDisc(d)
	}
	if card != "" {
		if b, err := os.ReadFile(card); err == nil {
			if !m.Card1.Load(b) {
				return fmt.Errorf("記憶卡 %s 不是 128 KB", card)
			}
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	if n := os.Getenv("PSGOLEM_TRACE_CD"); n != "" {
		var limit, count int
		fmt.Sscan(n, &limit)
		machine.CDTrace = func(w bool, reg uint32, idx uint8, v uint8) {
			if !w && (reg == 0 || reg == 3) {
				return // 輪詢狀態與中斷旗標的讀取太多，不記
			}
			if w && reg == 0 {
				return // 切換 bank 也不記
			}
			if count < limit {
				d := "R"
				if w {
					d = "W"
				}
				fmt.Fprintf(os.Stderr, "CD %s reg%d %02x  PC=%08x cyc=%d\n", d, reg, v, m.CPU.PC(), m.Cycles())
			}
			count++
		}
	}
	if os.Getenv("PSGOLEM_TRACE_IRQ") != "" {
		machine.IRQTrace = func(ack bool, bit uint) {
			if ack {
				fmt.Fprintf(os.Stderr, "IRQ ack %03x  PC=%08x SR=%08x cyc=%d\n", bit, m.CPU.PC(), m.CPU.COP0(12), m.Cycles())
			} else if bit != 0 {
				fmt.Fprintf(os.Stderr, "IRQ raise %d  PC=%08x SR=%08x cyc=%d\n", bit, m.CPU.PC(), m.CPU.COP0(12), m.Cycles())
			}
		}
	}
	if os.Getenv("PSGOLEM_TRACE_DMA") != "" {
		machine.DMATrace = func(off, v, dicr uint32) {
			fmt.Fprintf(os.Stderr, "DMA %02x <- %08x  DICR=%08x PC=%08x cyc=%d\n", off, v, dicr, m.CPU.PC(), m.Cycles())
		}
	}
	if exePath == "" {
		// 不側載：只跑 BIOS（或以光碟開機）。
		runCycles(m, cycles)
		return finish(m, vramOut, state)
	}
	exe, err := os.ReadFile(exePath)
	if err != nil {
		return err
	}
	if err := m.RunUntilPC(machine.ShellEntry, boot); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "psexe：第 %d 週期進入 shell，側載 %s\n", m.Cycles(), exePath)
	if err := m.SideloadEXE(exe); err != nil {
		return err
	}
	if n := os.Getenv("PSGOLEM_TRACE_GP0"); n != "" {
		var limit, count int
		fmt.Sscan(n, &limit)
		m.GPU.Trace = func(v uint32) {
			if count < limit {
				fmt.Fprintf(os.Stderr, "GP0 %08x  PC=%08x cyc=%d\n", v, m.CPU.PC(), m.Cycles())
			}
			count++
		}
	}
	runCycles(m, cycles)
	return finish(m, vramOut, state)
}

var (
	ramPath, cue, card       string
	screenOut, shotsDir      string
	shotStep, fieldLimit     uint64
	exactTiming, printDigest bool
	presses                  []press
)

func writeScreen(m *machine.Machine, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, m.GPU.Screen())
}

// runCycles 執行 n 個週期，每個 field 依按鍵腳本更新手把。
func runCycles(m *machine.Machine, n uint64) {
	end := m.Cycles() + n
	for m.Cycles() < end && (fieldLimit == 0 || m.Field() < fieldLimit) {
		m.Pad1.Buttons = buttonsAt(presses, m.Field())
		f := m.Field() + 1
		left := end - m.Cycles()
		if m.RunUntilField(f, left) && shotsDir != "" && shotStep > 0 && f%shotStep == 0 {
			if err := writeScreen(m, filepath.Join(shotsDir, fmt.Sprintf("f%06d.png", f))); err != nil {
				fmt.Fprintln(os.Stderr, "psexe:", err)
			}
		}
	}
}

func finish(m *machine.Machine, vramOut string, state bool) error {
	if printDigest {
		fmt.Println("digest", m.Digest())
	}
	fmt.Print(m.TTY())
	if card != "" && m.Card1.Dirty {
		if err := os.WriteFile(card, m.Card1.Data[:], 0o644); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "psexe：記憶卡已寫回", card)
	}
	if ramPath != "" {
		if err := os.WriteFile(ramPath, m.ReadRAM(0, 2<<20), 0o644); err != nil {
			return err
		}
	}
	if state {
		c := m.CPU
		fmt.Fprintf(os.Stderr, "PC=%08x RA=%08x SP=%08x V0=%08x A0=%08x SR=%08x CAUSE=%08x GPUSTAT=%08x fields=%d\n%s\n",
			c.PC(), c.GPR(31), c.GPR(29), c.GPR(2), c.GPR(4), c.COP0(12), c.COP0(13), m.GPU.Status(), m.Field(), m.GPU.DebugState()+"\n"+m.DebugIO())
	}
	if screenOut != "" {
		if err := writeScreen(m, screenOut); err != nil {
			return err
		}
	}
	if vramOut != "" {
		if err := os.WriteFile(vramOut, machine.VRAMPNG(m.VRAM()), 0o644); err != nil {
			return err
		}
	}
	return nil
}
