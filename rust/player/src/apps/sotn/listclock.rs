//! Shared verified list-helper CPU/resource endpoint, 026 §16.15/16.20.
//! GPL-3.0-or-later. No guest ISA, GP0/GP1 parsing or RAM-list reads.
use psgolem_core::cpu::State;
use sha2::{Digest, Sha256};
fn w(r: &[u8], p: usize) -> u32 {
    u32::from_le_bytes(r[p..p + 4].try_into().unwrap())
}
pub struct Clock {
    pub endpoint: State,
    pub costs: Vec<u32>,
}
impl Clock {
    pub fn prepare(r: &[u8], s: &State, biu: u32) -> Result<Self, String> {
        if r.len() != 2 << 20
            || s.pc != 0x800145d0
            || s.next_pc != s.pc + 4
            || s.bdbt != 0
            || s.branched
            || s.ld_which != 34
            || s.gpr[0] != 0
            || s.gpr[31] & 3 != 0
            || !(0x80000000..0x80200000).contains(&s.gpr[31])
            || biu != 125320
            || s.cop0[12] & (1 << 16) != 0
            || s.cop0[13] & 0x400 != 0
            || s.cop0[12] & s.cop0[13] & 0xff00 != 0 && s.cop0[12] & 1 != 0
            || s.icache.iter().any(|l| l.valid & !15 != 0)
            || s.now > u64::MAX - 256
        {
            return Err("環境SDK入口／CPU／cache未封閉".into());
        }
        if format!("{:x}", Sha256::digest(&r[0x145d0..0x1461c]))
            != "96a75f8c9ecd2647157165e1b5f1a002c0f55738d9bc2b67e6aba17e0e6f3bcc"
            || format!("{:x}", Sha256::digest(&r[0x180000..0x180024]))
                != "ce14842f7f0101760916814b497603a5db6241409581fff963d5fb5fd2154a1b"
        {
            return Err("環境SDK或ST0來源身份不符".into());
        }
        for (p, value) in [
            (0x2c280, 0x1f801814),
            (0x2c284, 0x1f8010a0),
            (0x2c288, 0x1f8010a4),
            (0x2c28c, 0x1f8010a8),
        ] {
            if w(r, p) != value {
                return Err("SDK MMIO來源指標不符".into());
            }
        }
        let mut endpoint = s.clone();
        let mut costs = Vec::new();
        for pc in (0x145d0..=0x14618).step_by(4) {
            let slot = (pc >> 2) & 3;
            let line = &mut endpoint.icache[((pc >> 4) & 255) as usize];
            let mut cost = 1;
            if line.tag != pc >> 12 || line.valid & (1 << slot) == 0 {
                let fill = ((15 << slot) & 15) as u8;
                line.valid = if line.tag == pc >> 12 {
                    line.valid | fill
                } else {
                    fill
                };
                line.tag = pc >> 12;
                cost += 7 - slot;
            }
            if [0x145d8, 0x145e8, 0x145f8, 0x14608].contains(&pc) {
                cost += 4;
            }
            endpoint.now += cost as u64;
            endpoint.cost = cost as u64;
            costs.push(cost);
        }
        endpoint.gpr[2] = 0x1f8010a8;
        endpoint.gpr[3] = 0x01000401;
        endpoint.gpr[34] = 0x1f8010a8;
        endpoint.ld_which = 34;
        endpoint.ld_value = 0x1f8010a8;
        endpoint.pc = s.gpr[31];
        endpoint.next_pc = endpoint.pc + 4;
        endpoint.bdbt = 0;
        endpoint.branched = false;
        Ok(Self { endpoint, costs })
    }
}
