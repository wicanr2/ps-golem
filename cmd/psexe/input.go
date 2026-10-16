package main

import "github.com/wicanr2/ps-golem/internal/padscript"

// press 是一項按鍵（internal/padscript）。
type press = padscript.Press

// parsePresses 解析 "FIELD:BTN[+BTN][:DUR],..."，DUR 預設 5 個 field。
func parsePresses(s string) ([]press, error) { return padscript.Parse(s) }

// buttonsAt 回傳第 f 個 field 應按住的鍵。
func buttonsAt(ps []press, f uint64) uint16 { return padscript.At(ps, f) }
