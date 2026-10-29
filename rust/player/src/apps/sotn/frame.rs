//! SotN whole-frame resource preparation, spec026 §16.3.1, GPL-3.0-or-later.
//! Data/ABI evidence: DRA sub_800E3880 prelude and boot sub_80012C90.
//! This data component is staged; it is NOT registered in the player or CPU.
//! It does not decode instructions, execute a guest call, or submit GP0.
use sha2::{Digest, Sha256};
use std::ops::Range;

const RAM_SIZE: usize = 2 << 20;
const BASE: u32 = 0x8000_0000;
const ENTRY: u32 = 0x800e_3b80;
const BUFFER: u32 = 0x8006_c37c;
const PREVIOUS: u32 = 0x8013_8364;
const COUNTER: u32 = 0x8003_c998;
const OT_POINTER: u32 = 0x8013_556c;
const COUNTERS: u32 = 0x8009_792c;
const DEBUG_LEVEL: u32 = 0x8002_c268;
const OT_WORDS: u32 = 512;
const OUTER_HASH: &str = "08f7ac668f1e5d491d2ef9a176c17d11760ce602a269a6dfe4f8acd659690da0";
const CLEAR_HASH: &str = "86577716397f0014ba20a60992c84534f9b48ad5138b9421d6581f6620a4982e";
type Result<T> = std::result::Result<T, String>;

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct EntryRegisters {
    pub pc: u32,
    pub sp: u32,
    pub s0: u32,
    pub s1: u32,
}
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct FrameResources {
    pub previous_buffer: u32,
    pub draw_buffer: u32,
    pub ordering_start: u32,
    pub ordering_words: u32,
    pub frame_counter: u32,
}
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct WordWrite {
    pub address: u32,
    pub value: u32,
}
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
struct Inputs {
    entry: EntryRegisters,
    previous: u32,
    next: u32,
    counter: u32,
    debug_level: u32,
}
#[derive(Debug)]
pub struct FramePrepPlan {
    input: Inputs,
    resources: FrameResources,
    writes: Vec<WordWrite>,
}

fn span(address: u32, bytes: u32) -> Result<Range<usize>> {
    if address & 3 != 0 {
        return Err(format!("RAM地址未對齊：{address:08x}"));
    }
    let begin = address.checked_sub(BASE).ok_or("地址不是KSEG0 RAM")?;
    let end = begin.checked_add(bytes).ok_or("RAM範圍溢位")?;
    if end as usize > RAM_SIZE {
        return Err(format!("RAM範圍超界：{address:08x}+{bytes}"));
    }
    Ok(begin as usize..end as usize)
}
fn word(ram: &[u8], a: u32) -> Result<u32> {
    Ok(u32::from_le_bytes(ram[span(a, 4)?].try_into().unwrap()))
}
fn overlap(a: &Range<usize>, b: &Range<usize>) -> bool {
    a.start < b.end && b.start < a.end
}
fn read_inputs(ram: &[u8], entry: EntryRegisters) -> Result<Inputs> {
    if ram.len() != RAM_SIZE || entry.pc != ENTRY || entry.s1 != COUNTERS {
        return Err("RAM長度或階段入口不符".into());
    }
    for (start, end, expected) in [
        (0xe3880, 0xe3fa8, OUTER_HASH),
        (0x12c90, 0x12d3c, CLEAR_HASH),
    ] {
        if format!("{:x}", Sha256::digest(&ram[start..end])) != expected {
            return Err("原程式身分不符，不能套用資料計畫".into());
        }
    }
    let debug_level = word(ram, DEBUG_LEVEL)?;
    if debug_level >= 2 {
        return Err("SDK診斷輸出分支尚未支援".into());
    }
    let previous = word(ram, BUFFER)?;
    Ok(Inputs {
        entry,
        previous,
        next: word(ram, previous)?,
        counter: word(ram, COUNTER)?,
        debug_level,
    })
}

