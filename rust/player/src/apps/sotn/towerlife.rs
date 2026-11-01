//! Tower destruction data and bounded resource tail, 026 §16.9.13.
//! GPL-3.0-or-later. Original ST0 801B4464 and DRA 800EDC30.
//! Source bytes are identity guards only; no guest ISA or GP0 execution.
use super::meshstate::DataPlan;
use psgolem_core::cpu::State;
use sha2::{Digest, Sha256};
type Result<T> = std::result::Result<T, String>;
const POOL: u32 = 0x80086fec;
const READS: &[u32] = &[
    0x801b4474, 0x801b448c, 0x801b4490, 0x800edc5c, 0x800edc7c, 0x801b44bc, 0x801b44c0, 0x801bb34c,
    0x801bb350, 0x801bb354, 0x801bb358,
];
struct Clock {
    state: State,
    costs: Vec<u32>,
}
impl Clock {
    fn span(&mut self, first: u32, last: u32) {
        for id in (first..=last).step_by(4) {
            let p = id & 0x1fffffff;
            let word = (p >> 2) & 3;
            let line = &mut self.state.icache[((p >> 4) & 255) as usize];
            let mut cost = 1;
            if line.tag != p >> 12 || line.valid & (1 << word) == 0 {
                let fill = ((15 << word) & 15) as u8;
                line.valid = if line.tag == p >> 12 {
                    line.valid | fill
                } else {
                    fill
                };
                line.tag = p >> 12;
                cost += 7 - word;
            }
            if READS.contains(&id) {
                cost += 4;
            }
            self.state.now += cost as u64;
            self.state.cost = cost as u64;
            self.costs.push(cost);
        }
    }
}
fn word(r: &[u8], p: usize) -> u32 {
    u32::from_le_bytes(r[p..p + 4].try_into().unwrap())
}
fn put(r: &mut [u8], p: usize, v: u32) {
    r[p..p + 4].copy_from_slice(&v.to_le_bytes());
}

/// Whole-call suffix at 801BB344, after any camera/model effects.
/// All changes remain detached until the caller's atomic Machine commit.
pub fn finish_deleted(
    data: &mut DataPlan,
    state: &State,
    entity: u32,
    caller_sp: u32,
) -> Result<(State, Vec<u32>)> {
    let r = &mut data.ram;
    for (a, n, hash) in [
        (
            0x1b4464,
            0x6c,
            "250831d9eda6e7c41cf844bcd165c41b399dc06d6efe85a0c8ad5af1849879ff",
        ),
        (
            0xedc30,
            0x64,
            "f729d44ff8d2f9e59b11e3c8ab21b7cf5cd62e2bb51175e12c2505d86b6df2d2",
        ),
    ] {
        if format!("{:x}", Sha256::digest(&r[a..a + n])) != hash {
            return Err("鐘塔銷毀來源不符".into());
        }
    }
    let e = (entity - 0x80000000) as usize;
    let frame = (caller_sp - 0x80000000 - 0x58) as usize;
    let freeing = word(r, e + 0x34) & 0x800000 != 0;
    let mut links = Vec::new();
    if freeing {
        if word(r, 0x3c7b4) != 0x800edc30 {
            return Err("鐘塔回收指標不符".into());
        }
        let ordinal = word(r, e + 0x64);
        if ordinal >= 1280 {
            return Err("鐘塔回收head越界".into());
        }
        let mut at = POOL + ordinal * 52;
        if word(r, e + 0x7c) != at {
            return Err("鐘塔回收head不符".into());
        }
        while at != 0 {
            if !(POOL..POOL + 1280 * 52).contains(&at)
                || (at - POOL) % 52 != 0
                || links.len() >= 40
                || links.contains(&at)
            {
                return Err("鐘塔回收鏈越界或循環".into());
            }
            let p = (at - 0x80000000) as usize;
            if ![4, 5].contains(&r[p + 7]) {
                return Err("鐘塔回收種類未封閉".into());
            }
            links.push(at);
            at = word(r, p);
        }
        if links.len() != 40 {
            return Err("鐘塔回收鏈長度不符".into());
        }
    }
    let mut c = Clock {
        state: state.clone(),
        costs: Vec::new(),
    };
    c.span(0x801bb344, 0x801bb348);
    put(r, frame - 0x18 + 0x10, state.gpr[16]);
    put(r, frame - 0x18 + 0x14, 0x801bb34c);
    c.span(0x801b4464, 0x801b4484);
    if freeing {
        c.span(0x801b4488, 0x801b4498);
        c.span(0x800edc30, 0x800edc58);
        for &link in &links {
            c.span(0x800edc5c, 0x800edc68);
            r[(link - 0x80000000) as usize + 7] = 0;
            c.span(0x800edc78, 0x800edc88);
        }
        c.span(0x800edc8c, 0x800edc90);
    }
    c.span(0x801b449c, 0x801b44a4);
    r[e..e + 188].fill(0);
    for _ in 0..47 {
        c.span(0x801b44a8, 0x801b44b8);
    }
    c.span(0x801b44bc, 0x801b44cc);
    c.span(0x801bb34c, 0x801bb364);
    let saved_s0 = word(r, frame + 0x48);
    let ret = word(r, frame + 0x54);
    for (reg, value) in [
        (2, 0),
        (3, 47),
        (4, entity + 188),
        (5, 47),
        (16, saved_s0),
        (17, word(r, frame + 0x4c)),
        (18, word(r, frame + 0x50)),
        (29, caller_sp),
        (31, ret),
        (34, saved_s0),
    ] {
        c.state.gpr[reg] = value;
    }
    c.state.pc = ret;
    c.state.next_pc = ret + 4;
    c.state.ld_which = 34;
    c.state.ld_value = saved_s0;
    c.state.bdbt = 0;
    c.state.branched = false;
    for d in &data.draws {
        data.hidden_handles.push(d.handle);
    }
    for link in links {
        data.hidden_handles.push(((link - POOL) / 52) as usize);
    }
    data.hidden_handles.sort_unstable();
    data.hidden_handles.dedup();
    data.draws.clear();
    Ok((c.state, c.costs))
}
