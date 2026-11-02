//! Plane RAM/scratch/CPU/GTE transaction, 026 §16.9.18, GPL-3.0-or-later.
//! Exact virtual timing from typed model resources; no guest instructions/GP0.
use super::planeclock::PlaneClockPlan;
use psgolem_core::{
    cpu::{Bus, State},
    gte::Gte,
    machine::{phase::RamWordPatch, Machine},
};
use sha2::{Digest, Sha256};
type Result<T> = std::result::Result<T, String>;
pub struct PlaneMachinePlan {
    clock: PlaneClockPlan,
    before: State,
    before_gte: Gte,
    ram_sha: [u8; 32],
    scratch_sha: [u8; 32],
    biu: u32,
}
impl PlaneMachinePlan {
    pub fn prepare(m: &Machine) -> Result<Self> {
        let biu = m.bus.biu();
        let clock =
            PlaneClockPlan::prepare(m.bus.ram(), m.bus.scratch(), &m.cpu.state, &m.cpu.gte, biu)?;
        Ok(Self {
            clock,
            before: m.cpu.state.clone(),
            before_gte: m.cpu.gte.clone(),
            ram_sha: Sha256::digest(m.bus.ram()).into(),
            scratch_sha: Sha256::digest(m.bus.scratch()).into(),
            biu,
        })
    }
    pub fn clock(&self) -> &PlaneClockPlan {
        &self.clock
    }
    pub fn commit(&self, m: &mut Machine) -> Result<()> {
        if m.cpu.state != self.before
            || m.cpu.gte != self.before_gte
            || m.bus.biu() != self.biu
            || <[u8; 32]>::from(Sha256::digest(m.bus.ram())) != self.ram_sha
            || <[u8; 32]>::from(Sha256::digest(m.bus.scratch())) != self.scratch_sha
        {
            return Err("平面機器計畫已過期".into());
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
        // Scratch is both an input and an output of this source routine.
        m.commit_completed_gte_data_phase(
            self.clock.endpoint(),
            &patches,
            &self.clock.data.scratch,
            self.clock.costs(),
            100000,
            &self.clock.data.gte,
        )
    }
}