impl FramePrepPlan {
    pub fn prepare(ram: &[u8], entry: EntryRegisters) -> Result<Self> {
        Self::from_inputs(read_inputs(ram, entry)?)
    }
    fn from_inputs(input: Inputs) -> Result<Self> {
        if input.entry.pc != ENTRY || input.entry.s1 != COUNTERS || input.debug_level >= 2 {
            return Err("階段或SDK診斷分支不符".into());
        }
        span(input.previous, 4)?;
        span(input.next, 4)?;
        let ot = input.next.checked_add(0x474).ok_or("排序表地址溢位")?;
        let stack = input.entry.sp.checked_sub(16).ok_or("堆疊地址溢位")?;
        let regions = [
            span(ot, OT_WORDS * 4)?,
            span(COUNTERS, 9 * 4)?,
            span(stack, 12)?,
            span(PREVIOUS, 4)?,
            span(BUFFER, 4)?,
            span(COUNTER, 4)?,
            span(OT_POINTER, 4)?,
        ];
        let protected = [
            0xe3880..0xe3fa8,
            0x12c90..0x12d3c,
            span(DEBUG_LEVEL, 4)?,
            span(input.previous, 4)?,
        ];
        for (i, a) in regions.iter().enumerate() {
            if regions[..i].iter().any(|b| overlap(a, b)) || protected.iter().any(|b| overlap(a, b))
            {
                return Err("輸出與來源／程式區域別名，拒絕未證實配置".into());
            }
        }
        let resources = FrameResources {
            previous_buffer: input.previous,
            draw_buffer: input.next,
            ordering_start: ot,
            ordering_words: OT_WORDS,
            frame_counter: input.counter.wrapping_add(1),
        };
        let mut writes = Vec::with_capacity(528);
        for (address, value) in [
            (PREVIOUS, input.previous),
            (BUFFER, input.next),
            (COUNTER, resources.frame_counter),
            (OT_POINTER, ot),
            (stack, input.entry.s0),
            (stack + 4, input.entry.s1),
            (stack + 8, 0x800e_3bc4),
        ] {
            writes.push(WordWrite { address, value });
        }
        // A typed forward ordering chain, not a replay of the original loop.
        for i in 0..OT_WORDS {
            writes.push(WordWrite {
                address: ot + i * 4,
                value: if i + 1 == OT_WORDS {
                    0x00ff_ffff
                } else {
                    (ot + (i + 1) * 4) & 0x00ff_ffff
                },
            });
        }
        for i in 0..9 {
            writes.push(WordWrite {
                address: COUNTERS + i * 4,
                value: 0,
            });
        }
        Ok(Self {
            input,
            resources,
            writes,
        })
    }
    pub fn resources(&self) -> FrameResources {
        self.resources
    }
    pub fn writes(&self) -> &[WordWrite] {
        &self.writes
    }

