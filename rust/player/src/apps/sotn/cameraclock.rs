//! Camera CPU/cache resources, 026 §16.9.5, GPL-3.0-or-later.
//! Original ST0 801BAFB4–801BB04C and SDK 800186AC; own 008/callclock contract.
//! Typed endpoint and resource footprints only, no guest instruction execution.
use super::camerastate::DataPlan;
use psgolem_core::{cpu::State, gte::Gte};
use sha2::{Digest, Sha256};
type Result<T> = std::result::Result<T, String>;

const READS: &[u32] = &[
    0x801bafd8, 0x801bafe4, 0x801baff4, 0x801bb00c, 0x801bb01c, 0x801bb030, 0x800186ac, 0x800186d4,
    0x800186fc, 0x80018710, 0x80018738, 0x80018760, 0x8001877c, 0x800187c4, 0x800187f8, 0x80017e3c,
    0x80017e40, 0x80017e44, 0x80017e48, 0x80017e4c, 0x80017cdc, 0x80017ce0, 0x80017ce4, 0x80017ecc,
    0x80017ed0, 0x80017ed4,
];
struct Clock {
    state: State,
    costs: Vec<u32>,
}
impl Clock {
    fn block(&mut self, first: u32, last: u32) {
        for id in (first..=last).step_by(4) {
            let p = id & 0x1fffffff;
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
            if READS.contains(&id) {
                cost += 4;
            }
            self.state.now += cost as u64;
            self.state.cost = cost as u64;
            self.costs.push(cost);
        }
    }
    fn product(&mut self, at: u32, first_operand: i32) {
        self.block(at, at);
        let zeros = ((first_operand as u32) | 0x400).leading_zeros();
        let delay = if zeros >= 21 {
            7
        } else if zeros >= 12 {
            10
        } else {
            14
        };
        self.state.muldiv_done = self.state.now + delay;
    }
    fn await_product(&mut self, at: u32) {
        self.block(at, at);
        if self.state.now < self.state.muldiv_done {
            let wait = self.state.muldiv_done - self.state.now;
            self.state.now += wait;
            self.state.cost += wait;
            *self.costs.last_mut().unwrap() += wait as u32;
        }
    }
    fn rotation(&mut self, angles: [i16; 3], scalars: [i32; 5]) {
        let [cy, cz, sz, zc, zs] = scalars;
        self.block(0x800186ac, 0x800186b8);
        if angles[0] < 0 {
            self.block(0x800186bc, 0x800186ec);
        } else {
            self.block(0x800186f0, 0x8001870c);
        }
        self.block(0x80018710, 0x8001871c);
        if angles[1] < 0 {
            self.block(0x80018720, 0x80018750);
        } else {
            self.block(0x80018754, 0x80018774);
        }
        self.product(0x80018778, cy);
        self.block(0x8001877c, 0x80018780);
        self.await_product(0x80018784);
        self.block(0x80018788, 0x8001878c);
        self.product(0x80018790, cy);
        self.block(0x80018794, 0x8001879c);
        if angles[2] < 0 {
            self.await_product(0x800187a0);
            self.block(0x800187a4, 0x800187dc);
        } else {
            self.await_product(0x800187e0);
            self.block(0x800187e4, 0x80018808);
        }
        for (at, wait, last, operand) in [
            (0x8001880c, 0x80018818, 0x80018820, cz),
            (0x80018824, 0x80018830, 0x80018838, sz),
            (0x8001883c, 0x80018848, 0x80018850, cz),
            (0x80018854, 0x80018860, 0x80018868, zc),
            (0x8001886c, 0x80018878, 0x80018880, sz),
            (0x80018884, 0x80018890, 0x80018898, zc),
            (0x8001889c, 0x800188a8, 0x800188b0, sz),
            (0x800188b4, 0x800188c0, 0x800188c8, sz),
            (0x800188cc, 0x800188d8, 0x800188e0, zs),
            (0x800188e4, 0x800188f0, 0x800188f8, cz),
            (0x800188fc, 0x80018908, 0x80018910, zs),
            (0x80018914, 0x80018920, 0x80018934, cz),
        ] {
            self.product(at, operand);
            self.block(at + 4, wait - 4);
            self.await_product(wait);
            self.block(wait + 4, last);
        }
    }
}

fn half(r: &[u8], p: usize) -> i16 {
    i16::from_le_bytes(r[p..p + 2].try_into().unwrap())
}
fn word(r: &[u8], p: usize) -> u32 {
    u32::from_le_bytes(r[p..p + 4].try_into().unwrap())
}
fn trig(r: &[u8], a: i16) -> (i32, i32) {
    let p = 0x2e210 + (((a as i32).abs() as usize) & 4095) * 4;
    let s = half(r, p) as i32;
    (if a < 0 { -s } else { s }, half(r, p + 2) as i32)
}

