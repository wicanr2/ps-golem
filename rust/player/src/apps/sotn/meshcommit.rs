//! Model RAM/CPU/GTE transaction, 026 §16.9.9, GPL-3.0-or-later.
//! Exact virtual timing from typed model resources; no guest instructions/GP0.
use super::meshclock::MeshClockPlan;
use psgolem_core::{
    cpu::{Bus, State},
    gte::Gte,
    machine::{phase::RamWordPatch, Machine},
};
use sha2::{Digest, Sha256};
type Result<T> = std::result::Result<T, String>;
pub struct MeshMachinePlan {
    clock: MeshClockPlan,
    before: State,
    before_gte: Gte,
    ram_sha: [u8; 32],
    biu: u32,
}
impl MeshMachinePlan {
    pub fn prepare(m: &Machine) -> Result<Self> {
        let biu = m.bus.biu();
        let clock = MeshClockPlan::prepare(m.bus.ram(), &m.cpu.state, &m.cpu.gte, biu)?;
        Ok(Self {
            clock,
            before: m.cpu.state.clone(),
            before_gte: m.cpu.gte.clone(),
            ram_sha: Sha256::digest(m.bus.ram()).into(),
            biu,
        })
    }
    pub fn clock(&self) -> &MeshClockPlan {
        &self.clock
    }
    pub fn commit(&self, m: &mut Machine) -> Result<()> {
        if m.cpu.state != self.before
            || m.cpu.gte != self.before_gte
            || m.bus.biu() != self.biu
            || <[u8; 32]>::from(Sha256::digest(m.bus.ram())) != self.ram_sha
        {
            return Err("模型機器計畫已過期".into());
        }
        let patches: Vec<_> = m
            .bus
            .ram()
            .chunks_exact(4)
            .zip(self.clock.data.ram.chunks_exact(4))
            .enumerate()
            .filter(|(_, (a, b))| a != b)
            .map(|(i, (_, b))| RamWordPatch {
                offset: (i * 4) as u32,
                value: u32::from_le_bytes(b.try_into().unwrap()),
            })
            .collect();
        // Scratch is not a model input. Preserve its live contents and let the
        // explicit completed-GTE transaction reject IRQ/DMA/field crossings.
        let scratch = m.bus.scratch().to_vec();
        m.commit_completed_gte_data_phase(
            self.clock.endpoint(),
            &patches,
            &scratch,
            self.clock.costs(),
            100000,
            &self.clock.data.gte,
        )
    }
}
