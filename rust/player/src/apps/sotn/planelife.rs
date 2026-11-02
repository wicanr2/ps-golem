//! Procedural-plane lifecycle, 026 §16.9.19, GPL-3.0-or-later.
//! ST0 801BA70C / 801B53A8 / 801B4464; DRA 800ED910 / 800EDC30.
//! Typed contiguous allocation and direct drawing data; no guest ISA or GP0.
use super::{planeclock::PlaneClockPlan, planes};
use psgolem_core::{
    cpu::{Bus, State},
    gte::Gte,
    machine::{phase::RamWordPatch, Machine},
};
use sha2::{Digest, Sha256};
type Result<T> = std::result::Result<T, String>;
const POOL: usize = 0x86fec;
const READS: &[u32] = &[
    0x801ba73c, 0x801ba764, 0x801ba7a4, 0x801ba838, 0x801ba85c, 0x801b53ac, 0x801b53b0, 0x801b53bc,
    0x801b53c8, 0x801b53d4, 0x801b53e0, 0x801b53f8, 0x801b5404, 0x801b5410, 0x801b541c, 0x801b5428,
    0x801b5434, 0x801b5444, 0x801b5448, 0x801b5458, 0x801b545c, 0x801b5468, 0x801b546c, 0x801b548c,
    0x800ed950, 0x800ed9f8, 0x800eda0c, 0x800eda10, 0x800eda14, 0x800eda18, 0x800eda1c, 0x800eda20,
    0x800eda24, 0x800eda28, 0x801b4474, 0x801b448c, 0x801b4490, 0x800edc5c, 0x800edc7c, 0x801b44bc,
    0x801b44c0, 0x801bae7c, 0x801bae80, 0x801bae84, 0x801bae88, 0x801bae8c, 0x801bae90, 0x801bae94,
    0x801bae98, 0x801bae9c, 0x801baea0,
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
            let l = &mut self.state.icache[((p >> 4) & 255) as usize];
            let mut cost = 1;
            if l.tag != p >> 12 || l.valid & (1 << word) == 0 {
                let fill = ((15 << word) & 15) as u8;
                l.valid = if l.tag == p >> 12 {
                    l.valid | fill
                } else {
                    fill
                };
                l.tag = p >> 12;
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
fn h(r: &[u8], p: usize) -> u16 {
    u16::from_le_bytes(r[p..p + 2].try_into().unwrap())
}
fn w(r: &[u8], p: usize) -> u32 {
    u32::from_le_bytes(r[p..p + 4].try_into().unwrap())
}
fn ph(r: &mut [u8], p: usize, v: u16) {
    r[p..p + 2].copy_from_slice(&v.to_le_bytes());
}
fn pw(r: &mut [u8], p: usize, v: u32) {
    r[p..p + 4].copy_from_slice(&v.to_le_bytes());
}
fn identity(r: &[u8]) -> Result<()> {
    for (a, n, id) in [
        (
            0x1ba70c,
            0x7a4,
            "de1e58a6352e4714bb50592730d2f9a83c534d5d51a512795750853664eb9fda",
        ),
        (
            0x1b53a8,
            0xfc,
            "fbb972a5f9a00107dfecec2ce4974e39fb2fc00483e246fe1a5e935911832107",
        ),
        (
            0xed910,
            0x128,
            "b56e54e5735054c850e8b8ab6b7add93d7a4d6fe9c361924bfbf8a1bb5e0029e",
        ),
        (
            0xed828,
            0x24,
            "a49cdfabc53da045c0841080b1ec4acefd7c076878ae83236c6be0676a4e210a",
        ),
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
        (
            0x1805d4,
            10,
            "44326e688080e16f73773e5e17597f3ffbdf44354340f530396b28eb88a32e55",
        ),
    ] {
        if format!("{:x}", Sha256::digest(&r[a..a + n])) != id {
            return Err("平面生命週期來源不符".into());
        }
    }
    Ok(())
}
pub struct PlanePlan {
    pub ram: Vec<u8>,
    pub scratch: Vec<u8>,
    pub gte: Gte,
    pub geometry: planes::Plan,
    before: State,
    before_gte: Gte,
    ram_sha: [u8; 32],
    scratch_sha: [u8; 32],
    biu: u32,
    endpoint: State,
    costs: Vec<u32>,
    pub initialized: bool,
    pub deleted: bool,
}
impl PlanePlan {
    pub fn prepare(r: &[u8], scratch: &[u8], entry: &State, gte: &Gte, biu: u32) -> Result<Self> {
        let entity = entry.gpr[4];
        let sp = entry.gpr[29];
        let ret = entry.gpr[31];
        if r.len() != 2 << 20
            || scratch.len() != 1024
            || entry.pc != 0x801ba70c
            || entry.next_pc != entry.pc + 4
            || entry.bdbt != 0
            || entry.branched
            || entry.ld_which != 34
            || entry.gpr[0] != 0
            || biu != 125320
            || entry.cop0[12] & (1 << 30) == 0
            || entry.cop0[12] & (1 << 16) != 0
            || entry.cop0[12] & entry.cop0[13] & 0xff00 != 0 && entry.cop0[12] & 1 != 0
            || entry.gte_done > entry.now
            || entry.icache.iter().any(|l| l.valid & !15 != 0)
            || entry.now > u64::MAX - 4000000
            || !(0x801f0100..=0x80200000).contains(&sp)
            || sp & 3 != 0
            || !(0x80000000..0x80200000).contains(&ret)
            || ret & 3 != 0
            || !(0x800733d8..0x8007efd8).contains(&entity)
            || (entity - 0x800733d8) % 188 != 0
        {
            return Err("平面生命週期入口未封閉".into());
        }
        identity(r)?;
        let e = (entity - 0x80000000) as usize;
        let initialized = h(r, e + 0x2c) == 0;
        let mut ram = r.to_vec();
        let mut c = Clock {
            state: entry.clone(),
            costs: Vec::new(),
        };
        let frame = (sp - 0x80000000 - 0x40) as usize;
        let mut allocated = true;
        if initialized {
            if w(r, 0x6c3b8) != entity || w(r, 0x3c7e0) != 0x800ed910 || w(r, 0x3c808) != 0x800a88ac
            {
                return Err("平面初始化API或current entity不符".into());
            }
            for (i, reg) in [16, 17, 18, 19, 20, 21, 22, 23, 30, 31]
                .into_iter()
                .enumerate()
            {
                pw(&mut ram, frame + 0x18 + i * 4, entry.gpr[reg]);
            }
            c.span(0x801ba70c, 0x801ba758);
            let priority = h(r, e + 0x24);
            let row = 0xa88ac + h(r, 0x1805dc) as usize * 40;
            if w(r, row + 0x24) & 0x800000 != 0 {
                return Err("平面初始化範本自帶回收旗標尚未封閉".into());
            }
            for (off, source) in [
                (0x54, 0x1805d4),
                (0x56, 0x1805d6),
                (0x5a, 0x1805d8),
                (0x16, 0x1805da),
                (0x3a, 0x1805dc),
                (0x3e, row + 4),
                (0x40, row + 6),
                (0x42, row + 8),
                (0x3c, row + 12),
            ] {
                ph(&mut ram, e + off, h(r, source));
            }
            ram[e + 0x46] = r[row + 0x22];
            ram[e + 0x47] = r[row + 0x23];
            pw(&mut ram, e + 0x34, w(r, row + 0x24));
            for off in [0x10, 0x12, 0x2e] {
                ph(&mut ram, e + off, 0);
            }
            ph(&mut ram, e + 0x2c, 1);
            c.span(0x801b53a8, 0x801b5484);
            if priority == 0 {
                ph(&mut ram, e + 0x24, h(r, 0x97408).wrapping_sub(12));
                c.span(0x801b5488, 0x801b5498);
            }
            c.span(0x801b549c, 0x801b54a0);
            c.span(0x801ba75c, 0x801ba770);
            let af = frame - 0x38;
            for (i, reg) in [16, 17, 18, 19, 20, 21, 22, 31].into_iter().enumerate() {
                let value = match reg {
                    18 => entity,
                    19 => 0x1f800000,
                    31 => 0x801ba774,
                    _ => entry.gpr[reg],
                };
                pw(&mut ram, af + 0x18 + i * 4, value);
            }
            c.span(0x800ed910, 0x800ed94c);
            let mut first = 0;
            let mut remaining = 112;
            let mut found = None;
            for i in 0..1024 {
                c.span(0x800ed950, 0x800ed95c);
                if r[POOL + i * 52 + 7] != 0 {
                    first = i + 1;
                    remaining = 112;
                    c.span(0x800ed960, 0x800ed968);
                } else {
                    remaining -= 1;
                    c.span(0x800ed96c, 0x800ed970);
                    if remaining == 0 {
                        found = Some(first);
                        break;
                    }
                }
                c.span(0x800ed974, 0x800ed980);
            }
            if found.is_none() {
                c.span(0x800ed984, 0x800ed988);
            }
            allocated = found.is_some();
            if let Some(first) = found {
                c.span(0x800ed990, 0x800ed9b8);
                for i in first..first + 112 {
                    let p = POOL + i * 52;
                    c.span(0x800ed9bc, 0x800ed9c0);
                    ram[p..p + 52].fill(0);
                    c.span(0x800ed828, 0x800ed82c);
                    for _ in 0..13 {
                        c.span(0x800ed830, 0x800ed840);
                    }
                    c.span(0x800ed844, 0x800ed848);
                    ram[p + 7] = 4;
                    pw(&mut ram, p, (p + 52) as u32 + 0x80000000);
                    c.span(0x800ed9c4, 0x800ed9f0);
                }
                pw(&mut ram, POOL + (first + 111) * 52, 0);
                c.span(0x800ed9f4, 0x800eda08);
                c.span(0x800eda0c, 0x800eda34);
                c.span(0x801ba774, 0x801ba844);
                let head = POOL + first * 52;
                pw(&mut ram, e + 0x64, first as u32);
                pw(&mut ram, e + 0x7c, head as u32 + 0x80000000);
                let flags = w(&ram, e + 0x34) | 0x800000;
                pw(&mut ram, e + 0x34, flags);
                for (off, value) in [
                    (0x1a, 15),
                    (0xe, 70),
                    (0x2c, 176),
                    (0x14, 176),
                    (0x2e, 123),
                    (0x22, 123),
                    (0x20, 75),
                    (8, 75),
                    (0x16, 0),
                    (0xa, 0),
                    (0x26, 4),
                    (0x32, 0),
                ] {
                    ph(&mut ram, head + off, value);
                }
                for (off, value) in [
                    (0x30, 101),
                    (0x18, 101),
                    (0x19, 128),
                    (0xd, 128),
                    (0x31, 251),
                    (0x25, 251),
                    (0x24, 0),
                    (0xc, 0),
                ] {
                    ram[head + off] = value;
                }
                ph(&mut ram, e + 0x9e, 75);
                ph(&mut ram, e + 0xa2, 0);
                c.span(0x801ba848, 0x801ba84c);
                for i in first + 1..first + 112 {
                    let p = POOL + i * 52;
                    ph(&mut ram, p + 0x1a, 15);
                    ph(&mut ram, p + 0xe, 0x226);
                    ph(&mut ram, p + 0x32, 8);
                    c.span(0x801ba850, 0x801ba868);
                }
                c.span(0x801ba86c, 0x801ba86c);
            } else {
                c.span(0x800eda0c, 0x800eda34);
                c.span(0x801ba774, 0x801ba784);
            }
        }
        let mut out_scratch = scratch.to_vec();
        let mut out_gte = gte.clone();
        let mut geometry = planes::Plan {
            quads: Vec::new(),
            hidden_handles: Vec::new(),
            updated_phases: [0; 4],
            updated_head_phase: [0; 2],
        };
        let deleted =
            !allocated || (h(&ram, 0x7308e) as i16 as i32 + h(&ram, 0x733da) as i16 as i32) < 256;
        let mut endpoint;
        if allocated {
            let p = PlaneClockPlan::prepare_lifecycle(
                &ram,
                scratch,
                entry,
                gte,
                biu,
                if initialized { Some(&c.state) } else { None },
            )?;
            if deleted {
                let (before, count) = p.before_restore();
                c.costs.extend_from_slice(&p.costs()[..count]);
                c.state = before.clone();
                endpoint = p.endpoint().clone();
            } else {
                c.costs.extend_from_slice(p.costs());
                c.state = p.endpoint().clone();
                endpoint = c.state.clone();
            }
            ram = p.data.ram;
            out_scratch = p.data.scratch;
            out_gte = p.data.gte;
            geometry = p.data.geometry;
        } else {
            endpoint = c.state.clone();
        }
        if deleted {
            c.span(0x801bae74, 0x801bae78);
            let freeing = w(&ram, e + 0x34) & 0x800000 != 0;
            let mut links = Vec::new();
            if freeing {
                if w(&ram, 0x3c7b4) != 0x800edc30 {
                    return Err("平面回收API不符".into());
                }
                let ordinal = w(&ram, e + 0x64);
                if ordinal >= 1280 || w(&ram, e + 0x7c) != 0x80000000 + (POOL as u32) + ordinal * 52
                {
                    return Err("平面回收head不符".into());
                }
                let mut at = w(&ram, e + 0x7c);
                while at != 0 {
                    let p = at.wrapping_sub(0x80000000) as usize;
                    if !(POOL..POOL + 1280 * 52).contains(&p)
                        || (p - POOL) % 52 != 0
                        || links.len() >= 112
                        || links.contains(&p)
                        || ram[p + 7] != 4
                    {
                        return Err("平面回收鏈未封閉".into());
                    }
                    links.push(p);
                    at = w(&ram, p);
                }
                if links.len() != 112 {
                    return Err("平面回收鏈長度不符".into());
                }
            }
            pw(
                &mut ram,
                frame - 0x18 + 0x10,
                if allocated { 0x1f800020 } else { entry.gpr[16] },
            );
            pw(&mut ram, frame - 0x18 + 0x14, 0x801bae7c);
            c.span(0x801b4464, 0x801b4484);
            if freeing {
                c.span(0x801b4488, 0x801b4498);
                c.span(0x800edc30, 0x800edc58);
                for &p in &links {
                    c.span(0x800edc5c, 0x800edc68);
                    ram[p + 7] = 0;
                    c.span(0x800edc78, 0x800edc88);
                }
                c.span(0x800edc8c, 0x800edc90);
                geometry
                    .hidden_handles
                    .extend(geometry.quads.iter().map(|q| q.handle));
                geometry
                    .hidden_handles
                    .extend(links.iter().map(|p| (p - POOL) / 52));
                geometry.hidden_handles.sort_unstable();
                geometry.hidden_handles.dedup();
                geometry.quads.clear();
            }
            c.span(0x801b449c, 0x801b44a4);
            ram[e..e + 188].fill(0);
            for _ in 0..47 {
                c.span(0x801b44a8, 0x801b44b8);
            }
            c.span(0x801b44bc, 0x801b44cc);
            c.span(0x801bae7c, 0x801baeac);
            for (reg, value) in [
                (2, 0),
                (3, 47),
                (4, entity + 188),
                (5, 47),
                (34, entry.gpr[16]),
            ] {
                endpoint.gpr[reg] = value;
            }
            endpoint.pc = ret;
            endpoint.next_pc = ret + 4;
            endpoint.ld_which = 34;
            endpoint.ld_value = entry.gpr[16];
            endpoint.bdbt = 0;
            endpoint.branched = false;
            endpoint.icache = c.state.icache;
            endpoint.now = c.state.now;
            endpoint.cost = c.state.cost;
        }
        if c.costs.len() > 100000 || endpoint.now - entry.now > 100000 {
            return Err("平面生命週期超過交易預算".into());
        }
        Ok(Self {
            ram,
            scratch: out_scratch,
            gte: out_gte,
            geometry,
            before: entry.clone(),
            before_gte: gte.clone(),
            ram_sha: Sha256::digest(r).into(),
            scratch_sha: Sha256::digest(scratch).into(),
            biu,
            endpoint,
            costs: c.costs,
            initialized,
            deleted,
        })
    }
    pub fn endpoint(&self) -> &State {
        &self.endpoint
    }
    pub fn costs(&self) -> &[u32] {
        &self.costs
    }
    pub fn from_machine(m: &Machine) -> Result<Self> {
        Self::prepare(
            m.bus.ram(),
            m.bus.scratch(),
            &m.cpu.state,
            &m.cpu.gte,
            m.bus.biu(),
        )
    }
    pub fn commit(&self, m: &mut Machine) -> Result<()> {
        if m.cpu.state != self.before
            || m.cpu.gte != self.before_gte
            || m.bus.biu() != self.biu
            || <[u8; 32]>::from(Sha256::digest(m.bus.ram())) != self.ram_sha
            || <[u8; 32]>::from(Sha256::digest(m.bus.scratch())) != self.scratch_sha
        {
            return Err("平面生命週期計畫已過期".into());
        }
        let patches: Vec<_> = m
            .bus
            .ram()
            .chunks_exact(4)
            .zip(self.ram.chunks_exact(4))
            .enumerate()
            .filter(|(_, (a, b))| a != b)
            .map(|(i, (_, b))| RamWordPatch {
                offset: (i * 4) as u32,
                value: u32::from_le_bytes(b.try_into().unwrap()),
            })
            .collect();
        m.commit_completed_gte_data_phase(
            &self.endpoint,
            &patches,
            &self.scratch,
            &self.costs,
            100000,
            &self.gte,
        )
    }
}
