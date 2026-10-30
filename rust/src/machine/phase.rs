//! Bounded no-I/O data transactions, spec026 §16.3.3, GPL-3.0-or-later.
//! Uses the existing device scheduler; never executes guest instructions.
use super::Machine;
use crate::cpu::State;

#[derive(Clone, Copy, Debug)]
pub struct RamWordPatch {
    pub offset: u32,
    pub value: u32,
}

impl Machine {
    fn data_phase_idle(&self) -> bool {
        let dpcr = self.bus.dma.read(0x1f80_10f0);
        !self.bus.irq_pending()
            && !self.bus.mdec.waiting_clock()
            && (0..7).all(|ch| {
                self.bus.dma.read(0x1f80_1088 + 16 * ch) & (1 << 24) == 0
                    || dpcr & (1 << (3 + 4 * ch)) == 0
            })
    }

    /// A caller must prove that its phase only consumes and produces RAM/CPU
    /// data and does not perform MMIO. This method validates the transport,
    /// not the semantics of the supplied CPU endpoint or RAM values.
    /// Rejection leaves this Machine intact, including queued audio.
    pub fn commit_ram_phase(
        &mut self,
        endpoint: &State,
        patches: &[RamWordPatch],
        costs: &[u32],
        budget: u64,
    ) -> Result<(), String> {
        self.commit_data_phase(endpoint, patches, None, costs, budget)
    }

    /// Explicit completed arithmetic + scratch transport, spec026 §16.5.
    /// The caller proves no MMIO/GTE or device-visible intermediate RAM writes.
    /// Neither method runs guest instructions. The older RAM-only contract
    /// continues to reject any changed HI/LO/multiply deadline.
    pub fn commit_cpu_data_phase(
        &mut self,
        endpoint: &State,
        patches: &[RamWordPatch],
        scratch: &[u8],
        costs: &[u32],
        budget: u64,
    ) -> Result<(), String> {
        if scratch.len() != self.bus.scratch.len() {
            return Err("scratch長度不符".into());
        }
        self.commit_data_phase(endpoint, patches, Some(scratch), costs, budget)
    }

