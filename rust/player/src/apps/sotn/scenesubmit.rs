//! ST0 scene list submission from retained semantic work, 026 §16.20.
//! GPL-3.0-or-later. No original helper ISA, GP0/GP1 parser or RAM list traversal.
use super::{listclock::Clock, nativework::Work};
use psgolem_core::{
    cpu::{Bus, State},
    gpu::transfer::Rect,
    machine::Machine,
};
use sha2::{Digest, Sha256};
type Result<T> = std::result::Result<T, String>;
fn w(r: &[u8], p: usize) -> u32 {
    u32::from_le_bytes(r[p..p + 4].try_into().unwrap())
}
fn h(r: &[u8], p: usize) -> u16 {
    u16::from_le_bytes(r[p..p + 2].try_into().unwrap())
}
pub struct ScenePlan {
    before: State,
    endpoint: State,
    costs: Vec<u32>,
    ram_sha: [u8; 32],
    biu: u32,
    address: u32,
    target: Rect,
    commands: Vec<psgolem_core::gpu::scene::NativeCommand>,
}
impl ScenePlan {
    pub fn prepare(r: &[u8], s: &State, biu: u32, work: &Work) -> Result<Self> {
        let clock = Clock::prepare(r, s, biu)?;
        let b = match s.gpr[4] {
            0x8003cf7c => 0x3cb08,
            0x80054770 => 0x542fc,
            _ => return Err("非已驗ST0場景列表".into()),
        };
        let x = if b == 0x3cb08 { 0 } else { 256 };
        let p = b + 4;
        if w(r, 0x6c37c) != b as u32 + 0x80000000
            || [
                h(r, p),
                h(r, p + 2),
                h(r, p + 4),
                h(r, p + 6),
                h(r, p + 8),
                h(r, p + 10),
            ] != [x, 20, 256, 207, x, 0]
            || h(r, p + 20) != 10
            || r[p + 22..p + 25] != [0, 1, 1]
            || w(r, 0x9794c) != 0
            || w(r, 0x9792c) != 48
            || work.tiles != w(r, 0x97940) as usize
            || work.pool + work.actors != w(r, 0x97930) as usize + w(r, 0x97938) as usize
            || work.commands.len() != work.pool + work.actors + work.tiles + 48
        {
            return Err("ST0場景來源、環境或工作計數不符".into());
        }
        Ok(Self {
            before: s.clone(),
            endpoint: clock.endpoint,
            costs: clock.costs,
            ram_sha: Sha256::digest(r).into(),
            biu,
            address: s.gpr[4] & 0x1fffff,
            target: Rect {
                x,
                y: 20,
                width: 256,
                height: 207,
            },
            commands: work.commands.clone(),
        })
    }
    pub fn costs(&self) -> &[u32] {
        &self.costs
    }
    pub fn endpoint(&self) -> &State {
        &self.endpoint
    }
    pub fn target(&self) -> Rect {
        self.target
    }
    pub fn commit(&self, m: &mut Machine) -> Result<()> {
        if m.cpu.state != self.before
            || m.bus.biu() != self.biu
            || <[u8; 32]>::from(Sha256::digest(m.bus.ram())) != self.ram_sha
        {
            return Err("場景提交計畫已過期".into());
        }
        m.commit_scene_data_phase(
            &self.endpoint,
            &self.costs,
            [4, 8, 12, 16],
            self.address,
            &self.commands,
            self.target,
        )
    }
}
