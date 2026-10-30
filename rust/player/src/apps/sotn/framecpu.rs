//! SotN frame preparation CPU/ABI resources, spec026 §16.3.2.
//! GPL-3.0-or-later. Evidence: sub_800E3880 prelude / sub_80012C90.
//! Staged data plan only: no Machine hook, devices, guest execution, or GP0.
use super::frame::{EntryRegisters, FramePrepPlan};
use psgolem_core::cpu::State;

type Result<T> = std::result::Result<T, String>;

pub struct FrameCpuPlan {
    ram: FramePrepPlan,
    before: State,
    after: State,
    biu: u32,
    cycles: u64,
}

fn entry(s: &State) -> EntryRegisters {
    EntryRegisters {
        pc: s.pc,
        sp: s.gpr[29],
        s0: s.gpr[16],
        s1: s.gpr[17],
    }
}

fn check_entry(s: &State, biu: u32) -> Result<()> {
    let irq = s.cop0[12] & s.cop0[13] & 0xff00 != 0 && s.cop0[12] & 1 != 0;
    if s.pc != 0x800e_3b80
        || s.next_pc != 0x800e_3b84
        || s.bdbt != 0
        || s.branched
        || s.ld_which != 34
        || biu & 0x800 == 0
        || s.cop0[12] & (1 << 16) != 0
        || irq
        || s.icache.iter().any(|line| line.valid & !15 != 0)
    {
        return Err("CPU入口、快取、隔離或IRQ分支不在已驗範圍".into());
    }
    Ok(())
}

// This is a cache resource footprint, not an instruction stream. Each cache
// line is considered at most once per span; all repeated OT work is a hit.
fn touch(s: &mut State, start: u32, end: u32) -> (u64, u64) {
    let mut pc = start & 0x1fff_ffff;
    let end = end & 0x1fff_ffff;
    let mut total = 0;
    let mut last = 0;
    while pc < end {
        let line = &mut s.icache[((pc >> 4) & 255) as usize];
        let last_word = ((end.min((pc | 15) + 1) - 1) >> 2) & 3;
        let first_word = (pc >> 2) & 3;
        last = 0;
        for w in first_word..=last_word {
            last = 0;
            if line.tag != pc >> 12 || line.valid & (1 << w) == 0 {
                let fill = ((15 << w) & 15) as u8;
                line.valid = if line.tag == pc >> 12 {
                    line.valid | fill
                } else {
                    fill
                };
                line.tag = pc >> 12;
                last = (7 - w) as u64;
                total += last;
            }
        }
        pc = (pc | 15) + 1;
    }
    (total, last)
}

impl FrameCpuPlan {
    pub fn prepare(ram: &[u8], state: &State, biu: u32) -> Result<Self> {
        check_entry(state, biu)?;
        let ram = FramePrepPlan::prepare(ram, entry(state))?;
        let mut after = state.clone();
        let mut fetch = 0;
        let mut last = 0;
        for (start, end) in [
            (0x800e_3b80, 0x800e_3bc4),
            (0x8001_2c90, 0x8001_2cb8),
            (0x8001_2cd4, 0x8001_2d3c),
            (0x800e_3bc4, 0x800e_3be4),
        ] {
            let (cost, final_word) = touch(&mut after, start, end);
            fetch += cost;
            last = final_word;
        }
        // 512 entries, non-diagnostic SDK branch: fixed base work and 518
        // RAM loads at the existing core's four-cycle RAM read cost (008).
        let cycles = 5161 + 518 * 4 + fetch;
        after.now = state.now.checked_add(cycles).ok_or("CPU時間溢位")?;
        after.cost = 1 + last;
        let tail = ram.resources().ordering_start + 511 * 4;
        for (reg, value) in [
            (0, 0),
            (1, 0x8013_0000),
            (2, tail),
            (3, tail & 0x00ff_ffff),
            (4, tail),
            (5, 0x00ff_ffff),
            (6, 0xff00_0000),
            (31, 0x800e_3bc4),
            (34, state.gpr[16]),
        ] {
            after.gpr[reg] = value;
        }
        after.pc = 0x800e_3be4;
        after.next_pc = 0x800e_3be8;
        after.bdbt = 0;
        after.branched = false;
        after.ld_which = 34;
        after.ld_value = state.gpr[16];
        Ok(Self {
            ram,
            before: state.clone(),
            after,
            biu,
            cycles,
        })
    }
    pub fn cycles(&self) -> u64 {
        self.cycles
    }
    pub fn endpoint(&self) -> &State {
        &self.after
    }