    fn commit_data_phase(
        &mut self,
        endpoint: &State,
        patches: &[RamWordPatch],
        scratch: Option<&[u8]>,
        costs: &[u32],
        budget: u64,
    ) -> Result<(), String> {
        if costs.is_empty() || costs.len() > 100_000 || patches.len() > 524_288 {
            return Err("資料階段大小不符".into());
        }
        let mut total = 0u64;
        for &n in costs {
            if !(1..=32).contains(&n) {
                return Err("時間量子超界".into());
            }
            total = total.checked_add(n as u64).ok_or("時間總量溢位")?;
        }
        let end = self.cycles.checked_add(total).ok_or("機器時間溢位")?;
        let before = &self.cpu.state;
        if total > budget
            || before.now != self.cycles
            || endpoint.now != end
            || endpoint.cost != *costs.last().unwrap() as u64
            || endpoint.pc & 3 != 0
            || endpoint.pc < 0x8000_0000
            || endpoint.pc >= 0x8020_0000
            || endpoint.next_pc != endpoint.pc + 4
            || endpoint.bdbt != 0
            || endpoint.branched
            || endpoint.ld_which != 34
            || endpoint.gpr[0] != 0
            || endpoint.cop0 != before.cop0
            || before.cop0[13] & 0x400 != 0
            || if scratch.is_some() {
                endpoint.muldiv_done > end
            } else {
                endpoint.hi != before.hi
                    || endpoint.lo != before.lo
                    || endpoint.muldiv_done != before.muldiv_done
            }
            || endpoint.gte_done != before.gte_done
            || endpoint.icache.iter().any(|line| line.valid & !15 != 0)
            || !self.data_phase_idle()
        {
            return Err("CPU／時間或裝置上下文不符".into());
        }
        let mut offsets = Vec::with_capacity(patches.len());
        for patch in patches {
            if patch.offset & 3 != 0 || patch.offset as usize >= self.bus.ram.len() {
                return Err("RAM字寫入超界或未對齊".into());
            }
            offsets.push(patch.offset);
        }
        offsets.sort_unstable();
        if offsets.windows(2).any(|w| w[0] == w[1]) {
            return Err("RAM字寫入重複".into());
        }
        // All speculative device events and audio stay in this owned clone.
        // Same immutable Disc Arc, no borrowed device/presentation callbacks.
        let mut candidate = self.clone();
        let field = self.field();
        for &n in costs {
            candidate.cycles += n as u64;
            candidate.bus.advance(n as u64);
            if candidate.field() != field || !candidate.data_phase_idle() {
                return Err("資料階段跨場、IRQ或活動裝置".into());
            }
        }
        for patch in patches {
            let offset = patch.offset as usize;
            candidate.bus.ram[offset..offset + 4].copy_from_slice(&patch.value.to_le_bytes());
        }
        if let Some(scratch) = scratch {
            candidate.bus.scratch.copy_from_slice(scratch);
        }
        candidate.cpu.state = endpoint.clone();
        *self = candidate;
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::cpu::Bus;
    fn machine() -> Machine {
        // Synthetic transport unit test, never an original-game substitute.
        Machine::new(&vec![0; 0x80000]).unwrap()
    }
    fn endpoint(m: &Machine, n: u64, last: u64) -> State {
        let mut s = m.cpu.state.clone();
        s.now += n;
        s.cost = last;
        s.pc = 0x8001_0000;
        s.next_pc = s.pc + 4;
        s
    }
    #[test]
    fn data_commit_advances_clock_and_ram_without_cpu_steps() {
        let mut m = machine();
        let mut s = endpoint(&m, 7, 1);
        s.gpr[2] = 123;
        m.commit_ram_phase(
            &s,
            &[RamWordPatch {
                offset: 16,
                value: 456,
            }],
            &[1, 5, 1],
            7,
        )
        .unwrap();
        assert_eq!(m.cycles(), 7);
        assert_eq!(m.cpu.state, s);
        assert_eq!(&m.bus.ram()[16..20], &456u32.to_le_bytes());
    }
    #[test]
    fn completed_arithmetic_and_scratch_have_an_explicit_contract() {
        let mut m = machine();
        let mut s = endpoint(&m, 7, 1);
        s.hi = 12;
        s.lo = 345;
        s.muldiv_done = 6;
        let scratch = vec![0x57; 1024];
        assert!(m.commit_ram_phase(&s, &[], &[1, 5, 1], 7).is_err());
        assert!(m
            .commit_cpu_data_phase(&s, &[], &scratch[..1023], &[1, 5, 1], 7)
            .is_err());
        s.muldiv_done = 8;
        assert!(m
            .commit_cpu_data_phase(&s, &[], &scratch, &[1, 5, 1], 7)
            .is_err());
        assert_eq!(m.cycles(), 0);
        assert!(m.bus.scratch().iter().all(|&x| x == 0));
        s.muldiv_done = 6;
        m.commit_cpu_data_phase(&s, &[], &scratch, &[1, 5, 1], 7)
            .unwrap();
        assert_eq!(m.cpu.state, s);
        assert_eq!(m.bus.scratch(), scratch);
    }
    #[test]
    fn scratch_transaction_rolls_back_on_irq() {
        let mut m = machine();
        m.bus.write32(0x1f801074, 1 << 4);
        m.bus.write32(0x1f801108, 2);
        m.bus.write32(0x1f801104, 1 << 4);
        let timer = m.bus.timers.peek(0x1f801100);
        let s = endpoint(&m, 5, 5);
        assert!(m
            .commit_cpu_data_phase(&s, &[], &[0x57; 1024], &[5], 5)
            .is_err());
        assert_eq!(m.cycles(), 0);
        assert_eq!(m.bus.timers.peek(0x1f801100), timer);
        assert!(m.bus.scratch().iter().all(|&x| x == 0));
        assert!(m.take_audio().is_empty());
    }
    #[test]
    fn malformed_data_and_costs_reject_before_mutation() {
        let mut m = machine();
        let before = m.cpu.state.clone();
        for costs in [vec![], vec![0], vec![33], vec![1; 100001]] {
            assert!(m
                .commit_ram_phase(&endpoint(&m, 1, 1), &[], &costs, 1000000)
                .is_err());
        }
        for patches in [
            vec![RamWordPatch {
                offset: 1,
                value: 1,
            }],
            vec![RamWordPatch {
                offset: 0x200000,
                value: 1,
            }],
            vec![
                RamWordPatch {
                    offset: 4,
                    value: 1
                };
                2
            ],
        ] {
            assert!(m
                .commit_ram_phase(&endpoint(&m, 1, 1), &patches, &[1], 1)
                .is_err());
        }
        assert!(m
            .commit_ram_phase(&endpoint(&m, 1, 1), &[], &[1], 0)
            .is_err());
        assert_eq!(m.cpu.state, before);
        assert_eq!(m.cycles(), 0);
        assert!(m.bus.ram().iter().all(|&x| x == 0));
    }
    #[test]
    fn wrong_cpu_endpoint_and_pending_irq_reject() {
        let mut m = machine();
        for change in 0..6 {
            let mut s = endpoint(&m, 1, 1);
            match change {
                0 => s.now += 1,
                1 => s.cost += 1,
                2 => s.pc += 1,
                3 => s.cop0[12] ^= 1,
                4 => s.ld_which = 2,
                _ => s.icache[0].valid = 16,
            }
            assert!(m.commit_ram_phase(&s, &[], &[1], 1).is_err());
        }
        m.bus.i_stat = 1;
        m.bus.i_mask = 1;
        assert!(m
            .commit_ram_phase(&endpoint(&m, 1, 1), &[], &[1], 1)
            .is_err());
        assert_eq!(m.bus.i_stat, 1);
        assert_eq!(m.cycles(), 0);
    }
    #[test]
    fn irq_during_trial_leaves_live_timer_and_audio_intact() {
        let mut m = machine();
        m.bus.write32(0x1f801074, 1 << 4);
        m.bus.write32(0x1f801108, 2);
        m.bus.write32(0x1f801104, 1 << 4);
        let timer = m.bus.timers.peek(0x1f801100);
        assert!(m
            .commit_ram_phase(&endpoint(&m, 5, 5), &[], &[5], 5)
            .is_err());
        assert_eq!(m.bus.timers.peek(0x1f801100), timer);
        assert_eq!(m.cycles(), 0);
        assert_eq!(m.bus.irq_registers(), (0, 1 << 4));
        assert!(m.take_audio().is_empty());
    }
    #[test]
    fn field_boundary_during_trial_rejects_whole_transaction() {
        let mut m = machine();
        let s = endpoint(&m, 640000, 32);
        assert!(m
            .commit_ram_phase(&s, &[], &vec![32; 20000], 640000)
            .is_err());
        assert_eq!(m.cycles(), 0);
        assert_eq!(m.field(), 0);
        assert_eq!(m.bus.video_acc, 0);
        assert!(m.take_audio().is_empty());
    }
}
