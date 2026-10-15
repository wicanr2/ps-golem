// psexe 在真 BIOS 上側載一個 PS-EXE，跑固定週期後印出 TTY（docs/spec/006-minimal-machine.md §8）。
//
//	psexe -bios /orig/bios/SCPH5500.BIN -exe test.exe -cycles 200000000
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/wicanr2/ps-golem/internal/disc"
	"github.com/wicanr2/ps-golem/machine"
)

func main() {
	biosPath := flag.String("bios", "", "BIOS 檔（512 KB）")
	exePath := flag.String("exe", "", "PS-EXE 檔")
	cycles := flag.Uint64("cycles", 200_000_000, "側載之後執行的 CPU 週期數")
	boot := flag.Uint64("boot-cycles", 400_000_000, "等 BIOS 進入 shell 的週期上限")
	vramOut := flag.String("vram", "", "結束時把 VRAM 存成 PNG（docs/spec/007 §2.1 的展開方式）")
	state := flag.Bool("state", false, "結束時印出 PC、GPUSTAT 與部分暫存器到 stderr")
	ramOut := flag.String("ram", "", "結束時把 2 MB 主記憶體存成檔案（除錯用）")
	cuePath := flag.String("cue", "", "光碟映像的 cue 檔（docs/spec/009）")
	flag.Parse()

	ramPath = *ramOut
	cue = *cuePath
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
	if os.Getenv("PSGOLEM_TRACE_DMA") != "" {
		machine.DMATrace = func(off, v, dicr uint32) {
			fmt.Fprintf(os.Stderr, "DMA %02x <- %08x  DICR=%08x PC=%08x cyc=%d\n", off, v, dicr, m.CPU.PC(), m.Cycles())
		}
	}
	if exePath == "" {
		// 不側載：只跑 BIOS（看開機畫面用）。
		m.Run(cycles)
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
	m.Run(cycles)
	return finish(m, vramOut, state)
}

var ramPath, cue string

func finish(m *machine.Machine, vramOut string, state bool) error {
	fmt.Print(m.TTY())
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
	if vramOut != "" {
		if err := os.WriteFile(vramOut, machine.VRAMPNG(m.VRAM()), 0o644); err != nil {
			return err
		}
	}
	return nil
}