    /// Checks the entire read set first. Rejection leaves the RAM unchanged.
    /// This does not commit CPU, clock, devices, GPU, or a player frame.
    pub fn apply_ram(&self, ram: &mut [u8], entry: EntryRegisters) -> Result<()> {
        if read_inputs(ram, entry)? != self.input {
            return Err("計畫已過期，RAM或入口暫存器已改變".into());
        }
        // Every output was bounded and checked for aliases during prepare.
        for w in &self.writes {
            let at = (w.address - BASE) as usize;
            ram[at..at + 4].copy_from_slice(&w.value.to_le_bytes());
        }
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    fn input() -> Inputs {
        Inputs {
            entry: EntryRegisters {
                pc: ENTRY,
                sp: 0x801f_ffc0,
                s0: 123,
                s1: COUNTERS,
            },
            previous: 0x8004_0000,
            next: 0x8005_0000,
            counter: u32::MAX,
            debug_level: 0,
        }
    }
    #[test]
    fn counter_wrap_and_stack_abi_are_explicit() {
        let p = FramePrepPlan::from_inputs(input()).unwrap();
        assert_eq!(p.resources().frame_counter, 0);
        assert_eq!(p.writes().len(), 528);
        assert!(p.writes().contains(&WordWrite {
            address: 0x801f_ffb0,
            value: 123
        }));
        assert!(p.writes().contains(&WordWrite {
            address: 0x801f_ffb4,
            value: COUNTERS
        }));
        assert!(p.writes().contains(&WordWrite {
            address: 0x801f_ffb8,
            value: 0x800e_3bc4
        }));
    }
    #[test]
    fn invalid_pointers_and_stack_reject() {
        for next in [0, 0x8000_0001, 0x8020_0000, 0xffff_ffff, 0x801f_ffff] {
            let mut v = input();
            v.next = next;
            assert!(FramePrepPlan::from_inputs(v).is_err());
        }
        for sp in [0, 0x8000_0000, 0x8020_0010, 0x801f_ffff] {
            let mut v = input();
            v.entry.sp = sp;
            assert!(FramePrepPlan::from_inputs(v).is_err());
        }
    }
    #[test]
    fn output_aliases_are_not_silently_ordered() {
        for next in [
            BUFFER - 0x474,
            PREVIOUS - 0x474,
            COUNTERS - 0x474,
            DEBUG_LEVEL - 0x474,
            0x800e_3880 - 0x474,
            input().previous - 0x474,
        ] {
            let mut v = input();
            v.next = next;
            assert!(FramePrepPlan::from_inputs(v).is_err(), "{next:08x}");
        }
    }
    #[test]
    fn unsupported_entry_and_debug_reject() {
        let mut v = input();
        v.entry.s1 += 4;
        assert!(FramePrepPlan::from_inputs(v).is_err());
        let mut v = input();
        v.entry.pc += 4;
        assert!(FramePrepPlan::from_inputs(v).is_err());
        let mut v = input();
        v.debug_level = 2;
        assert!(FramePrepPlan::from_inputs(v).is_err());
    }
    #[test]
    fn invalid_program_and_length_never_modify_ram() {
        let plan = FramePrepPlan::from_inputs(input()).unwrap();
        for len in [0, RAM_SIZE - 1, RAM_SIZE, RAM_SIZE + 1] {
            let mut ram = vec![0u8; len];
            let held = ram.clone();
            assert!(plan.apply_ram(&mut ram, input().entry).is_err());
            assert_eq!(ram, held);
        }
    }
    #[test]
    #[ignore = "requires local original snapshots; no substitute game data"]
    fn original_frame_preparation_corpus() {
        use serde_json::{json, Value};
        use std::{fs, path::PathBuf};
        let root = PathBuf::from(
            std::env::var_os("PSGOLEM_FRAMEPREP_FIXTURES").expect("original snapshots required"),
        );
        let report = PathBuf::from(
            std::env::var_os("PSGOLEM_FRAMEPREP_REPORT").expect("report path required"),
        );
        let mut rows = Vec::new();
        for i in 0..8 {
            let meta: Value =
                serde_json::from_slice(&fs::read(root.join(format!("phase-{i:02}.json"))).unwrap())
                    .unwrap();
            let s = &meta["CPU_before"];
            let g = &s["gpr"];
            let regs = EntryRegisters {
                pc: s["pc"].as_u64().unwrap() as u32,
                sp: g[29].as_u64().unwrap() as u32,
                s0: g[16].as_u64().unwrap() as u32,
                s1: g[17].as_u64().unwrap() as u32,
            };
            let before = fs::read(root.join(format!("phase-{i:02}-before.ram"))).unwrap();
            let after = fs::read(root.join(format!("phase-{i:02}-after.ram"))).unwrap();
            let p = FramePrepPlan::prepare(&before, regs).unwrap();
            let mut actual = before.clone();
            p.apply_ram(&mut actual, regs).unwrap();
            let equal = actual == after;
            let mut rejected = 0;
            for address in [COUNTER, BUFFER, 0x8001_2c90] {
                let mut changed = before.clone();
                changed[(address - BASE) as usize] ^= 1;
                let held = changed.clone();
                assert!(p.apply_ram(&mut changed, regs).is_err());
                assert_eq!(changed, held);
                rejected += 1;
            }
            let mut changed = before.clone();
            let mut wrong = regs;
            wrong.pc += 4;
            assert!(p.apply_ram(&mut changed, wrong).is_err());
            assert_eq!(changed, before);
            rejected += 1;
            rows.push(json!({"case":i,"all_RAM_equal":equal,"RAM_bytes":actual.len(),"word_writes":p.writes().len(),"draw_buffer":p.resources().draw_buffer,"ordering_start":p.resources().ordering_start,"atomic_rejections":rejected,"actual_sha256":format!("{:x}",Sha256::digest(&actual)),"expected_sha256":format!("{:x}",Sha256::digest(&after))}));
        }
        let passed = rows.iter().all(|v| v["all_RAM_equal"] == true);
        fs::write(report,serde_json::to_vec_pretty(&json!({"RAM_data_plan_pass":passed,"rows":rows,"normal_game_replays":0,"CPU_clock_device_commit":false,"gameplay_hook_installed":false,"full_HD_complete":false})).unwrap()).unwrap();
        assert!(passed, "complete original RAM differs");
    }
}
