// sotnbot 以 Clone 搜尋自動遊玩（docs/spec/015-sotn-autoplay.md §5）。
// 從腳本跑到起點後，每個決策點平行試所有動作，各自向前看一段，取分數最高的動作執行。
//
//	sotnbot -bios B -cue C -script S -at F -slots 80 -out DIR [-max-fields N]
//
// 結果：DIR/script.txt（從開機起的 psexe -press 腳本，每個決策點更新）、DIR/shots/。
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/wicanr2/ps-golem/apps/sotn"
	"github.com/wicanr2/ps-golem/machine"
)

type step struct {
	b uint16
	n int
}

type action struct {
	name string
	seq  []step
}

func btn(s string) uint16 {
	b, err := sotn.ParseButtons(s)
	if err != nil {
		panic(err)
	}
	return b
}

// phase 是一段目標：動作集合、向前看的長度、評分與結束條件。
type phase struct {
	actions   []action
	lookahead int
	score     func(before, after *machine.Machine) float64
	done      func(m *machine.Machine) bool
}

func jumpWhip(dir string) action {
	seq := []step{}
	if dir != "" {
		seq = append(seq, step{btn(dir), 2})
	}
	seq = append(seq, step{btn("CROSS"), 8}, step{btn("CROSS+SQUARE"), 3}, step{0, 9})
	return action{"jumpwhip-" + dir, seq}
}

// bossHP 取目標槽中最小的 HP（第二型態的幾個判定槽共用同一個值）。
func bossHP(m *machine.Machine, slots []int) int16 {
	hp := sotn.EntityAt(m, slots[0]).HP
	for _, s := range slots[1:] {
		if v := sotn.EntityAt(m, s).HP; v < hp {
			hp = v
		}
	}
	return hp
}

func bossPhase(slots []int) phase {
	acts := []action{
		{"idle", []step{{0, 20}}},
		{"left", []step{{btn("LEFT"), 20}}},
		{"right", []step{{btn("RIGHT"), 20}}},
		{"jump", []step{{btn("CROSS"), 12}, {0, 8}}},
		{"jump-left", []step{{btn("CROSS+LEFT"), 12}, {btn("LEFT"), 8}}},
		{"jump-right", []step{{btn("CROSS+RIGHT"), 12}, {btn("RIGHT"), 8}}},
		{"whip", []step{{btn("SQUARE"), 3}, {0, 17}}},
		jumpWhip(""), jumpWhip("LEFT"), jumpWhip("RIGHT"),
	}
	return phase{
		actions:   acts,
		lookahead: 60,
		score: func(b, a *machine.Machine) float64 {
			db := float64(bossHP(b, slots) - bossHP(a, slots))
			dh := float64(sotn.PlayerHP(b) - sotn.PlayerHP(a))
			p, d := sotn.EntityAt(a, 0), sotn.EntityAt(a, slots[0])
			dist := float64(p.X - d.X)
			if dist < 0 {
				dist = -dist
			}
			// 鞭子約 48 像素長：貼太近或太遠都打不到。
			gap := dist - 48
			if gap < 0 {
				gap = -gap
			}
			return 10*db - 40*dh - 0.05*gap
		},
		done: func(m *machine.Machine) bool { return bossHP(m, slots) <= 0 },
	}
}

func apply(m *machine.Machine, a action, log *sotn.Log) {
	for _, s := range a.seq {
		for i := 0; i < s.n; i++ {
			if log != nil {
				log.Buttons = append(log.Buttons, s.b)
			}
			sotn.Step(m, s.b)
		}
	}
}

func main() {
	bios := flag.String("bios", "", "BIOS")
	cue := flag.String("cue", "", "cue")
	script := flag.String("script", "", "起點以前的 psexe -press 腳本")
	at := flag.Uint64("at", 0, "起點 field")
	phaseName := flag.String("phase", "boss", "目標")
	slotList := flag.String("slots", "80", "boss 的實體槽，逗號分隔（序章德古拉 80；第二型態 85,86,87,88）")
	out := flag.String("out", "", "輸出目錄")
	maxFields := flag.Uint64("max-fields", 6000, "最多再跑幾個 field")
	flag.Parse()

	ps, err := sotn.ParseScript(*script)
	if err != nil {
		fail(err)
	}
	m, err := sotn.Boot(*bios, *cue, nil)
	if err != nil {
		fail(err)
	}
	log := &sotn.Log{}
	for m.Field() < *at {
		b := sotn.ButtonsAt(ps, m.Field())
		log.Buttons = append(log.Buttons, b)
		sotn.Step(m, b)
	}
	var ph phase
	var watch []int
	switch *phaseName {
	case "boss":
		var slots []int
		for _, f := range strings.Split(*slotList, ",") {
			n, err := strconv.Atoi(f)
			if err != nil {
				fail(err)
			}
			slots = append(slots, n)
		}
		ph = bossPhase(slots)
		watch = slots
	default:
		fail(fmt.Errorf("不認識的 phase %q", *phaseName))
	}
	_ = os.MkdirAll(filepath.Join(*out, "shots"), 0o755)
	end := m.Field() + *maxFields
	for k := 0; m.Field() < end && !ph.done(m); k++ {
		scores := make([]float64, len(ph.actions))
		var wg sync.WaitGroup
		for i, a := range ph.actions {
			wg.Add(1)
			go func(i int, a action) {
				defer wg.Done()
				c := m.Clone()
				apply(c, a, nil)
				sotn.Hold(c, 0, ph.lookahead)
				scores[i] = ph.score(m, c)
			}(i, a)
		}
		wg.Wait()
		best := 0
		for i := range scores {
			if scores[i] > scores[best] {
				best = i
			}
		}
		apply(m, ph.actions[best], log)
		fmt.Printf("field=%d act=%s score=%.1f hp=%d boss=%d px=%d bx=%d\n",
			m.Field(), ph.actions[best].name, scores[best], sotn.PlayerHP(m), bossHP(m, watch), sotn.EntityAt(m, 0).X, sotn.EntityAt(m, watch[0]).X)
		if k%10 == 0 {
			_ = sotn.SavePNG(m, filepath.Join(*out, "shots", fmt.Sprintf("f%06d.png", m.Field())))
			_ = os.WriteFile(filepath.Join(*out, "script.txt"), []byte(log.Script()+"\n"), 0o644)
		}
	}
	_ = sotn.SavePNG(m, filepath.Join(*out, "shots", fmt.Sprintf("f%06d.png", m.Field())))
	_ = os.WriteFile(filepath.Join(*out, "script.txt"), []byte(log.Script()+"\n"), 0o644)
	fmt.Printf("end field=%d done=%v\n", m.Field(), ph.done(m))
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "sotnbot:", err)
	os.Exit(1)
}
