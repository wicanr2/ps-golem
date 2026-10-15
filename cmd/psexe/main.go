// psexe 在真 BIOS 上側載一個 PS-EXE，跑固定週期後印出 TTY（docs/spec/006-minimal-machine.md §8）。
//
//	psexe -bios /orig/bios/SCPH5500.BIN -exe test.exe -cycles 200000000
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/wicanr2/ps-golem/machine"
)

func main() {
	biosPath := flag.String("bios", "", "BIOS 檔（512 KB）")
	exePath := flag.String("exe", "", "PS-EXE 檔")
	cycles := flag.Uint64("cycles", 200_000_000, "側載之後執行的 CPU 週期數")
	boot := flag.Uint64("boot-cycles", 400_000_000, "等 BIOS 進入 shell 的週期上限")
	flag.Parse()

	if err := run(*biosPath, *exePath, *boot, *cycles); err != nil {
		fmt.Fprintln(os.Stderr, "psexe:", err)
		os.Exit(1)
	}
}

func run(biosPath, exePath string, boot, cycles uint64) error {
	bios, err := os.ReadFile(biosPath)
	if err != nil {
		return err
	}
	exe, err := os.ReadFile(exePath)
	if err != nil {
		return err
	}
	m, err := machine.New(bios)
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
	m.Run(cycles)
	fmt.Print(m.TTY())
	return nil
}