pub struct CameraClockPlan {
    pub data: DataPlan,
    before: State,
    before_gte: Gte,
    ram_sha: [u8; 32],
    endpoint: State,
    costs: Vec<u32>,
}
impl CameraClockPlan {
    pub fn prepare(r: &[u8], entry: &State, gte: &Gte, biu: u32) -> Result<Self> {
        if entry.next_pc != entry.pc.wrapping_add(4)
            || entry.bdbt != 0
            || entry.branched
            || entry.gpr[0] != 0
            || biu != 125320
            || entry.gte_done > entry.now
            || entry.cop0[12] & entry.cop0[13] & 0xff00 != 0 && entry.cop0[12] & 1 != 0
            || entry.icache.iter().any(|c| c.valid & !15 != 0)
            || entry.now > u64::MAX - 2_000_000
        {
            return Err("相機CPU／cache入口不支援".into());
        }
        let data = DataPlan::prepare(r, entry, gte)?;
        let sp = (entry.gpr[29] - 0x80000000) as usize;
        let angles = std::array::from_fn(|i| half(&data.ram, sp + 0x10 + i * 2));
        let (sx, cx) = trig(r, angles[0]);
        let (sy, cy) = trig(r, angles[1]);
        let (sz, cz) = trig(r, angles[2]);
        let mul = |a: i32, b: i32| a.wrapping_mul(b) >> 12;
        let zc = mul(cz, -sy);
        let zs = mul(sz, -sy);
        let mut c = Clock {
            state: entry.clone(),
            costs: Vec::with_capacity(220),
        };
        c.block(0x801bafb4, 0x801bafb8);
        c.block(0x8001805c, 0x80018064);
        c.block(0x801bafbc, 0x801bafc4);
        c.block(0x8001803c, 0x80018050);
        c.block(0x801bafc8, 0x801bb000);
        c.rotation(angles, [cy, cz, sz, zc, zs]);
        c.block(0x801bb004, 0x801bb008);
        c.block(0x80017e3c, 0x80017e68);
        c.block(0x801bb00c, 0x801bb040);
        c.block(0x80017cdc, 0x80017cfc);
        c.block(0x801bb044, 0x801bb048);
        c.block(0x80017ecc, 0x80017ee8);
        let matrix = entry.gpr[29] + 0x28;
        let translation = data.camera.translation.map(|v| v as u32);
        for (reg, value) in [
            (2, matrix),
            (3, 0x580),
            (4, matrix),
            (5, entry.gpr[29] + 0x18),
            (8, translation[0]),
            (9, translation[1]),
            (10, translation[2]),
            (11, word(&data.ram, sp + 0x34)),
            (12, word(&data.ram, sp + 0x38)),
            (13, sz as u32),
            (14, mul(cz, sx).wrapping_sub(mul(zs, cx)) as u32),
            (15, mul(zs, cx) as u32),
            (16, matrix),
            (24, zs as u32),
            (25, mul(cz, sx) as u32),
            (31, 0x801bb04c),
            (34, translation[2]),
        ] {
            c.state.gpr[reg] = value;
        }
        let product = (cz as u32 as u64) * (sx as u32 as u64);
        c.state.hi = (product >> 32) as u32;
        c.state.lo = product as u32;
        c.state.pc = 0x801bb04c;
        c.state.next_pc = 0x801bb050;
        c.state.bdbt = 0;
        c.state.branched = false;
        c.state.ld_which = 34;
        c.state.ld_value = translation[2];
        Ok(Self {
            data,
            before: entry.clone(),
            before_gte: gte.clone(),
            ram_sha: Sha256::digest(r).into(),
            endpoint: c.state,
            costs: c.costs,
        })
    }
    pub fn endpoint(&self) -> &State {
        &self.endpoint
    }
    pub fn costs(&self) -> &[u32] {
        &self.costs
    }
    /// Detached RAM/CPU/GTE update only; never advances devices or a Machine.
    pub fn apply_data(
        &self,
        ram: &mut [u8],
        state: &mut State,
        gte: &mut Gte,
        biu: u32,
    ) -> Result<()> {
        if *state != self.before
            || *gte != self.before_gte
            || biu != 125320
            || <[u8; 32]>::from(Sha256::digest(&*ram)) != self.ram_sha
        {
            return Err("相機CPU資料計畫已過期".into());
        }
        ram.copy_from_slice(&self.data.ram);
        *state = self.endpoint.clone();
        *gte = self.data.gte.clone();
        Ok(())
    }
}
