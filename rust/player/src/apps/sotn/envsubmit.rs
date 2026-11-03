//! SotN environment SDK submission, 026 §16.15, GPL-3.0-or-later.
//! Data model + resource clock; never executes guest ISA or decodes GP0.
use psgolem_core::{
    cpu::{Bus, State},
    gpu::{environment::DrawEnvironment, transfer::Rect},
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
pub struct EnvPlan {
    pub job: DrawEnvironment,
    before: State,
    endpoint: State,
    costs: Vec<u32>,
    ram_sha: [u8; 32],
    biu: u32,
}
impl EnvPlan {
    pub fn prepare(r: &[u8], s: &State, biu: u32) -> Result<Self> {
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
        let buffer = match s.gpr[4] {
            0x8003cb28 => 0x3cb08,
            0x8005431c => 0x542fc,
            _ => return Err("非已驗環境提交".into()),
        };
        let p = buffer + 4;
        let x = if buffer == 0x3cb08 { 0 } else { 256 };
        if [
            h(r, p),
            h(r, p + 2),
            h(r, p + 4),
            h(r, p + 6),
            h(r, p + 8),
            h(r, p + 10),
        ] != [x, 20, 256, 207, x, 0]
            || r[p + 12..p + 20] != [0; 8]
            || h(r, p + 20) != 10
            || r[p + 22..p + 25] != [0, 1, 1]
        {
            return Err("ST0環境來源形狀未封閉".into());
        }
        let job = DrawEnvironment {
            area: Rect {
                x,
                y: 20,
                width: 256,
                height: 207,
            },
            offset: [x as i16, 0],
            page: 0x40a,
            color: [r[p + 25], r[p + 26], r[p + 27]],
        };
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
        Ok(Self {
            job,
            before: s.clone(),
            endpoint,
            costs,
            ram_sha: Sha256::digest(r).into(),
            biu,
        })
    }
    pub fn costs(&self) -> &[u32] {
        &self.costs
    }
    pub fn endpoint(&self) -> &State {
        &self.endpoint
    }
    pub fn commit(&self, m: &mut Machine) -> Result<()> {
        if m.cpu.state != self.before
            || m.bus.biu() != self.biu
            || <[u8; 32]>::from(Sha256::digest(m.bus.ram())) != self.ram_sha
        {
            return Err("環境提交計畫已過期".into());
        }
        m.commit_environment_data_phase(&self.endpoint, &self.costs, self.job)
    }
}
