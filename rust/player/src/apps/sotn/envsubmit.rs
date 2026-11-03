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
        let clock = super::listclock::Clock::prepare(r, s, biu)?;
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
        let endpoint = clock.endpoint;
        let costs = clock.costs;
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
