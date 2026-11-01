//! Candidate for 026 §16.8.3: typed empty ordering-run compaction.
//! GPL-3.0-or-later. Original bytes identify resources, never execute guest ISA.
use psgolem_core::{
    cpu::{Bus, State},
    machine::{phase::RamWordPatch, Machine},
};
use sha2::{Digest, Sha256};
type Result<T> = std::result::Result<T, String>;
fn word(r: &[u8], p: usize) -> u32 {
    u32::from_le_bytes(r[p..p + 4].try_into().unwrap())
}

#[derive(Clone, Debug)]
pub struct EmptyRun {
    pub first: usize,
    pub terminal: usize,
}
struct Resources {
    state: State,
    costs: Vec<u32>,
}
impl Resources {
    fn block(&mut self, first: u32, last: u32) {
        for resource in (first..=last).step_by(4) {
            let p = resource & 0x1fff_ffff;
            let slot = (p >> 2) & 3;
            let line = &mut self.state.icache[((p >> 4) & 255) as usize];
            let mut cost = 1;
            if line.tag != p >> 12 || line.valid & (1 << slot) == 0 {
                let fill = ((15 << slot) & 15) as u8;
                line.valid = if line.tag == p >> 12 {
                    line.valid | fill
                } else {
                    fill
                };
                line.tag = p >> 12;
                cost += 7 - slot;
            }
            if resource == 0x800e376c || resource == 0x800e3790 {
                cost += 4;
            }
            self.state.now += cost as u64;
            self.state.cost = cost as u64;
            self.costs.push(cost);
        }
    }
}
pub struct OrderPlan {
    pub runs: Vec<EmptyRun>,
    patches: Vec<RamWordPatch>,
    endpoint: State,
    before: State,
    costs: Vec<u32>,
    ram_sha: [u8; 32],
    scratch: Vec<u8>,
    biu: u32,
}
impl OrderPlan {
    pub fn prepare(r: &[u8], scratch: &[u8], entry: &State, biu: u32) -> Result<Self> {
        if r.len() != 2 << 20
            || scratch.len() != 1024
            || entry.pc != 0x800e3754
            || entry.next_pc != entry.pc + 4
            || entry.gpr[31] != 0x800e3c98
            || entry.bdbt != 0
            || entry.branched
            || entry.ld_which != 34
            || entry.gpr[0] != 0
            || biu != 125320
            || entry.cop0[12] & (1 << 16) != 0
            || entry.cop0[12] & entry.cop0[13] & 0xff00 != 0 && entry.cop0[12] & 1 != 0
            || entry.icache.iter().any(|c| c.valid & !15 != 0)
            || entry.now > u64::MAX - 100_000
        {
            return Err("排序入口／CPU／cache不在已驗範圍".into());
        }
        if format!("{:x}", Sha256::digest(&r[0xe3754..0xe37c4]))
            != "bad677d498d2e13ba800ad767abddc9f970799da4f16646411023a87807a82f0"
        {
            return Err("排序來源不符".into());
        }
        let base = entry.gpr[4];
        let frame = word(r, 0x6c37c);
        if ![0x8003cb08, 0x800542fc].contains(&frame)
            || base != frame + 0x474
            || word(r, 0x13556c) != base
        {
            return Err("排序表不是目前已驗雙緩衝".into());
        }
        let offset = (base & 0x1fff_ffff) as usize;
        let tags: Vec<_> = (0..512).map(|i| word(r, offset + i * 4)).collect();
        let mut c = Resources {
            state: entry.clone(),
            costs: Vec::new(),
        };
        let mut runs = Vec::new();
        let mut pending = None;
        let mut last_first = entry.gpr[9];
        c.block(0x800e3754, 0x800e3760);
        for (i, &tag) in tags.iter().enumerate() {
            let physical = (base & 0xff_ffff) + i as u32 * 4;
            let forward = tag & 0xff_ffff == physical + 4;
            c.block(0x800e3764, 0x800e3768);
            if let Some(first) = pending {
                c.block(0x800e3790, 0x800e37a0);
                if !forward {
                    runs.push(EmptyRun { first, terminal: i });
                    pending = None;
                    c.block(0x800e37a4, 0x800e37a8);
                }
            } else {
                c.block(0x800e376c, 0x800e3780);
                if forward {
                    pending = Some(i);
                    last_first = base + i as u32 * 4;
                    c.block(0x800e3784, 0x800e378c);
                }
            }
            c.block(0x800e37ac, 0x800e37b8);
        }
        c.block(0x800e37bc, 0x800e37c0);
        let patches = runs
            .iter()
            .filter_map(|run| {
                let at = offset + run.first * 4;
                let value = (base & 0xff_ffff) + run.terminal as u32 * 4;
                (word(r, at) != value).then_some(RamWordPatch {
                    offset: at as u32,
                    value,
                })
            })
            .collect();
        let raw = tags[511];
        for (reg, value) in [
            (2, 0),
            (3, raw & 0xff_ffff),
            (4, base + 2048),
            (5, (base & 0xff_ffff) + 2044),
            (6, 0xff_ffff),
            (7, 512),
            (8, u32::from(pending.is_some())),
            (9, last_first),
            (34, raw),
        ] {
            c.state.gpr[reg] = value;
        }
        c.state.ld_which = 34;
        c.state.ld_value = raw;
        c.state.pc = entry.gpr[31];
        c.state.next_pc = entry.gpr[31] + 4;
        c.state.bdbt = 0;
        c.state.branched = false;
        Ok(Self {
            runs,
            patches,
            endpoint: c.state,
            before: entry.clone(),
            costs: c.costs,
            ram_sha: Sha256::digest(r).into(),
            scratch: scratch.to_vec(),
            biu,
        })
    }
    pub fn endpoint(&self) -> &State {
        &self.endpoint
    }
    pub fn costs(&self) -> &[u32] {
        &self.costs
    }
    pub fn patches(&self) -> &[RamWordPatch] {
        &self.patches
    }
    pub fn commit(&self, m: &mut Machine) -> Result<()> {
        if m.cpu.state != self.before
            || m.bus.biu() != self.biu
            || m.bus.scratch() != self.scratch.as_slice()
            || <[u8; 32]>::from(Sha256::digest(m.bus.ram())) != self.ram_sha
        {
            return Err("排序計畫已過期".into());
        }
        m.commit_ram_phase(&self.endpoint, &self.patches, &self.costs, 16_384)
    }
}
