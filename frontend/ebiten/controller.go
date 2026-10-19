package ebiten

import (
	"fmt"
	"math"
	"sort"

	eb "github.com/hajimehoshi/ebiten/v2"
	"github.com/wicanr2/ps-golem/internal/sio"
)

// GamepadSettings 只影響前端輸入。設定與介面契約：020-gamepad-toolbar。
type GamepadSettings struct {
	Device   string                    `json:"device"`
	DeadZone float64                   `json:"dead_zone"`
	Profiles map[string]GamepadProfile `json:"profiles"`
}

type GamepadProfile struct {
	Bindings []GamepadBinding `json:"bindings"`
}
type GamepadBinding struct {
	Kind      string `json:"kind"`
	Index     int    `json:"index"`
	Direction int    `json:"direction"`
}

var actionNames = []string{"上", "下", "左", "右", "×", "○", "□", "△", "L1", "R1", "L2", "R2", "START", "SELECT"}

func DefaultGamepadSettings() GamepadSettings {
	return GamepadSettings{Device: "auto", DeadZone: 0.2, Profiles: map[string]GamepadProfile{}}
}

// Clone 隔離設定草稿，取消不能改到生效中的 slice 或 map。
func (s GamepadSettings) Clone() GamepadSettings {
	out := s
	out.Profiles = make(map[string]GamepadProfile, len(s.Profiles))
	for k, p := range s.Profiles {
		out.Profiles[k] = GamepadProfile{Bindings: append([]GamepadBinding(nil), p.Bindings...)}
	}
	return out
}

func (s GamepadSettings) Validate() error {
	if s.Device == "" || len(s.Device) > 512 {
		return fmt.Errorf("裝置識別不合法")
	}
	if math.IsNaN(s.DeadZone) || math.IsInf(s.DeadZone, 0) || s.DeadZone < 0.05 || s.DeadZone > 0.9 {
		return fmt.Errorf("死區必須介於 5%% 與 90%%")
	}
	for k, p := range s.Profiles {
		if k == "" || len(k) > 256 || len(p.Bindings) != len(padMap) {
			return fmt.Errorf("手把配置 %q 需要 14 個動作", k)
		}
		seen := map[GamepadBinding]bool{}
		for i, b := range p.Bindings {
			switch b.Kind {
			case "none":
				if b.Index != 0 || b.Direction != 0 {
					return fmt.Errorf("未指派動作含多餘數值")
				}
				continue
			case "standard":
				if b.Index < 0 || b.Index > int(eb.StandardGamepadButtonMax) || b.Direction != 0 {
					return fmt.Errorf("標準按鍵範圍不合法")
				}
			case "button":
				if b.Index < 0 || b.Index >= 256 || b.Direction != 0 {
					return fmt.Errorf("原始按鍵範圍不合法")
				}
			case "axis":
				if b.Index < 0 || b.Index >= 64 || (b.Direction != -1 && b.Direction != 1) {
					return fmt.Errorf("搖桿軸或方向不合法")
				}
			default:
				return fmt.Errorf("未知綁定類型 %q", b.Kind)
			}
			if seen[b] {
				return fmt.Errorf("動作 %s 使用重複綁定", actionNames[i])
			}
			seen[b] = true
		}
	}
	return nil
}

type padSnapshot struct {
	ID                  eb.GamepadID
	Name, Model, Device string
	Standard            bool
	Buttons             map[eb.StandardGamepadButton]bool
	RawButtons          []bool
	Axes                []float64
	LeftX, LeftY        float64
}

func connectedPads() []padSnapshot {
	ids := eb.AppendGamepadIDs(nil)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	counts := map[string]int{}
	result := make([]padSnapshot, 0, len(ids))
	for _, id := range ids {
		p := padSnapshot{ID: id, Name: eb.GamepadName(id), Standard: eb.IsStandardGamepadLayoutAvailable(id), Buttons: map[eb.StandardGamepadButton]bool{}}
		p.Model = "sdl:" + eb.GamepadSDLID(id)
		if p.Model == "sdl:" {
			p.Model = "name:" + p.Name
		}
		p.Device = fmt.Sprintf("%s#%d", p.Model, counts[p.Model])
		counts[p.Model]++
		for b := eb.StandardGamepadButton(0); b <= eb.StandardGamepadButtonMax; b++ {
			p.Buttons[b] = eb.IsStandardGamepadButtonPressed(id, b)
		}
		for i := 0; i < min(eb.GamepadButtonCount(id), 256); i++ {
			p.RawButtons = append(p.RawButtons, eb.IsGamepadButtonPressed(id, eb.GamepadButton(i)))
		}
		for i := 0; i < min(eb.GamepadAxisCount(id), 64); i++ {
			p.Axes = append(p.Axes, eb.GamepadAxisValue(id, eb.GamepadAxisType(i)))
		}
		p.LeftX = eb.StandardGamepadAxisValue(id, eb.StandardGamepadAxisLeftStickHorizontal)
		p.LeftY = eb.StandardGamepadAxisValue(id, eb.StandardGamepadAxisLeftStickVertical)
		result = append(result, p)
	}
	return result
}

