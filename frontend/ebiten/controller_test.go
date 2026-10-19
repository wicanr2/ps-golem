package ebiten

import (
	"errors"
	"image"
	"math"
	"strings"
	"testing"
	"time"

	eb "github.com/hajimehoshi/ebiten/v2"
	"github.com/wicanr2/ps-golem/internal/sio"
	"github.com/wicanr2/ps-golem/machine"
)

func testPad() padSnapshot {
	return padSnapshot{Device: "test#0", Model: "test", Name: "測試手把", Standard: true, Buttons: map[eb.StandardGamepadButton]bool{}}
}

func TestControllerSelectionAndDeadZone(t *testing.T) {
	s := DefaultGamepadSettings()
	a, b := testPad(), testPad()
	b.Device = "other#0"
	b.Model = "other"
	a.LeftX = 0.199
	b.Buttons[eb.StandardGamepadButtonRightBottom] = true
	if got := controllerButtons(s, []padSnapshot{a, b}); got != sio.BtnCross {
		t.Fatalf("auto union=%04x", got)
	}
	s.Device = a.Device
	if controllerButtons(s, []padSnapshot{a, b}) != 0 {
		t.Fatal("unselected device or dead zone leaked")
	}
	a.LeftX = 0.2
	a.LeftY = -0.2
	if got := controllerButtons(s, []padSnapshot{a, b}); got != (sio.BtnRight | sio.BtnUp) {
		t.Fatalf("boundary=%04x", got)
	}
	if controllerButtons(s, []padSnapshot{b}) != 0 {
		t.Fatal("disconnected selected device used another pad")
	}
	s.Device = "keyboard"
	if controllerButtons(s, []padSnapshot{a, b}) != 0 {
		t.Fatal("keyboard mode")
	}
}

func TestControllerRawBindingsAndUnknownLayout(t *testing.T) {
	s := DefaultGamepadSettings()
	p := testPad()
	p.Standard = false
	p.RawButtons = []bool{false, false, true}
	p.Axes = []float64{-0.5}
	if controllerButtons(s, []padSnapshot{p}) != 0 {
		t.Fatal("guessed unknown layout")
	}
	profile := defaultPadProfile(false)
	profile.Bindings[4] = GamepadBinding{Kind: "button", Index: 2}
	profile.Bindings[2] = GamepadBinding{Kind: "axis", Index: 0, Direction: -1}
	s.Profiles[p.Model] = profile
	if got := controllerButtons(s, []padSnapshot{p}); got != (sio.BtnCross | sio.BtnLeft) {
		t.Fatalf("custom=%04x", got)
	}
	s.DeadZone = 0.6
	if got := controllerButtons(s, []padSnapshot{p}); got != sio.BtnCross {
		t.Fatalf("dead zone custom=%04x", got)
	}
}

func TestControllerDraftIsolationAndValidation(t *testing.T) {
	s := DefaultGamepadSettings()
	s.Profiles["test"] = defaultPadProfile(true)
	draft := s.Clone()
	draft.Profiles["test"].Bindings[4] = GamepadBinding{Kind: "button", Index: 2}
	if s.Profiles["test"].Bindings[4].Kind != "standard" {
		t.Fatal("draft changed active profile")
	}
	if err := draft.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []float64{0, 0.91, math.NaN(), math.Inf(1)} {
		x := s.Clone()
		x.DeadZone = bad
		if x.Validate() == nil {
			t.Fatal("accepted invalid dead zone")
		}
	}
	x := s.Clone()
	x.Profiles["test"].Bindings[4] = x.Profiles["test"].Bindings[5]
	if x.Validate() == nil {
		t.Fatal("duplicate binding")
	}
	x = s.Clone()
	x.Profiles["test"] = GamepadProfile{Bindings: []GamepadBinding{{Kind: "none"}}}
	if x.Validate() == nil {
		t.Fatal("partial profile")
	}
}

func TestControllerCaptureFreshInputs(t *testing.T) {
	before := testPad()
	before.Standard = false
	before.RawButtons = []bool{true}
	before.Axes = []float64{-1, 0}
	now := before
	if _, ok := freshBinding(before, now); ok {
		t.Fatal("held input or resting trigger captured")
	}
	now.Axes = []float64{-0.8, 0.1}
	if _, ok := freshBinding(before, now); ok {
		t.Fatal("noise captured")
	}
	now.Axes = []float64{1, 0}
	binding, ok := freshBinding(before, now)
	if !ok || binding != (GamepadBinding{Kind: "axis", Index: 0, Direction: 1}) {
		t.Fatalf("trigger: %+v %v", binding, ok)
	}
	before.RawButtons = []bool{false}
	now.Axes = []float64{-1, 0}
	now.RawButtons = []bool{true}
	binding, ok = freshBinding(before, now)
	if !ok || binding.Kind != "button" {
		t.Fatal("new raw press missing")
	}
	now.Buttons = map[eb.StandardGamepadButton]bool{eb.StandardGamepadButtonRightBottom: true}
	binding, ok = freshBinding(before, now)
	if !ok || binding.Kind != "standard" {
		t.Fatal("standard button priority")
	}
}

