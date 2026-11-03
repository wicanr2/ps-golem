//! SotN display control source/ABI, 026 §16.16, GPL-3.0-or-later.
//! Original DISPENV -> typed GPU metadata + shadow RAM, no guest ISA/GP1.
use psgolem_core::{
    cpu::{Bus, State},
    gpu::display::DisplayControl,
    machine::{phase::RamWordPatch, Machine},
};
use sha2::{Digest, Sha256};
type Result<T> = std::result::Result<T, String>;
fn w(r: &[u8], p: usize) -> u32 {
    u32::from_le_bytes(r[p..p + 4].try_into().unwrap())
}
fn h(r: &[u8], p: usize) -> u16 {
    u16::from_le_bytes(r[p..p + 2].try_into().unwrap())
}
pub struct DisplayPlan {
    pub job: DisplayControl,
    pub origin: bool,
    before: State,
    endpoint: State,
    patches: Vec<RamWordPatch>,
    costs: Vec<u32>,
    ram_sha: [u8; 32],
    biu: u32,
}
impl DisplayPlan {
    pub fn prepare(r: &[u8], s: &State, biu: u32) -> Result<Self> {
        if r.len() != 2 << 20
            || s.pc != 0x80014528
            || s.next_pc != s.pc + 4
            || s.bdbt != 0
            || s.branched
            || s.ld_which != 34
            || s.gpr[0] != 0
            || biu != 125320
            || s.cop0[12] & (1 << 16) != 0
            || s.cop0[13] & 0x400 != 0
            || s.cop0[12] & s.cop0[13] & 0xff00 != 0 && s.cop0[12] & 1 != 0
            || s.icache.iter().any(|l| l.valid & !15 != 0)
            || s.now > u64::MAX - 128
        {
            return Err("顯示控制入口／CPU／cache不符".into());
        }
        if format!("{:x}", Sha256::digest(&r[0x14528..0x14560]))
            != "5187295a25db14950899f1b48c158a534c59790f78004935f397327e1bc77ee7"
            || format!("{:x}", Sha256::digest(&r[0x180000..0x180024]))
                != "ce14842f7f0101760916814b497603a5db6241409581fff963d5fb5fd2154a1b"
            || w(r, 0x2c280) != 0x1f801814
        {
            return Err("顯示helper／ST0或MMIO指標身份不符".into());
        }
        let b = match w(r, 0x6c37c) {
            0x8003cb08 => 0x3cb08,
            0x800542fc => 0x542fc,
            _ => return Err("顯示來源不是已驗雙緩衝".into()),
        };
        let p = b + 96;
        let x = if b == 0x3cb08 { 256 } else { 0 };
        if [h(r, p), h(r, p + 2), h(r, p + 4), h(r, p + 6)] != [x, 0, 256, 240]
            || r[p + 8..p + 20] != [0; 12]
        {
            return Err("ST0顯示結構形狀未封閉".into());
        }
        let (job, origin, index, payload) = match s.gpr[31] {
            0x800130a4 => {
                if s.gpr[4] != (0x05000000 | x as u32) {
                    return Err("顯示原點不是來源值".into());
                }
                (DisplayControl::Origin { x, y: 0 }, true, 5, x as u32)
            }
            0x800133b0 => {
                if s.gpr[4] != 0x08000000 {
                    return Err("顯示模式不是來源值".into());
                }
                (DisplayControl::Ntsc256x240, false, 8, 0)
            }
            _ => return Err("顯示呼叫者未封閉".into()),
        };
        let mut endpoint = s.clone();
        let mut costs = Vec::new();
        for pc in (0x14528..=0x1455c).step_by(4) {
            let slot = (pc >> 2) & 3;
            let line = &mut endpoint.icache[((pc >> 4) & 255) as usize];
            let mut n = 1;
            if line.tag != pc >> 12 || line.valid & (1 << slot) == 0 {
                let fill = ((15 << slot) & 15) as u8;
                line.valid = if line.tag == pc >> 12 {
                    line.valid | fill
                } else {
                    fill
                };
                line.tag = pc >> 12;
                n += 7 - slot;
            }
            if pc == 0x14530 {
                n += 4;
            }
            endpoint.now += n as u64;
            endpoint.cost = n as u64;
            costs.push(n);
        }
        let at = 0x37ed0 + index * 4;
        endpoint.gpr[1] = 0x80000000 + at;
        endpoint.gpr[2] = index * 4;
        endpoint.gpr[3] = 0xffffff;
        endpoint.gpr[4] = payload;
        endpoint.gpr[34] = 0x1f801814;
        endpoint.ld_value = 0x1f801814;
        endpoint.ld_which = 34;
        endpoint.pc = s.gpr[31];
        endpoint.next_pc = endpoint.pc + 4;
        endpoint.bdbt = 0;
        endpoint.branched = false;
        Ok(Self {
            job,
            origin,
            before: s.clone(),
            endpoint,
            patches: vec![RamWordPatch {
                offset: at,
                value: payload,
            }],
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
            return Err("顯示控制計畫已過期".into());
        }
        m.commit_display_data_phase(&self.endpoint, &self.patches, &self.costs, 4, self.job)
    }
}