    /// Atomic RAM + CPU data commit; does not advance a Machine or its devices.
    pub fn apply_data(&self, ram: &mut [u8], state: &mut State, biu: u32) -> Result<()> {
        if *state != self.before || biu != self.biu {
            return Err("CPU計畫已過期".into());
        }
        self.ram.apply_ram(ram, entry(state))?;
        *state = self.after.clone();
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    fn input() -> State {
        State {
            pc: 0x800e_3b80,
            next_pc: 0x800e_3b84,
            ..State::default()
        }
    }
    #[test]
    fn unsupported_cpu_contexts_reject() {
        let s = input();
        assert!(check_entry(&s, 0x800).is_ok());
        assert!(check_entry(&s, 0).is_err());
        for change in 0..8 {
            let mut s = input();
            match change {
                0 => s.pc += 4,
                1 => s.next_pc += 4,
                2 => s.bdbt = 3,
                3 => s.branched = true,
                4 => s.ld_which = 2,
                5 => s.cop0[12] |= 1 << 16,
                6 => {
                    s.cop0[12] |= 0x401;
                    s.cop0[13] |= 0x400;
                }
                _ => s.icache[10].valid = 16,
            }
            assert!(check_entry(&s, 0x800).is_err(), "case {change}");
        }
    }
    #[test]
    fn cache_partial_lines_keep_unrequested_prefix_invalid() {
        let mut s = input();
        let (cost, _) = touch(&mut s, 0x8001_2cd4, 0x8001_2d3c);
        assert!(cost > 0);
        assert_eq!(s.icache[0xcd].valid, 14);
        assert_eq!(touch(&mut s, 0x8001_2cd4, 0x8001_2d3c).0, 0);
        let mut s = input();
        s.icache[0xb8].tag = 0xe3;
        s.icache[0xb8].valid = 1;
        assert_eq!(touch(&mut s, 0x800e_3b80, 0x800e_3b90), (6, 0));
        assert_eq!(s.icache[0xb8].valid, 15);
    }
    #[test]
    #[ignore = "requires local original snapshots; never replace with fake game code"]
    fn original_cpu_endpoints_and_cache_variants() {
        use psgolem_core::cpu::{Bus, CacheLine, Cpu};
        use serde_json::{json, Value};
        use std::{fs, path::PathBuf};
        fn number(v: &Value, key: &str) -> u64 {
            v[key].as_u64().unwrap()
        }
        fn state(v: &Value) -> State {
            State {
                gpr: std::array::from_fn(|i| v["gpr"][i].as_u64().unwrap() as u32),
                hi: number(v, "hi") as u32,
                lo: number(v, "lo") as u32,
                pc: number(v, "pc") as u32,
                next_pc: number(v, "nextPC") as u32,
                bdbt: number(v, "bdbt") as u32,
                ld_which: number(v, "ldWhich") as usize,
                ld_value: number(v, "ldValue") as u32,
                cop0: std::array::from_fn(|i| v["cop0"][i].as_u64().unwrap() as u32),
                branched: v["branched"].as_bool().unwrap(),
                now: number(v, "now"),
                cost: number(v, "cost"),
                muldiv_done: number(v, "muldivDone"),
                gte_done: number(v, "gteDone"),
                icache: std::array::from_fn(|i| CacheLine {
                    tag: number(&v["icache"][i], "tag") as u32,
                    valid: number(&v["icache"][i], "valid") as u8,
                }),
            }
        }
        struct Ram(Vec<u8>);
        impl Bus for Ram {
            fn read8(&mut self, a: u32) -> u8 {
                self.0[(a & 0x1fffff) as usize]
            }
            fn read16(&mut self, a: u32) -> u16 {
                let i = (a & 0x1fffff) as usize;
                u16::from_le_bytes(self.0[i..i + 2].try_into().unwrap())
            }
            fn read32(&mut self, a: u32) -> u32 {
                let i = (a & 0x1fffff) as usize;
                u32::from_le_bytes(self.0[i..i + 4].try_into().unwrap())
            }
            fn write8(&mut self, a: u32, v: u32) {
                self.0[(a & 0x1fffff) as usize] = v as u8;
            }
            fn write16(&mut self, a: u32, v: u32) {
                let i = (a & 0x1fffff) as usize;
                self.0[i..i + 2].copy_from_slice(&(v as u16).to_le_bytes());
            }
            fn write32(&mut self, a: u32, v: u32) {
                let i = (a & 0x1fffff) as usize;
                self.0[i..i + 4].copy_from_slice(&v.to_le_bytes());
            }
            fn isolated_write(&mut self, _: u32, _: u32, _: usize) {
                panic!("unexpected isolation");
            }
            fn instruction_bus_error(&self, _: u32) -> bool {
                false
            }
            fn read_cost(&self, _: u32, _: usize) -> u32 {
                4
            }
            fn biu(&self) -> u32 {
                0x800
            }
        }
        let root = PathBuf::from(
            std::env::var_os("PSGOLEM_FRAMEPREP_FIXTURES").expect("original snapshots required"),
        );
        let report =
            PathBuf::from(std::env::var_os("PSGOLEM_FRAMECPU_REPORT").expect("report required"));
        let mut rows = Vec::new();
        for i in 0..8 {
            let v: Value =
                serde_json::from_slice(&fs::read(root.join(format!("phase-{i:02}.json"))).unwrap())
                    .unwrap();
            let original = state(&v["CPU_before"]);
            let after = state(&v["CPU_after"]);
            let data = fs::read(root.join(format!("phase-{i:02}-before.ram"))).unwrap();
            let expected = fs::read(root.join(format!("phase-{i:02}-after.ram"))).unwrap();
            for variant in 0..5 {
                // Local diagnostic mutation, never a normal-game receipt.
                let mut data = data.clone();
                let mut expected = expected.clone();
                if variant == 4 {
                    data[0x97948..0x9794c].copy_from_slice(&0xdeadbeefu32.to_le_bytes());
                    expected[0x97948..0x9794c].copy_from_slice(&0xdeadbeefu32.to_le_bytes());
                }
                let mut s = original.clone();
                for (j, line) in s.icache.iter_mut().enumerate() {
                    match variant {
                        1 => *line = CacheLine::default(),
                        2 => line.valid = 1 << (j % 4),
                        3 => {
                            line.valid = 15;
                            if (0xb8..=0xbe).contains(&j) {
                                line.tag = 0xe3;
                            }
                            if (0xc9..=0xd3).contains(&j) {
                                line.tag = 0x12;
                            }
                        }
                        _ => (),
                    }
                }
                let p = FrameCpuPlan::prepare(&data, &s, 0x800).unwrap();
                if variant == 0 {
                    assert_eq!(p.endpoint(), &after);
                }
                let mut ram = data.clone();
                let mut actual = s.clone();
                p.apply_data(&mut ram, &mut actual, 0x800).unwrap();
                let mut bus = Ram(data.clone());
                let mut oracle = Cpu {
                    state: s.clone(),
                    ..Cpu::new()
                };
                let mut steps = 0;
                while oracle.pc() != 0x800e_3be4 && steps < 6000 {
                    oracle.step(&mut bus);
                    steps += 1;
                }
                assert_eq!(steps, 5161);
                assert_eq!(actual, oracle.state);
                assert_eq!(ram, bus.0);
                assert_eq!(ram, expected);
                // Changed registers/time/cache and a changed RAM source must all
                // reject atomically, even if only the endpoint fields match.
                let mut rejected = 0;
                for change in 0..5 {
                    let mut stale = s.clone();
                    let mut r = data.clone();
                    let mut biu = 0x800;
                    match change {
                        0 => stale.gpr[20] ^= 1,
                        1 => stale.now += 1,
                        2 => stale.icache[0].valid ^= 1,
                        3 => biu ^= 1,
                        _ => r[0x6c37c] ^= 1,
                    }
                    let held = (r.clone(), stale.clone());
                    assert!(p.apply_data(&mut r, &mut stale, biu).is_err());
                    assert_eq!((r, stale), held);
                    rejected += 1;
                }
                rows.push(json!({"original_case":i,"cache_variant":variant,"CPU_RAM_equal":true,"cycles":p.cycles(),"base_oracle_steps":steps,"atomic_rejections":rejected}));
            }
            let mut overflow = original.clone();
            overflow.now = u64::MAX - 1;
            assert!(FrameCpuPlan::prepare(&data, &overflow, 0x800).is_err());
        }
        fs::write(report,serde_json::to_vec_pretty(&json!({"passed":true,"rows":rows,"normal_game_runs":0,"Machine_device_commit":false,"formal_hook":false,"speed_pass":false})).unwrap()).unwrap();
    }
}
