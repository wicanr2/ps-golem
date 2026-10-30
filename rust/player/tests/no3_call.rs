//! Private original snapshot checks, spec026 §16.5. No fake game sources.
#[path = "../src/apps/sotn/call.rs"]
mod call;
#[path = "../src/apps/sotn/callclock.rs"]
mod callclock;
use psgolem_core::cpu::{Bus, CacheLine, Cpu, State};
use serde_json::{json, Value};
use std::{fs, path::PathBuf};
fn state(v: &Value) -> State {
    let n = |k: &str| v[k].as_u64().unwrap();
    State {
        gpr: std::array::from_fn(|i| v["gpr"][i].as_u64().unwrap() as u32),
        hi: n("hi") as u32,
        lo: n("lo") as u32,
        pc: n("pc") as u32,
        next_pc: n("nextPC") as u32,
        bdbt: n("bdbt") as u32,
        ld_which: n("ldWhich") as usize,
        ld_value: n("ldValue") as u32,
        cop0: std::array::from_fn(|i| v["cop0"][i].as_u64().unwrap() as u32),
        branched: v["branched"].as_bool().unwrap(),
        now: n("now"),
        cost: n("cost"),
        muldiv_done: n("muldivDone"),
        gte_done: n("gteDone"),
        icache: std::array::from_fn(|i| CacheLine {
            tag: v["icache"][i]["tag"].as_u64().unwrap() as u32,
            valid: v["icache"][i]["valid"].as_u64().unwrap() as u8,
        }),
    }
}
struct Memory {
    ram: Vec<u8>,
    scratch: Vec<u8>,
    biu: u32,
}
impl Memory {
    fn at(&self, a: u32) -> (&[u8], usize) {
        let p = a & 0x1fff_ffff;
        if (0x1f800000..0x1f800400).contains(&p) {
            (&self.scratch, (p - 0x1f800000) as usize)
        } else {
            assert!(p < 0x200000, "unexpected bus read {a:x}");
            (&self.ram, p as usize)
        }
    }
    fn write(&mut self, a: u32, b: &[u8]) {
        let p = a & 0x1fff_ffff;
        if (0x1f800000..0x1f800400).contains(&p) {
            let i = (p - 0x1f800000) as usize;
            self.scratch[i..i + b.len()].copy_from_slice(b);
        } else {
            assert!(p < 0x200000, "unexpected bus write {a:x}");
            let i = p as usize;
            self.ram[i..i + b.len()].copy_from_slice(b);
        }
    }
}
impl Bus for Memory {
    fn read8(&mut self, a: u32) -> u8 {
        let (b, i) = self.at(a);
        b[i]
    }
    fn read16(&mut self, a: u32) -> u16 {
        let (b, i) = self.at(a);
        u16::from_le_bytes(b[i..i + 2].try_into().unwrap())
    }
    fn read32(&mut self, a: u32) -> u32 {
        let (b, i) = self.at(a);
        u32::from_le_bytes(b[i..i + 4].try_into().unwrap())
    }
    fn write8(&mut self, a: u32, v: u32) {
        self.write(a, &[v as u8]);
    }
    fn write16(&mut self, a: u32, v: u32) {
        self.write(a, &(v as u16).to_le_bytes());
    }
    fn write32(&mut self, a: u32, v: u32) {
        self.write(a, &v.to_le_bytes());
    }
    fn isolated_write(&mut self, _: u32, _: u32, _: usize) {
        panic!("unexpected isolation");
    }
    fn instruction_bus_error(&self, _: u32) -> bool {
        false
    }
    fn read_cost(&self, a: u32, _: usize) -> u32 {
        if a & 0x1fff_ffff < 0x200000 {
            4
        } else {
            0
        }
    }
    fn biu(&self) -> u32 {
        self.biu
    }
}
#[test]
#[ignore = "requires retained local original snapshots; never synthesize game data"]
fn retained_original_no3_calls() {
    let root =
        PathBuf::from(std::env::var_os("PSGOLEM_CALL_FIXTURES").expect("local originals required"));
    let report = PathBuf::from(std::env::var_os("PSGOLEM_CALL_REPORT").expect("report required"));
    let mut rows = Vec::new();
    for name in [
        "precision-oracle",
        "precision-next-8383",
        "precision-next-8384",
        "precision-next-8385",
        "precision-next-8386",
        "precision-next-8387",
    ] {
        let meta: Value =
            serde_json::from_slice(&fs::read(root.join(format!("{name}-before-v1.json"))).unwrap())
                .unwrap();
        let after: Value =
            serde_json::from_slice(&fs::read(root.join(format!("{name}-after-v1.json"))).unwrap())
                .unwrap();
        let s = state(&meta["CPU"]);
        let expected = state(&after["CPU"]);
        let biu = meta["BIU"].as_u64().unwrap() as u32;
        let ram = fs::read(root.join(format!("{name}-before-RAM.bin"))).unwrap();
        let scratch = fs::read(root.join(format!("{name}-before-scratch.bin"))).unwrap();
        let p = call::CallPlan::prepare(&ram, &scratch, &s, biu).unwrap();
        let mut output = ram.clone();
        for w in p.patches() {
            let i = w.offset as usize;
            output[i..i + 4].copy_from_slice(&w.value.to_le_bytes());
        }
        let expected_ram = fs::read(root.join(format!("{name}-after-RAM.bin"))).unwrap();
        assert!(
            output == expected_ram,
            "{name} RAM first differing offset {:?}",
            output.iter().zip(&expected_ram).position(|(a, b)| a != b)
        );
        assert_eq!(
            p.scratch(),
            fs::read(root.join(format!("{name}-after-scratch.bin"))).unwrap(),
            "{name} scratch"
        );
        assert_eq!(p.endpoint(), &expected, "{name} CPU");
        let mut oracle = Cpu {
            state: s.clone(),
            ..Cpu::new()
        };
        let mut bus = Memory {
            ram: ram.clone(),
            scratch: scratch.clone(),
            biu,
        };
        let mut costs = Vec::new();
        while oracle.pc() != s.gpr[31] && costs.len() < 60000 {
            costs.push(oracle.step(&mut bus));
        }
        assert_eq!(p.costs(), costs, "{name} cost sequence");
        assert_eq!(oracle.state, expected);
        assert!(bus.ram == output);
        assert_eq!(bus.scratch, p.scratch());
        let mut rejected = 0;
        for change in 0..5 {
            let mut r = ram.clone();
            let mut sc = scratch.clone();
            let mut st = s.clone();
            let mut b = biu;
            match change {
                0 => r[0x180024] ^= 1,
                1 => sc[100] ^= 1,
                2 => st.gpr[20] ^= 1,
                3 => st.now += 1,
                _ => b ^= 1,
            }
            assert!(p.validate(&r, &sc, &st, b).is_err());
            rejected += 1;
        }
        for change in 0..7 {
            let mut r = ram.clone();
            let mut st = s.clone();
            match change {
                0 => r[0xecc10] ^= 1,
                1 => r[0x730ac] = 1,
                2 => r[0x73124] = 1,
                3 => r[0x2c26c] = 1,
                4 => r[0x97940] = 1,
                5 => r[0x6c39c] = 1,
                _ => st.gpr[29] -= 4,
            }
            assert!(
                call::CallPlan::prepare(&r, &scratch, &st, biu).is_err(),
                "guard {change}"
            );
            rejected += 1;
        }
        rows.push(json!({"case":name,"RAM_scratch_CPU_equal":true,"all_cost_quanta_equal":true,"native_steps":costs.len(),"cycles":expected.now-s.now,"word_patches":p.patches().len(),"rejections":rejected}));
    }
    fs::write(report,serde_json::to_vec_pretty(&json!({"passed":true,"rows":rows,"normal_game_runs":0,"full_HD_complete":false,"speed_pass":false})).unwrap()).unwrap();
}
