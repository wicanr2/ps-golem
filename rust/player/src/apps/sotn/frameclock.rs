//! Typed frame-preparation clock resources, spec026 §16.3.3.
//! GPL-3.0-or-later. Staged app component, no player registration.
use super::framecpu::FrameCpuPlan;
use psgolem_core::{
    cpu::{Bus, State},
    machine::{phase::RamWordPatch, Machine},
};

pub struct FrameClockPlan {
    cpu: FrameCpuPlan,
    costs: Vec<u32>,
}
fn fetch(cache: &mut State, pc: u32) -> u32 {
    let p = pc & 0x1fff_ffff;
    let line = &mut cache.icache[((p >> 4) & 255) as usize];
    let w = (p >> 2) & 3;
    if line.tag == p >> 12 && line.valid & (1 << w) != 0 {
        return 0;
    }
    let fill = ((15 << w) & 15) as u8;
    line.valid = if line.tag == p >> 12 {
        line.valid | fill
    } else {
        fill
    };
    line.tag = p >> 12;
    7 - w
}
fn block(cache: &mut State, costs: &mut Vec<u32>, start: u32, slots: u32, loads: &[u32]) {
    for slot in 0..slots {
        costs.push(1 + fetch(cache, start + slot * 4) + if loads.contains(&slot) { 4 } else { 0 });
    }
}
impl FrameClockPlan {
    pub fn prepare(ram: &[u8], state: &State, biu: u32) -> Result<Self, String> {
        let cpu = FrameCpuPlan::prepare(ram, state, biu)?;
        let mut cache = state.clone();
        let mut costs = Vec::with_capacity(5161);
        block(&mut cache, &mut costs, 0x800e3b80, 17, &[1, 3, 6]);
        block(&mut cache, &mut costs, 0x80012c90, 10, &[1]);
        block(&mut cache, &mut costs, 0x80012cd4, 6, &[]);
        block(&mut cache, &mut costs, 0x80012cec, 10, &[3]);
        // Remaining chain elements reuse hot cache resources. No opcode fetch,
        // original instruction execution, or old rendering loop is involved.
        for _ in 0..510 {
            costs.extend_from_slice(&[1, 1, 1, 5, 1, 1, 1, 1, 1, 1]);
        }
        block(&mut cache, &mut costs, 0x80012d14, 10, &[4, 5, 6]);
        block(&mut cache, &mut costs, 0x800e3bc4, 8, &[]);
        if costs.len() != 5161
            || costs.iter().map(|&n| n as u64).sum::<u64>() != cpu.cycles()
            || *costs.last().unwrap() as u64 != cpu.endpoint().cost
            || cache.icache != cpu.endpoint().icache
        {
            return Err("時間足跡與資料端點不符".into());
        }
        Ok(Self { cpu, costs })
    }
    pub fn costs(&self) -> &[u32] {
        &self.costs
    }
    pub fn commit(&self, m: &mut Machine) -> Result<(), String> {
        self.cpu.validate(m.bus.ram(), &m.cpu.state, m.bus.biu())?;
        let patches: Vec<_> = self
            .cpu
            .writes()
            .iter()
            .map(|w| RamWordPatch {
                offset: w.address - 0x8000_0000,
                value: w.value,
            })
            .collect();
        m.commit_ram_phase(
            self.cpu.endpoint(),
            &patches,
            &self.costs,
            self.cpu.cycles(),
        )
    }
}
