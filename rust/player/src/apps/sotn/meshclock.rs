//! Model CPU/cache/GTE resource timing, 026 §16.9.8, GPL-3.0-or-later.
//! Original ST0 801BB04C–801BB320; own 008/cache and typed mesh data contracts.
//! No guest opcodes, saved-PC replay, or per-instruction GPR execution.
use super::meshstate::DataPlan;
use psgolem_core::{cpu::State, gte::Gte};
type Result<T> = std::result::Result<T, String>;
const RAM_READS: &[u32] = &[
    0x801bb058, 0x801bb05c, 0x801bb074, 0x801bb088, 0x801bb09c, 0x801bb0a0, 0x801bb0b4, 0x801bb0c0,
    0x801bb0cc, 0x801bb0d8, 0x801bb0e4, 0x801bb0f0, 0x801bb0fc, 0x801bb10c, 0x801bb110, 0x801bb114,
    0x801bb130, 0x801bb134, 0x801bb138, 0x801bb13c, 0x801bb140, 0x801bb144, 0x801bb168, 0x801bb17c,
    0x801bb180, 0x801bb194, 0x801bb1b0, 0x801bb1c4, 0x801bb1d8, 0x801bb1dc, 0x801bb1f8, 0x801bb208,
    0x801bb218, 0x801bb21c, 0x801bb230, 0x801bb234, 0x801bb248, 0x801bb258, 0x801bb25c, 0x801bb260,
    0x801bb27c, 0x801bb280, 0x801bb284, 0x801bb288, 0x801bb28c, 0x801bb290, 0x801bb2ac, 0x801bb2bc,
    0x801bb2c0, 0x801bb2d8, 0x801bb2f4, 0x801bb2f8, 0x801bb310,
];
const GTE_ACCESS: &[u32] = &[
    0x801bb130, 0x801bb134, 0x801bb138, 0x801bb13c, 0x801bb140, 0x801bb144, 0x801bb150, 0x801bb154,
    0x801bb158, 0x801bb15c, 0x801bb27c, 0x801bb280, 0x801bb284, 0x801bb288, 0x801bb28c, 0x801bb290,
    0x801bb29c, 0x801bb2a0, 0x801bb2a4, 0x801bb2a8, 0x801bb2bc, 0x801bb2c0, 0x801bb2cc, 0x801bb2d4,
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
            // LWC2 waits before its RAM read. Adding RAM first undercounts
            // a pending GTE dependency even when the final total looks close.
            if GTE_ACCESS.contains(&id) && self.state.now + cost < self.state.gte_done {
                cost = self.state.gte_done - self.state.now;
            }
            if RAM_READS.contains(&id) {
                cost += 4;
            }
            self.state.now += cost;
            self.state.cost = cost;
            self.costs.push(cost as u32);
        }
    }
    fn projection(&mut self, id: u32, latency: u64) {
        self.block(id, id);
        self.state.gte_done = self.state.now + latency - 1;
    }
}
struct FaceResource {
    corners: usize,
    switched: bool,
    bank: u32,
    vertex0: u32,
}
fn word(r: &[u8], p: usize) -> u32 {
    u32::from_le_bytes(r[p..p + 4].try_into().unwrap())
}
// DataPlan has already validated these exact streams and bounds. This view
// retains only semantic bank transitions and endpoint source pointers.
fn resources(r: &[u8]) -> (Vec<FaceResource>, [u32; 2]) {
    let mut faces = Vec::new();
    let mut ends = [0; 2];
    for (group, start, corners) in [(0, 0x1823c4, 3), (1, 0x1823fc, 4)] {
        let mut at = start;
        let mut bank = 0;
        while r[at] != 255 {
            let switched = r[at] & 128 != 0;
            if switched {
                bank = word(r, 0x182440 + r[at + 1] as usize * 4);
                at += 2;
            }
            faces.push(FaceResource {
                corners,
                switched,
                bank,
                vertex0: bank + r[at + 1] as u32 * 8,
            });
            at += corners + 2;
        }
        ends[group] = at as u32 + 0x80000000;
    }
    (faces, ends)
}
pub struct MeshClockPlan {
    pub data: DataPlan,
    endpoint: State,
    costs: Vec<u32>,
}
impl MeshClockPlan {
    pub fn prepare(r: &[u8], entry: &State, gte: &Gte, biu: u32) -> Result<Self> {
        if entry.next_pc != entry.pc.wrapping_add(4)
            || entry.bdbt != 0
            || entry.branched
            || entry.gpr[0] != 0
            || biu != 125320
            || entry.gte_done > entry.now
            || entry.cop0[12] & entry.cop0[13] & 0xff00 != 0 && entry.cop0[12] & 1 != 0
            || entry.icache.iter().any(|l| l.valid & !15 != 0)
            || entry.now > u64::MAX - 2_000_000
        {
            return Err("模型CPU／cache入口未支援".into());
        }
        let data = DataPlan::prepare(r, entry, gte)?;
        let (faces, ends) = resources(r);
        if faces.len() != data.draws.len()
            || faces
                .iter()
                .zip(&data.draws)
                .any(|(a, b)| a.corners != b.face.corners)
        {
            return Err("模型資源與型別面不符".into());
        }
        let mut c = Clock {
            state: entry.clone(),
            costs: Vec::with_capacity(2000),
        };
        c.block(0x801bb04c, 0x801bb064);
        if faces.iter().any(|f| f.corners == 3) {
            c.block(0x801bb068, 0x801bb070);
            for f in faces.iter().filter(|f| f.corners == 3) {
                c.block(0x801bb074, 0x801bb084);
                if f.switched {
                    c.block(0x801bb088, 0x801bb09c);
                }
                c.block(0x801bb0a0, 0x801bb14c);
                c.projection(0x801bb150, 23);
                c.block(0x801bb154, 0x801bb188);
            }
        }
        c.block(0x801bb18c, 0x801bb1a0);
        if faces.iter().any(|f| f.corners == 4) {
            c.block(0x801bb1a4, 0x801bb1ac);
            for f in faces.iter().filter(|f| f.corners == 4) {
                c.block(0x801bb1b0, 0x801bb1c0);
                if f.switched {
                    c.block(0x801bb1c4, 0x801bb1d8);
                }
                c.block(0x801bb1dc, 0x801bb298);
                c.projection(0x801bb29c, 23);
                c.block(0x801bb2a0, 0x801bb2c8);
                c.projection(0x801bb2cc, 15);
                c.block(0x801bb2d0, 0x801bb300);
            }
        }
        c.block(0x801bb304, 0x801bb308);
        for _ in &data.hidden_handles {
            c.block(0x801bb30c, 0x801bb31c);
        }
        let last = faces.last().ok_or("模型無末面")?;
        for (reg, value) in [
            (2, 8),
            (3, 255),
            (4, last.vertex0),
            (5, 0),
            (6, ends[1]),
            (
                7,
                ends[if last.corners == 3 { 0 } else { 1 }] + last.corners as u32,
            ),
            (8, 0x80182440),
            (17, last.bank),
            (34, 0),
        ] {
            c.state.gpr[reg] = value;
        }
        c.state.pc = 0x801bb320;
        c.state.next_pc = 0x801bb324;
        c.state.bdbt = 0;
        c.state.branched = false;
        c.state.ld_which = 34;
        c.state.ld_value = 0;
        if c.state.muldiv_done > c.state.now || c.state.gte_done > c.state.now {
            return Err("模型終點有未完成的算術資源".into());
        }
        Ok(Self {
            data,
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
}