func TestControllerToolbarVisibilityAndGeometry(t *testing.T) {
	for _, p := range []image.Point{{10, 32}, {-1, 0}, {960, 0}, {10, 100}} {
		if toolbarShown(false, false, p.X, p.Y, 960) {
			t.Fatal("unexpected hover", p)
		}
	}
	if !toolbarShown(false, false, 10, 31, 960) || !toolbarShown(true, false, 10, 100, 960) || !toolbarShown(false, true, 10, 100, 960) {
		t.Fatal("toolbar activation")
	}
	for _, size := range []image.Point{{960, 720}, {640, 480}, {320, 240}, {1280, 720}} {
		x, y, scale := panelGeometry(size.X, size.Y)
		if x < 0 || y < 0 || x+644*scale > float64(size.X)+1 || y+588*scale > float64(size.Y)+1 {
			t.Fatal("panel overflow", size)
		}
		for i := range actionNames {
			r := panelRow(i)
			if r.Min.X < 0 || r.Max.X > 644 || r.Min.Y < 0 || r.Max.Y > 588 {
				t.Fatal("row geometry")
			}
		}
	}
}

func TestControllerSaveFailureAndCancel(t *testing.T) {
	g := &Game{controller: DefaultGamepadSettings()}
	g.openControllerPanel()
	g.padUI.draft.Device = "keyboard"
	g.cfg.SaveGamepad = func(GamepadSettings) error { return errors.New("disk full") }
	g.saveControllerPanel()
	if !g.padUI.open || g.controller.Device != "auto" || g.padUI.status == "" {
		t.Fatal("failed save applied")
	}
	g.closeControllerPanel()
	if g.controller.Device != "auto" {
		t.Fatal("cancel applied draft")
	}
	g.openControllerPanel()
	g.padUI.draft.Device = "keyboard"
	var saved GamepadSettings
	g.cfg.SaveGamepad = func(s GamepadSettings) error { saved = s; return nil }
	g.saveControllerPanel()
	if g.padUI.open || g.controller.Device != "keyboard" || saved.Device != "keyboard" {
		t.Fatal("successful save missing")
	}
}

func TestControllerPausedEmulatorCanExit(t *testing.T) {
	// 只驗證暫停／退出排程，不執行 CPU，不以代製 BIOS 代替原版。
	e := newEmulator(&machine.Machine{}, Config{}, nil, nil)
	e.paused.Store(true)
	e.start()
	done := make(chan struct{})
	go func() { e.halt(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("paused emulator could not stop")
	}
}

func TestControllerSwapBindingsAndRawTest(t *testing.T) {
	p := testPad()
	g := &Game{controller: DefaultGamepadSettings(), pads: []padSnapshot{p}}
	g.controller.Device = p.Device
	g.openControllerPanel()
	cross := GamepadBinding{Kind: "standard", Index: int(eb.StandardGamepadButtonRightBottom)}
	circle := GamepadBinding{Kind: "standard", Index: int(eb.StandardGamepadButtonRightRight)}
	g.padUI.capture = 4
	g.assignControllerBinding(circle)
	if g.padUI.capture != 4 || g.padUI.draft.profile(p).Bindings[4] != cross {
		t.Fatal("conflict overwrote existing binding")
	}
	g.assignControllerBinding(GamepadBinding{Kind: "none"})
	if g.padUI.capture != -1 || g.padUI.draft.profile(p).Bindings[4].Kind != "none" {
		t.Fatal("clear binding failed")
	}
	g.padUI.capture = 5
	g.assignControllerBinding(cross)
	g.padUI.capture = 4
	g.assignControllerBinding(circle)
	if err := g.padUI.draft.Validate(); err != nil {
		t.Fatal(err)
	}
	if g.controller.profile(p).Bindings[4] != cross {
		t.Fatal("swap modified active configuration before save")
	}
	g.saveControllerPanel()
	p.Buttons[crossButton()] = true
	if got := controllerButtons(g.controller, []padSnapshot{p}); got != sio.BtnCircle {
		t.Fatalf("swapped input = %04x", got)
	}
	p.Standard = false
	p.RawButtons = []bool{false, false, true}
	p.Axes = []float64{-1, 0.75}
	text := rawControllerInput(p)
	if !strings.Contains(text, "B2") || strings.Contains(text, "B0") || !strings.Contains(text, "A0:-1.00") || !strings.Contains(text, "A1:+0.75") {
		t.Fatalf("raw input missing values: %s", text)
	}
}

func crossButton() eb.StandardGamepadButton { return eb.StandardGamepadButtonRightBottom }
