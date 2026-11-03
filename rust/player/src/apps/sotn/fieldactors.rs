//! ST0 actor/3D/HUD typed coordination, 026 §16.12, GPL-3.0-or-later.
//! Prepare all actor sources before Machine commit. No old framebuffer or GP0.
use super::{
    actorclock::ActorClockPlan,
    actorview::ActorView,
    field3d::{Epoch, Role},
    fieldhud, planeview,
};
use psgolem_core::{cpu::Bus, machine::Machine, scene::Canvas};
use std::{path::Path, sync::Arc};
type Result<T> = std::result::Result<T, String>;
fn word(r: &[u8], p: usize) -> u32 {
    u32::from_le_bytes(r[p..p + 4].try_into().unwrap())
}
#[derive(Clone, Default)]
pub struct Coordinator {
    base: fieldhud::Coordinator,
    actors: Option<Arc<ActorView>>,
    prefix: u32,
    native_sources: Option<Arc<(Vec<super::actor::Quad>, Vec<u8>)>>,
}
pub struct Bundle {
    pub base: fieldhud::Bundle,
    pub actors: Arc<ActorView>,
    pub actor_prefix: u32,
}
impl Coordinator {
    pub fn epoch(&self) -> Option<Epoch> {
        self.base.epoch()
    }
    pub fn mark(&self) -> String {
        format!(
            "{} {:?} {:?}",
            self.base.mark(),
            self.actors.as_ref().map(Arc::as_ptr),
            (self.prefix, self.native_sources.as_ref().map(Arc::as_ptr))
        )
    }
    pub fn begin(&mut self, m: &mut Machine) -> Result<Vec<u32>> {
        let costs = self.base.begin(m)?;
        self.actors = None;
        self.prefix = 0;
        self.native_sources = None;
        Ok(costs)
    }
    pub(super) fn after_scroll(&self, m: &Machine) -> Result<Epoch> {
        self.base.after_scroll(m)
    }
    pub(super) fn batch(&self, role: Role) -> Option<&super::field3d::Batch> {
        self.base.batch(role)
    }
    pub fn actors(&mut self, m: &mut Machine, pack: &Path) -> Result<Vec<u32>> {
        self.actors_in_order(m, pack, 104, 495)
    }
    pub(super) fn actors_in_order(
        &mut self,
        m: &mut Machine,
        pack: &Path,
        lower: usize,
        upper: usize,
    ) -> Result<Vec<u32>> {
        self.base.after_scroll(m)?;
        if self.actors.is_some()
            || m.cpu.pc() != 0x800eb998
            || word(m.bus.ram(), 0x97930) != 0
            || word(m.bus.ram(), 0x97938) != 0
        {
            return Err("人物階段、重複或未知圖元前綴".into());
        }
        let plan = ActorClockPlan::prepare(
            m.bus.ram(),
            m.bus.scratch(),
            &m.cpu.state,
            m.bus.gpu.texture_context(),
            m.bus.irq_registers().1,
            m.bus.biu(),
        )?;
        let n = plan.data.geometry.quads.len();
        if !(1..=32).contains(&n) || plan.data.geometry.count_after != n as u32 {
            return Err("ST0人物數量或前綴未封閉".into());
        }
        let mut predicted = m.bus.gpu.clone();
        let jobs: Vec<_> = plan
            .data
            .geometry
            .images
            .iter()
            .map(|j| j.job.clone())
            .collect();
        predicted.apply_texture_jobs(&jobs)?;
        let actors = ActorView::prepare_at(
            &plan.data.geometry.ram,
            &predicted,
            &plan.data.geometry,
            pack,
            0,
            0,
        )?;
        if actors.orders.iter().any(|&o| o < lower || o >= upper) {
            return Err("人物與HUD排序尚未封閉".into());
        }
        let native_sources = Arc::new((plan.data.geometry.quads.clone(), m.bus.scratch().to_vec()));
        let costs = plan.costs().to_vec();
        plan.commit(m)?;
        self.native_sources = Some(native_sources);
        self.actors = Some(Arc::new(actors));
        self.prefix = n as u32;
        Ok(costs)
    }
    pub(super) fn native_sources(&self, m: &Machine) -> Result<(&[super::actor::Quad], &[u8])> {
        self.needs_actors(m)?;
        let source = self.native_sources.as_ref().ok_or("缺少人物原生來源")?;
        Ok((&source.0, &source.1))
    }
    fn needs_actors(&self, m: &Machine) -> Result<()> {
        self.base.check_epoch(m)?;
        if self.actors.is_none() {
            return Err("尚未接收本場人物".into());
        }
        Ok(())
    }
    pub fn plane(&mut self, m: &mut Machine) -> Result<Vec<u32>> {
        self.base.plane(m)
    }
    pub fn tower(&mut self, m: &mut Machine) -> Result<Vec<u32>> {
        self.base.tower(m)
    }
    pub fn scroll(&mut self, m: &Machine) -> Result<()> {
        self.base.scroll(m)
    }
    pub fn primitives(&mut self, m: &mut Machine, pack: &Path) -> Result<Vec<u32>> {
        self.needs_actors(m)?;
        if word(m.bus.ram(), 0x97930) != self.prefix || word(m.bus.ram(), 0x97938) != 0 {
            return Err("人物後出現未知圖元前綴".into());
        }
        let min = *self
            .actors
            .as_ref()
            .unwrap()
            .orders
            .iter()
            .min()
            .ok_or("人物排序缺失")?;
        for role in [Role::Plane, Role::Tower] {
            if self
                .base
                .batch(role)
                .ok_or("缺少3D來源")?
                .quads
                .iter()
                .any(|q| q.order as usize >= min)
            {
                return Err("人物與3D交錯排序未封閉".into());
            }
        }
        self.base.primitives(m, pack)
    }
    pub fn seal(&mut self, m: &Machine) -> Result<Bundle> {
        self.needs_actors(m)?;
        let base = self.base.seal(m)?;
        Ok(Bundle {
            base,
            actors: self.actors.as_ref().unwrap().clone(),
            actor_prefix: self.prefix,
        })
    }
}
impl Bundle {
    pub fn render(&self, r: &[u8], v: &[u16], pack: &Path, scale: usize) -> Result<Canvas> {
        if ![1, 4].contains(&scale) {
            return Err("人物共同畫布倍率未封閉".into());
        }
        let view = planeview::View::from_quads(r, v, self.base.three.quads().to_vec(), pack)?;
        let mut canvas = Canvas {
            width: 256 * scale,
            height: 240 * scale,
            words: vec![self.base.clip.background; 256 * 240 * scale * scale],
        };
        view.paint(&mut canvas, scale)?;
        self.actors.paint(&mut canvas, scale)?;
        self.base.hud.paint(&mut canvas, scale)?;
        self.base.clip.apply(&mut canvas, scale)?;
        Ok(canvas)
    }
}