func defaultPadProfile(standard bool) GamepadProfile {
	p := GamepadProfile{Bindings: make([]GamepadBinding, len(padMap))}
	for i, m := range padMap {
		p.Bindings[i] = GamepadBinding{Kind: "none"}
		if standard {
			p.Bindings[i] = GamepadBinding{Kind: "standard", Index: int(m.btn)}
		}
	}
	return p
}

func (s GamepadSettings) profile(p padSnapshot) GamepadProfile {
	if profile, ok := s.Profiles[p.Model]; ok && len(profile.Bindings) == len(padMap) {
		return profile
	}
	return defaultPadProfile(p.Standard)
}

func bindingDown(b GamepadBinding, p padSnapshot, dead float64) bool {
	switch b.Kind {
	case "standard":
		return p.Buttons[eb.StandardGamepadButton(b.Index)]
	case "button":
		return b.Index >= 0 && b.Index < len(p.RawButtons) && p.RawButtons[b.Index]
	case "axis":
		return b.Index >= 0 && b.Index < len(p.Axes) && p.Axes[b.Index]*float64(b.Direction) >= dead
	}
	return false
}

func controllerButtons(s GamepadSettings, pads []padSnapshot) uint16 {
	if s.Device == "keyboard" {
		return 0
	}
	var out uint16
	for _, p := range pads {
		if s.Device != "auto" && s.Device != p.Device {
			continue
		}
		profile := s.profile(p)
		for i, b := range profile.Bindings {
			if bindingDown(b, p, s.DeadZone) {
				out |= padMap[i].ps
			}
		}
		if p.Standard {
			if p.LeftX <= -s.DeadZone {
				out |= sio.BtnLeft
			}
			if p.LeftX >= s.DeadZone {
				out |= sio.BtnRight
			}
			if p.LeftY <= -s.DeadZone {
				out |= sio.BtnUp
			}
			if p.LeftY >= s.DeadZone {
				out |= sio.BtnDown
			}
		}
	}
	return out
}

func findPad(pads []padSnapshot, device string) (padSnapshot, bool) {
	for _, p := range pads {
		if p.Device == device {
			return p, true
		}
	}
	return padSnapshot{}, false
}

func freshBinding(before, now padSnapshot) (GamepadBinding, bool) {
	for b := eb.StandardGamepadButton(0); b <= eb.StandardGamepadButtonMax; b++ {
		if now.Buttons[b] && !before.Buttons[b] {
			return GamepadBinding{Kind: "standard", Index: int(b)}, true
		}
	}
	for i, v := range now.RawButtons {
		if v && (i >= len(before.RawButtons) || !before.RawButtons[i]) {
			return GamepadBinding{Kind: "button", Index: i}, true
		}
	}
	for i, v := range now.Axes {
		if i >= len(before.Axes) || math.Abs(v-before.Axes[i]) < 0.6 || math.Abs(v) < 0.6 {
			continue
		}
		dir := 1
		if v < 0 {
			dir = -1
		}
		return GamepadBinding{Kind: "axis", Index: i, Direction: dir}, true
	}
	return GamepadBinding{}, false
}

func bindingName(b GamepadBinding) string {
	if b.Kind == "none" {
		return "未指派"
	}
	if b.Kind == "button" {
		return fmt.Sprintf("按鈕 %d", b.Index)
	}
	if b.Kind == "axis" {
		sign := "+"
		if b.Direction < 0 {
			sign = "-"
		}
		return fmt.Sprintf("軸 %d %s", b.Index, sign)
	}
	names := map[eb.StandardGamepadButton]string{
		eb.StandardGamepadButtonLeftTop: "十字鍵上", eb.StandardGamepadButtonLeftBottom: "十字鍵下",
		eb.StandardGamepadButtonLeftLeft: "十字鍵左", eb.StandardGamepadButtonLeftRight: "十字鍵右",
		eb.StandardGamepadButtonRightBottom: "A／底部", eb.StandardGamepadButtonRightRight: "B／右側",
		eb.StandardGamepadButtonRightLeft: "X／左側", eb.StandardGamepadButtonRightTop: "Y／上方",
		eb.StandardGamepadButtonFrontTopLeft: "LB", eb.StandardGamepadButtonFrontTopRight: "RB",
		eb.StandardGamepadButtonFrontBottomLeft: "LT", eb.StandardGamepadButtonFrontBottomRight: "RT",
		eb.StandardGamepadButtonCenterRight: "Menu", eb.StandardGamepadButtonCenterLeft: "View",
	}
	if name := names[eb.StandardGamepadButton(b.Index)]; name != "" {
		return name
	}
	return fmt.Sprintf("標準按鈕 %d", b.Index)
}
