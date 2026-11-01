//! Complete initialized/retained tower call, 026 §16.9.11, GPL-3.0-or-later.
//! Original ST0 sub_801BAEB0; typed composition of conformed camera and model.
//! No guest ISA, GP0 or old framebuffer. Init/deletion remain explicit errors.
use super::{
    camera::Sources, cameraclock::CameraClockPlan, meshclock::MeshClockPlan, meshstate::DataPlan,
};
use psgolem_core::{
    cpu::{Bus, State},
    gte::Gte,
    machine::{phase::RamWordPatch, Machine},
};
use sha2::{Digest, Sha256};
type Result<T> = std::result::Result<T, String>;
const READS: &[u32] = &[
    0x801baec8, 0x801bb324, 0x801bb32c, 0x801bb34c, 0x801bb350, 0x801bb354, 0x801bb358,
];
struct BoundaryClock {
    state: State,
    costs: Vec<u32>,
}
impl BoundaryClock {
    fn span(&mut self, first: u32, last: u32) {
        for id in (first..=last).step_by(4) {
            let p = id & 0x1fffffff;
            let w = (p >> 2) & 3;
            let line = &mut self.state.icache[((p >> 4) & 255) as usize];
            let mut cost = 1;
            if line.tag != p >> 12 || line.valid & (1 << w) == 0 {
                let fill = ((15 << w) & 15) as u8;
                line.valid = if line.tag == p >> 12 {
                    line.valid | fill
                } else {
                    fill
                };
                line.tag = p >> 12;
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
fn half(r: &[u8], p: usize) -> u16 {
    u16::from_le_bytes(r[p..p + 2].try_into().unwrap())
}
fn word(r: &[u8], p: usize) -> u32 {
    u32::from_le_bytes(r[p..p + 4].try_into().unwrap())
}
fn put(r: &mut [u8], p: usize, v: u32) {
    r[p..p + 4].copy_from_slice(&v.to_le_bytes());
}
pub struct TowerActivePlan {
    pub data: DataPlan,
    before: State,
    before_gte: Gte,
    ram_sha: [u8; 32],
    endpoint: State,
    costs: Vec<u32>,
    biu: u32,
}
impl TowerActivePlan {
    pub fn prepare(r: &[u8], entry: &State, gte: &Gte, biu: u32) -> Result<Self> {
        Sources::new(r)?;
        let ram_sha: [u8; 32] = Sha256::digest(r).into();
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
            || !(0x801f0058..=0x80200000).contains(&sp)
            || sp & 3 != 0
            || !(0x80000000..0x80200000).contains(&ret)
            || ret & 3 != 0
            || !(0x800733d8..0x8007efd8).contains(&entity)
            || (entity - 0x800733d8) % 0xbc != 0
        {
            return Err("鐘塔完整活動呼叫入口未支援".into());
        }
        let e = (entity - 0x80000000) as usize;
        let step = half(r, e + 0x2c) as u32;
        if step == 0 {
            return Err("鐘塔初始化分支尚未接管".into());
        }
        let world_x = half(r, 0x7308e) as i16 as i32 + half(r, 0x733da) as i16 as i32;
        if world_x < 256 {
            return Err("鐘塔刪除分支尚未接管".into());
        }
        let frame = (sp - 0x58 - 0x80000000) as usize;
        let mut ram = r.to_vec();
        for (offset, reg) in [(0x50, 18), (0x54, 31), (0x4c, 17), (0x48, 16)] {
            put(&mut ram, frame + offset, entry.gpr[reg]);
        }
        let mut pre = BoundaryClock {
            state: entry.clone(),
            costs: Vec::with_capacity(10),
        };
        pre.span(0x801baeb0, 0x801baed4);
        pre.state.gpr[29] = sp - 0x58;
        pre.state.gpr[18] = entity;
        pre.state.gpr[2] = step;
        pre.state.gpr[34] = step;
        pre.state.ld_value = step;
        pre.state.ld_which = 34;
        pre.state.pc = 0x801bafb4;
        pre.state.next_pc = 0x801bafb8;
        pre.state.bdbt = 0;
        pre.state.branched = false;
        let camera = CameraClockPlan::prepare(&ram, &pre.state, gte, biu)?;
        let model =
            MeshClockPlan::prepare(&camera.data.ram, camera.endpoint(), &camera.data.gte, biu)?;
        let mut costs = pre.costs;
        costs.extend_from_slice(camera.costs());
        costs.extend_from_slice(model.costs());
        let mut tail = BoundaryClock {
            state: model.endpoint().clone(),
            costs: Vec::with_capacity(16),
        };
        tail.span(0x801bb320, 0x801bb340);
        tail.span(0x801bb34c, 0x801bb364);
        let r = &model.data.ram;
        let saved_s0 = word(r, frame + 0x48);
        let saved_ra = word(r, frame + 0x54);
        for (reg, value) in [
            (1, 0x80070000),
            (2, 0),
            (3, half(r, 0x733da) as i16 as i32 as u32),
            (16, saved_s0),
            (17, word(r, frame + 0x4c)),
            (18, word(r, frame + 0x50)),
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
        Ok(Self {
            data: model.data,
            before: entry.clone(),
            before_gte: gte.clone(),
            ram_sha,
            endpoint: tail.state,
            costs,
            biu,
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
            return Err("鐘塔完整活動計畫已過期".into());
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
