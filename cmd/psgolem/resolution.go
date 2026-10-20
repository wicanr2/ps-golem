package main

import (
	"fmt"
	"strconv"
	"strings"
)

// 012 §11：只決定呈現尺寸，不變更HD倍率或寬螢幕模式。
func resolveResolution(cli string, explicit bool, saved string) (name string, width, height int, err error) {
	if !explicit {
		cli = saved
	}
	value := strings.ToLower(strings.TrimSpace(cli))
	if value == "" || value == "auto" {
		return "", 0, 0, nil
	}
	w, h, ok := strings.Cut(value, "x")
	if ok {
		width, err = strconv.Atoi(w)
		if err == nil {
			height, err = strconv.Atoi(h)
		}
	}
	if !ok || err != nil || width < 320 || width > 7680 || height < 240 || height > 4320 {
		return "", 0, 0, fmt.Errorf("resolution須為寬x高，寬320–7680、高240–4320，例如1920x1080；auto清除")
	}
	return fmt.Sprintf("%dx%d", width, height), width, height, nil
}
