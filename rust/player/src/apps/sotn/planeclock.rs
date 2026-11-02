//! Active plane CPU/cache/resource clock, spec026 §16.9.17.
//! GPL-3.0-or-later. Own 008/cameraclock and confirmed ST0 resource footprints.
//! No guest ISA decoding, saved trace input, GPR interpretation or Machine jump.
use super::planestate::{DataPlan, Outcome};
use psgolem_core::{cpu::State, gte::Gte};
type Result<T> = std::result::Result<T, String>;
const RAM_READS: &[u32] = &[
    0x800186ac, 0x800186d4, 0x800186fc, 0x80018710, 0x80018738, 0x80018760, 0x8001877c, 0x800187c4,
    0x800187f8, 0x801ba73c, 0x801ba764, 0x801ba7a4, 0x801ba838, 0x801ba85c, 0x801ba898, 0x801ba8a8,
    0x801ba8ac, 0x801ba8b8, 0x801ba8d8, 0x801ba8dc, 0x801ba8e0, 0x801ba8e4, 0x801ba908, 0x801ba90c,
    0x801ba910, 0x801ba914, 0x801ba938, 0x801ba93c, 0x801ba940, 0x801ba944, 0x801ba968, 0x801ba96c,
    0x801ba970, 0x801ba974, 0x801ba9c8, 0x801baa34, 0x801baa38, 0x801baa60, 0x801baa74, 0x801baa94,
    0x801baaa4, 0x801baab0, 0x801baac4, 0x801baad8, 0x801baaec, 0x801bab00, 0x801bab04, 0x801bab08,
    0x801bab2c, 0x801bab34, 0x801bab38, 0x801bab58, 0x801bac1c, 0x801bac20, 0x801bac60, 0x801bad28,
    0x801bad3c, 0x801badf4, 0x801bae40, 0x801bae54, 0x801bae5c, 0x801bae7c, 0x801bae80, 0x801bae84,
    0x801bae88, 0x801bae8c, 0x801bae90, 0x801bae94, 0x801bae98, 0x801bae9c, 0x801baea0,
];
const GTE_ACCESS: &[u32] = &[
    0x80017e50, 0x80017e54, 0x80017e58, 0x80017e5c, 0x80017e60, 0x80018044, 0x80018048, 0x8001805c,
    0x801bab50, 0x801bab80, 0x801bac70, 0x801bac78, 0x801bac7c, 0x801bac88, 0x801bacb4, 0x801bacb8,
    0x801bacbc, 0x801bacc0, 0x801bacc4, 0x801bacc8, 0x801baccc, 0x801bacd8, 0x801bad68, 0x801bad6c,
    0x801bad70,
];
struct Clock {
    state: State,
    costs: Vec<u32>,
    ids: Vec<u32>,
    overflow: bool,
}
impl Clock {
    fn block(&mut self, first: u32, last: u32) {
        for id in (first..=last).step_by(4) {
            if self.costs.len() >= 100000 {
                self.overflow = true;
                return;
            }
            let p = id & 0x1fffffff;
            let slot = (p >> 2) & 3;
            let line = &mut self.state.icache[((p >> 4) & 255) as usize];
            let mut cost = 1u64;
            if line.tag != p >> 12 || line.valid & (1 << slot) == 0 {
                let fill = ((15 << slot) & 15) as u8;
                line.valid = if line.tag == p >> 12 {
                    line.valid | fill
                } else {
                    fill
                };
                line.tag = p >> 12;
                cost += (7 - slot) as u64;
            }
            // Coprocessor access waits before the actual RAM read.
            if GTE_ACCESS.contains(&id) && self.state.now + cost < self.state.gte_done {
                cost = self.state.gte_done - self.state.now;
            }
            if RAM_READS.contains(&id) {
                cost += 4;
            }
            self.state.now += cost;
            self.state.cost = cost;
            self.costs.push(cost as u32);
            self.ids.push(id);
        }
    }
    fn product(&mut self, id: u32, value: i32) {
        self.block(id, id);
        let key = (value as u32) ^ ((value >> 31) as u32);
        let zeros = (key | 0x400).leading_zeros();
        let delay = if zeros >= 21 {
            7
        } else if zeros >= 12 {
            10
        } else {
            14
        };
        self.state.muldiv_done = self.state.now + delay;
    }
    fn await_product(&mut self, id: u32) {
        self.block(id, id);
        if self.state.now < self.state.muldiv_done {
            let wait = self.state.muldiv_done - self.state.now;
            self.state.now += wait;
            self.state.cost += wait;
            if let Some(cost) = self.costs.last_mut() {
                *cost += wait as u32;
            }
        }
    }
    fn projection(&mut self, id: u32, latency: u64) {
        self.block(id, id);
        self.state.gte_done = self.state.now + latency - 1;
    }
    // Fixed zero-angle SDK domain already checked by the plane data plan.
    fn rotation_zero(&mut self) {
        for (a, b) in [
            (0x800186ac, 0x800186b8),
            (0x800186f0, 0x8001870c),
            (0x80018710, 0x8001871c),
            (0x80018754, 0x80018774),
        ] {
            self.block(a, b);
        }
        self.product(0x80018778, 4096);
        self.block(0x8001877c, 0x80018780);
        self.await_product(0x80018784);
        self.block(0x80018788, 0x8001878c);
        self.product(0x80018790, 4096);
        self.block(0x80018794, 0x8001879c);
        self.await_product(0x800187e0);
        self.block(0x800187e4, 0x80018808);
        for (at, wait, last, value) in [
            (0x8001880c, 0x80018818, 0x80018820, 4096),
            (0x80018824, 0x80018830, 0x80018838, 0),
            (0x8001883c, 0x80018848, 0x80018850, 4096),
            (0x80018854, 0x80018860, 0x80018868, 0),
            (0x8001886c, 0x80018878, 0x80018880, 0),
            (0x80018884, 0x80018890, 0x80018898, 0),
            (0x8001889c, 0x800188a8, 0x800188b0, 0),
            (0x800188b4, 0x800188c0, 0x800188c8, 0),
            (0x800188cc, 0x800188d8, 0x800188e0, 0),
            (0x800188e4, 0x800188f0, 0x800188f8, 4096),
            (0x800188fc, 0x80018908, 0x80018910, 0),
            (0x80018914, 0x80018920, 0x80018934, 4096),
        ] {
            self.product(at, value);
            self.block(at + 4, wait - 4);
            self.await_product(wait);
            self.block(wait + 4, last);
        }
    }
}
fn h(r: &[u8], p: usize) -> i16 {
    i16::from_le_bytes(r[p..p + 2].try_into().unwrap())
}
fn w(r: &[u8], p: usize) -> u32 {
    u32::from_le_bytes(r[p..p + 4].try_into().unwrap())
}
pub struct PlaneClockPlan {
    pub data: DataPlan,
    endpoint: State,
    costs: Vec<u32>,
    resource_ids: Vec<u32>,
}
impl PlaneClockPlan {
    pub fn prepare(r: &[u8], scratch: &[u8], entry: &State, gte: &Gte, biu: u32) -> Result<Self> {
        if entry.next_pc != entry.pc.wrapping_add(4)
            || biu != 125320
            || !(0x80000000..0x80200000).contains(&entry.gpr[31])
            || entry.gpr[31] & 3 != 0
            || entry.gte_done > entry.now
            || entry.cop0[12] & entry.cop0[13] & 0xff00 != 0 && entry.cop0[12] & 1 != 0
            || entry.icache.iter().any(|l| l.valid & !15 != 0)
            || entry.now > u64::MAX - 4000000
        {
            return Err("平面CPU、cache或事件入口未封閉".into());
        }
        let data = DataPlan::prepare(r, scratch, entry, gte)?;
        if data.rows.len() != 16 || data.rows.iter().any(|row| row.visits.is_empty()) {
            return Err("平面語意列或候選資源不完整".into());
        }
        let mut c = Clock {
            state: entry.clone(),
            costs: Vec::with_capacity(13000),
            ids: Vec::with_capacity(13000),
            overflow: false,
        };
        c.block(0x801ba70c, 0x801ba748);
        c.block(0x801ba870, 0x801ba9c0);
        for _ in 0..36 {
            c.block(0x801ba9c4, 0x801ba9dc);
        }
        c.block(0x801ba9e0, 0x801baa00);
        c.block(0x8001805c, 0x80018064);
        c.block(0x801baa04, 0x801baa0c);
        c.block(0x8001803c, 0x80018050);
        c.block(0x801baa10, 0x801baa1c);
        c.rotation_zero();
        c.block(0x801baa20, 0x801baa24);
        c.block(0x80017e3c, 0x80017e68);
        c.block(0x801baa28, 0x801baa44);
        if w(r, 0x3c998) & 1 != 0 {
            c.block(0x801baa48, 0x801baa4c);
        } else {
            c.block(0x801baa50, 0x801baa50);
        }
        c.block(0x801baa54, 0x801baa68);
        c.product(0x801baa6c, (w(r, 0x97908) << 16) as i32);
        c.block(0x801baa70, 0x801baa74);
        c.await_product(0x801baa78);
        c.block(0x801baa7c, 0x801baa80);
        c.product(0x801baa84, (w(r, 0x9790c) << 16) as i32);
        c.block(0x801baa88, 0x801baaa4);
        c.await_product(0x801baaa8);
        c.block(0x801baaac, 0x801bab00);
        for (i, row) in data.rows.iter().enumerate() {
            if i % 8 == 0 {
                c.block(0x801bab04, 0x801bab18);
                if row.source_x < 0 {
                    c.block(0x801bab1c, 0x801bab1c);
                }
                c.block(0x801bab20, 0x801bab40);
                if h(r, 0x73092) < 0 {
                    c.block(0x801bab44, 0x801bab44);
                }
                c.block(0x801bab48, 0x801bab54);
            }
            c.block(0x801bab58, 0x801bab68);
            c.block(0x801bab70, 0x801bab8c);
            c.block(0x801bab94, 0x801bac50);
            let mut prefix = 0x801bac54;
            for visit in &row.visits {
                c.block(prefix, 0x801bac6c);
                if matches!(visit.outcome, Outcome::Empty) {
                    prefix = 0x801bac54;
                    continue;
                }
                c.block(0x801bac70, 0x801bac84);
                c.projection(0x801bac88, 15);
                c.block(0x801bac8c, 0x801bacd4);
                c.projection(0x801bacd8, 23);
                c.block(0x801bacdc, 0x801bace8);
                if matches!(visit.outcome, Outcome::Top) {
                    c.block(0x801bacec, 0x801bacf0);
                    break;
                }
                c.block(0x801bacf4, 0x801bad00);
                if matches!(visit.outcome, Outcome::Left) {
                    c.block(0x801bad04, 0x801bad08);
                    if visit.direction < 0 {
                        c.block(0x801bad0c, 0x801bad18);
                    }
                    prefix = 0x801bac54;
                    continue;
                }
                c.block(0x801bad1c, 0x801bad24);
                if visit.tile < 5 {
                    c.block(0x801bad28, 0x801bad38);
                } else {
                    c.block(0x801bad3c, 0x801bad48);
                }
                c.block(0x801bad4c, 0x801bad84);
                if matches!(visit.outcome, Outcome::Right) {
                    c.block(0x801bad88, 0x801bad8c);
                    if visit.direction > 0 {
                        break;
                    }
                    c.block(0x801bad90, 0x801bad94);
                    prefix = 0x801bac58;
                    continue;
                }
                c.block(0x801bad98, 0x801bae08);
                prefix = 0x801bac58;
            }
            c.block(0x801bae0c, 0x801bae18);
            if i % 8 == 7 {
                c.block(0x801bae1c, 0x801bae30);
            }
        }
        c.block(0x801bae34, 0x801bae38);
        for _ in &data.geometry.hidden_handles {
            c.block(0x801bae3c, 0x801bae4c);
        }
        c.block(0x801bae50, 0x801bae70);
        c.block(0x801bae7c, 0x801baeac);
        if c.overflow {
            return Err("平面資源超過有界成本項".into());
        }
        let row = data.rows.last().unwrap();
        let visit = row.visits.last().unwrap();
        let world = h(r, 0x7308e) as i32 + h(r, 0x733da) as i32;
        for (reg, value) in [
            (1, 0x1f800000),
            (2, 0),
            (3, h(r, 0x733da) as i32 as u32),
            (4, 0x80182158 + row.tile_row as u32 * 8),
            (5, row.top[1] as u32),
            (6, visit.tile as u32),
            (7, row.top[0] as u8 as u32),
            (8, (0xf0000 - 352 * (row.z - 448)) as u32),
            (9, 0),
            (10, row.tile_row as u32),
            (11, visit.column as u32),
            (12, visit.direction as u32),
            (13, world as u32),
            (14, row.origin as u32),
            (15, 8),
            (24, entry.gpr[4] + 0x96),
            (25, 2),
            (34, entry.gpr[16]),
        ] {
            c.state.gpr[reg] = value;
        }
        let product = (((w(r, 0x9790c) << 16) as i32 as i64) * 0x2aaaaaabi64) as u64;
        c.state.hi = (product >> 32) as u32;
        c.state.lo = product as u32;
        c.state.pc = entry.gpr[31];
        c.state.next_pc = entry.gpr[31].wrapping_add(4);
        c.state.bdbt = 0;
        c.state.branched = false;
        c.state.ld_which = 34;
        c.state.ld_value = entry.gpr[16];
        if c.state.gte_done > c.state.now || c.state.muldiv_done > c.state.now {
            return Err("平面末端算術資源尚未完成".into());
        }
        Ok(Self {
            data,
            endpoint: c.state,
            costs: c.costs,
            resource_ids: c.ids,
        })
    }
    pub fn endpoint(&self) -> &State {
        &self.endpoint
    }
    pub fn costs(&self) -> &[u32] {
        &self.costs
    }
    /// Generated footprint IDs for diagnostics, never native trace input.
    pub fn resource_ids(&self) -> &[u32] {
        &self.resource_ids
    }
}
