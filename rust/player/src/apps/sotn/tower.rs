//! Unified whole-call tower adapter, 026 §16.9.12, GPL-3.0-or-later.
//! Caller enables this only for HD takeover; native execution is untouched.
use super::{meshstate::DataPlan, toweractive::TowerActivePlan, towerinit::TowerInitPlan};
use psgolem_core::{cpu::Bus, cpu::State, machine::Machine};
pub enum TowerPlan {
    Initialize(TowerInitPlan),
    Active(TowerActivePlan),
}
impl TowerPlan {
    pub fn prepare(m: &Machine) -> Result<Self, String> {
        if m.cpu.pc() != 0x801baeb0 {
            return Err("不是鐘塔完整呼叫入口".into());
        }
        let e = m
            .cpu
            .gpr(4)
            .checked_sub(0x80000000)
            .ok_or("鐘塔物件位址不符")? as usize;
        let step = m.bus.ram().get(e + 0x2c..e + 0x2e).ok_or("鐘塔物件越界")?;
        if u16::from_le_bytes(step.try_into().unwrap()) == 0 {
            TowerInitPlan::prepare(m.bus.ram(), &m.cpu.state, &m.cpu.gte, m.bus.biu())
                .map(Self::Initialize)
        } else {
            TowerActivePlan::prepare(m.bus.ram(), &m.cpu.state, &m.cpu.gte, m.bus.biu())
                .map(Self::Active)
        }
    }
    pub fn is_initialization(&self) -> bool {
        matches!(self, Self::Initialize(_))
    }
    pub fn data(&self) -> &DataPlan {
        match self {
            Self::Initialize(p) => &p.data,
            Self::Active(p) => &p.data,
        }
    }
    pub fn endpoint(&self) -> &State {
        match self {
            Self::Initialize(p) => p.endpoint(),
            Self::Active(p) => p.endpoint(),
        }
    }
    pub fn costs(&self) -> &[u32] {
        match self {
            Self::Initialize(p) => p.costs(),
            Self::Active(p) => p.costs(),
        }
    }
    pub fn commit(&self, m: &mut Machine) -> Result<(), String> {
        match self {
            Self::Initialize(p) => p.commit(m),
            Self::Active(p) => p.commit(m),
        }
    }
}
