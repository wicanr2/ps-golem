//! Atomic camera CPU/RAM/GTE transaction, 026 §16.9.6, GPL-3.0-or-later.
//! Uses the existing 16.3.3 device scheduler contract; no guest steps/GP0.
use super::cameraclock::CameraClockPlan;
use psgolem_core::{
    cpu::{Bus, State},
    gte::Gte,
    machine::{phase::RamWordPatch, Machine},
};
use sha2::{Digest, Sha256};
type Result<T> = std::result::Result<T, String>;

pub struct CameraMachinePlan {
    clock: CameraClockPlan,
    before: State,
    before_gte: Gte,
    ram_sha: [u8; 32],
    biu: u32,
}
impl CameraMachinePlan {
    pub fn prepare(m: &Machine) -> Result<Self> {
        let biu = m.bus.biu();
        let clock = CameraClockPlan::prepare(m.bus.ram(), &m.cpu.state, &m.cpu.gte, biu)?;
        Ok(Self {
            clock,
            before: m.cpu.state.clone(),
            before_gte: m.cpu.gte.clone(),
            ram_sha: Sha256::digest(m.bus.ram()).into(),
            biu,
        })
    }
    pub fn clock(&self) -> &CameraClockPlan {
        &self.clock
    }
    pub fn commit(&self, m: &mut Machine) -> Result<()> {
        if m.cpu.state != self.before
            || m.cpu.gte != self.before_gte
            || m.bus.biu() != self.biu
            || <[u8; 32]>::from(Sha256::digest(m.bus.ram())) != self.ram_sha
        {
            return Err("相機機器計畫已過期".into());
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
        // The existing transaction rejects active DMA/MDEC, IRQ and field
        // crossings. Preserve live scratch, which this phase never reads.
        let mut candidate = m.clone();
        candidate.commit_cpu_data_phase(
            self.clock.endpoint(),
            &patches,
            m.bus.scratch(),
            self.clock.costs(),
            100000,
        )?;
        // GTE is CPU-owned; SystemBus scheduling cannot read its controls.
        // No device observes the deferred control writes within this phase.
        candidate.cpu.gte = self.clock.data.gte.clone();
        *m = candidate;
        Ok(())
    }
}
