//! Successful tower initialization whole call, 026 §16.9.12, GPL-3.0-or-later.
//! Original ST0 801BAEB0 / 801B53A8 and DRA 800EDA38 / 800ED828.
//! Typed allocation/data and bounded resource accounting, no guest ISA/GP0.
use super::{
    camera::Sources, cameraclock::CameraClockPlan, meshclock::MeshClockPlan, meshstate::DataPlan,
};
use psgolem_core::{
    cpu::Bus,
    cpu::State,
    gte::Gte,
    machine::{phase::RamWordPatch, Machine},
};
use sha2::{Digest, Sha256};
type Result<T> = std::result::Result<T, String>;
const POOL: usize = 0x86fec;
const READS: &[u32] = &[
    0x801baec8, 0x801baef0, 0x801baf04, 0x801baf18, 0x801baf58, 0x801baf88, 0x801baf8c, 0x801baf90,
    0x801bafa4, 0x801b53ac, 0x801b53b0, 0x801b53bc, 0x801b53c8, 0x801b53d4, 0x801b53e0, 0x801b53f8,
    0x801b5404, 0x801b5410, 0x801b541c, 0x801b5428, 0x801b5434, 0x801b5444, 0x801b5448, 0x801b5458,
    0x801b545c, 0x801b5468, 0x801b546c, 0x801b548c, 0x800eda6c, 0x800eda98, 0x800edb30, 0x800edb34,
    0x800edb38, 0x800edb3c, 0x800edb40, 0x800edb44, 0x801bb324, 0x801bb32c, 0x801bb34c, 0x801bb350,
    0x801bb354, 0x801bb358,
];
struct Clock {
    state: State,
    costs: Vec<u32>,
}
impl Clock {
    fn span(&mut self, first: u32, last: u32) {
        for id in (first..=last).step_by(4) {
            let p = id & 0x1fffffff;
            let w = (p >> 2) & 3;
            let l = &mut self.state.icache[((p >> 4) & 255) as usize];
            let mut cost = 1;
            if l.tag != p >> 12 || l.valid & (1 << w) == 0 {
                let fill = ((15 << w) & 15) as u8;
                l.valid = if l.tag == p >> 12 {
                    l.valid | fill
                } else {
                    fill
                };
                l.tag = p >> 12;
                cost += 7 - w;
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
    Sources::new(r)?;
    for (a, n, hash) in [
        (
            0x1b53a8,
            0xfc,
            "fbb972a5f9a00107dfecec2ce4974e39fb2fc00483e246fe1a5e935911832107",
        ),
        (
            0xeda38,
            0x11c,
            "5b9c61d67f9741218b09484201884939a5941fa087bfc7dffa1b7324a87fba64",
        ),
        (
            0xed828,
            0x24,
            "a49cdfabc53da045c0841080b1ec4acefd7c076878ae83236c6be0676a4e210a",
        ),
        (
            0x1805d4,
            10,
            "44326e688080e16f73773e5e17597f3ffbdf44354340f530396b28eb88a32e55",
        ),
    ] {
        if format!("{:x}", Sha256::digest(&r[a..a + n])) != hash {
            return Err("鐘塔初始化來源不符".into());
        }
    }
    Ok(())
}
/// Selectors and ABI records follow allocation depth, not interpreted registers.
fn allocate(
    r: &mut [u8],
    c: &mut Clock,
    sp: usize,
    slots: &[usize],
    depth: usize,
    saved: [u32; 6],
) {
    let frame = sp - 40;
    let ordinal = slots[depth];
    let node = POOL + ordinal * 52;
    for (off, value) in [
        (0x10, saved[0]),
        (0x14, saved[1]),
        (0x18, saved[2]),
        (0x1c, saved[3]),
        (0x20, saved[4]),
        (0x24, saved[5]),
    ] {
        pw(r, frame + off, value);
    }
    c.span(0x800eda38, 0x800eda68);
    for _ in 0..ordinal {
        c.span(0x800eda6c, 0x800eda78);
        c.span(0x800edb18, 0x800edb28);
    }
    c.span(0x800eda6c, 0x800eda80);
    r[node..node + 52].fill(0);
    r[node + 7] = 4;
    c.span(0x800ed828, 0x800ed82c);
    for _ in 0..13 {
        c.span(0x800ed830, 0x800ed840);
    }
    c.span(0x800ed844, 0x800ed848);
    c.span(0x800eda84, 0x800eda8c);
    if depth + 1 == slots.len() {
        c.span(0x800eda90, 0x800edaa8);
        if (w(r, 0xa23f8) as i32) < ordinal as i32 {
            pw(r, 0xa23f8, ordinal as u32);
            c.span(0x800edaac, 0x800edab8);
            c.span(0x800edb10, 0x800edb14);
        } else {
            c.span(0x800edb0c, 0x800edb14);
        }
    } else {
        c.span(0x800edabc, 0x800edac8);
        allocate(
            r,
            c,
            frame,
            slots,
            depth + 1,
            [
                (node + 7) as u32 + 0x80000000,
                node as u32 + 0x80000000,
                ordinal as u32,
                4,
                (slots.len() - depth) as u32,
                0x800edacc,
            ],
        );
        pw(r, node, (POOL + slots[depth + 1] * 52) as u32 + 0x80000000);
        c.span(0x800edacc, 0x800edadc);
        c.span(0x800edae8, 0x800edb14);
    }
    c.span(0x800edb30, 0x800edb50);
}
pub struct TowerInitPlan {
    pub data: DataPlan,
    before: State,
    before_gte: Gte,
    ram_sha: [u8; 32],
    biu: u32,
    endpoint: State,
    costs: Vec<u32>,
}
impl TowerInitPlan {
    pub fn prepare(r: &[u8], entry: &State, gte: &Gte, biu: u32) -> Result<Self> {
        identity(r)?;
        let entity = entry.gpr[4];
        let sp = entry.gpr[29];
        let ret = entry.gpr[31];
        if entry.pc != 0x801baeb0
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
            || entry.now > u64::MAX - 2_000_000
            || !(0x801f0700..=0x80200000).contains(&sp)
            || sp & 3 != 0
            || !(0x80000000..0x80200000).contains(&ret)
            || ret & 3 != 0
            || !(0x800733d8..0x8007efd8).contains(&entity)
            || (entity - 0x800733d8) % 188 != 0
            || w(r, 0x6c3b8) != entity
            || w(r, 0x3c7b8) != 0x800eda38
            || w(r, 0x3c808) != 0x800a88ac
        {
            return Err("鐘塔初始化入口未支援".into());
        }
        let e = (entity - 0x80000000) as usize;
        if h(r, e + 0x2c) != 0 || (h(r, 0x7308e) as i16 as i32 + h(r, 0x733da) as i16 as i32) < 256
        {
            return Err("鐘塔初始化分支或保留條件不符".into());
        }
        let slots: Vec<_> = (0..1024)
            .filter(|&i| r[POOL + i * 52 + 7] == 0)
            .take(40)
            .collect();
        if slots.len() != 40 {
            return Err("鐘塔配置不足分支尚未接管".into());
        }
        let mut ram = r.to_vec();
        let frame = (sp - 0x58 - 0x80000000) as usize;
        for (off, reg) in [(0x50, 18), (0x54, 31), (0x4c, 17), (0x48, 16)] {
            pw(&mut ram, frame + off, entry.gpr[reg]);
        }
        let mut c = Clock {
            state: entry.clone(),
            costs: Vec::new(),
        };
        c.span(0x801baeb0, 0x801baee4);
        let row = 0xa88ac + h(r, 0x1805dc) as usize * 40;
        let priority = h(r, e + 0x24);
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
        if priority == 0 {
            ph(&mut ram, e + 0x24, h(r, 0x97408).wrapping_sub(12));
        }
        c.span(0x801b53a8, 0x801b5484);
        if priority == 0 {
            c.span(0x801b5488, 0x801b5498);
        }
        c.span(0x801b549c, 0x801b54a0);
        ph(&mut ram, e + 2, 1152u16.wrapping_sub(h(r, 0x7308e)));
        ph(&mut ram, e + 6, 128u16.wrapping_sub(h(r, 0x73092)));
        c.span(0x801baee8, 0x801baf24);
        allocate(
            &mut ram,
            &mut c,
            frame,
            &slots,
            0,
            [
                entry.gpr[16],
                entry.gpr[17],
                entity,
                entry.gpr[19],
                entry.gpr[20],
                0x801baf28,
            ],
        );
        let head = POOL + slots[0] * 52;
        pw(&mut ram, e + 0x64, slots[0] as u32);
        pw(&mut ram, e + 0x7c, head as u32 + 0x80000000);
        let flags = w(&ram, e + 0x34) | 0x00800000;
        pw(&mut ram, e + 0x34, flags);
        c.span(0x801baf28, 0x801baf78);
        for &i in &slots {
            let node = POOL + i * 52;
            for off in [4, 0x10, 0x1c, 0x28] {
                pw(&mut ram, node + off, 0x04808080);
            }
            ph(&mut ram, node + 0x1a, 15);
            c.span(0x801baf7c, 0x801bafb0);
        }
        let at = if (w(r, 0xa23f8) as i32) < *slots.last().unwrap() as i32 {
            0x800a0000
        } else {
            entry.gpr[1]
        };
        for (reg, value) in [
            (1, at),
            (2, 0x04808080),
            (3, 0x04808080),
            (4, 0x04808080),
            (5, 0),
            (6, 128),
            (7, 15),
            (18, entity),
            (29, sp - 0x58),
            (31, 0x801baf28),
            (34, 0),
        ] {
            c.state.gpr[reg] = value;
        }
        c.state.ld_value = 0;
        c.state.ld_which = 34;
        c.state.pc = 0x801bafb4;
        c.state.next_pc = 0x801bafb8;
        c.state.bdbt = 0;
        c.state.branched = false;
        let camera = CameraClockPlan::prepare(&ram, &c.state, gte, biu)?;
        let model =
            MeshClockPlan::prepare(&camera.data.ram, camera.endpoint(), &camera.data.gte, biu)?;
        let mut costs = c.costs;
        costs.extend_from_slice(camera.costs());
        costs.extend_from_slice(model.costs());
        let mut tail = Clock {
            state: model.endpoint().clone(),
            costs: Vec::new(),
        };
        tail.span(0x801bb320, 0x801bb340);
        tail.span(0x801bb34c, 0x801bb364);
        let rr = &model.data.ram;
        let saved_s0 = w(rr, frame + 0x48);
        let saved_ra = w(rr, frame + 0x54);
        for (reg, value) in [
            (2, 0),
            (3, h(rr, 0x733da) as i16 as i32 as u32),
            (16, saved_s0),
            (17, w(rr, frame + 0x4c)),
            (18, w(rr, frame + 0x50)),
            (29, sp),
            (31, saved_ra),
            (34, saved_s0),
        ] {
            tail.state.gpr[reg] = value;
        }
        tail.state.pc = saved_ra;
        tail.state.next_pc = saved_ra + 4;
        tail.state.ld_which = 34;
        tail.state.ld_value = saved_s0;
        tail.state.bdbt = 0;
        tail.state.branched = false;
        costs.extend_from_slice(&tail.costs);
        if costs.len() > 100000 {
            return Err("鐘塔初始化超過已驗交易資源上限".into());
        }
        Ok(Self {
            data: model.data,
            before: entry.clone(),
            before_gte: gte.clone(),
            ram_sha: Sha256::digest(r).into(),
            biu,
            endpoint: tail.state,
            costs,
        })
    }
    pub fn endpoint(&self) -> &State {
        &self.endpoint
    }
    pub fn costs(&self) -> &[u32] {
        &self.costs
    }
    pub fn commit(&self, m: &mut Machine) -> Result<()> {
        if m.cpu.state != self.before
            || m.cpu.gte != self.before_gte
            || m.bus.biu() != self.biu
            || <[u8; 32]>::from(Sha256::digest(m.bus.ram())) != self.ram_sha
        {
            return Err("鐘塔初始化計畫已過期".into());
        }
        let patches: Vec<_> = m
            .bus
            .ram()
            .chunks_exact(4)
            .zip(self.data.ram.chunks_exact(4))
            .enumerate()
            .filter(|(_, (a, b))| a != b)
            .map(|(i, (_, b))| RamWordPatch {
                offset: (i * 4) as u32,
                value: u32::from_le_bytes(b.try_into().unwrap()),
            })
            .collect();
        let scratch = m.bus.scratch().to_vec();
        m.commit_completed_gte_data_phase(
            &self.endpoint,
            &patches,
            &scratch,
            &self.costs,
            100000,
            &self.data.gte,
        )
    }
}
