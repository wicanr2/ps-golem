//! ST0 typed tile/actor/3D/HUD composition, 026 §16.13, GPL-3.0-or-later.
use super::{
    call::CallPlan,
    field3d::{Epoch, Role},
    fieldactors, planeview, tileview,
};
use psgolem_core::{cpu::Bus, machine::Machine, scene::Canvas};
use std::{path::Path, sync::Arc};
type Result<T> = std::result::Result<T, String>;
fn word(r: &[u8], p: usize) -> u32 {
    u32::from_le_bytes(r[p..p + 4].try_into().unwrap())
}
#[derive(Clone, Default)]
pub struct Coordinator {
    base: fieldactors::Coordinator,
    tiles: Option<Arc<tileview::View>>,
}
pub struct Bundle {
    pub base: fieldactors::Bundle,
    pub tiles: Arc<tileview::View>,
}
impl Coordinator {
    pub fn epoch(&self) -> Option<Epoch> {
        self.base.epoch()
    }
    pub fn mark(&self) -> String {
        format!(
            "{} {:?}",
            self.base.mark(),
            self.tiles.as_ref().map(Arc::as_ptr)
        )
    }
    pub fn begin(&mut self, m: &mut Machine) -> Result<Vec<u32>> {
        let c = self.base.begin(m)?;
        self.tiles = None;
        Ok(c)
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
    pub fn tiles(&mut self, m: &mut Machine, gfx: &[u8], pack: &Path) -> Result<Vec<u32>> {
        self.base.after_scroll(m)?;
        if self.tiles.is_some() {
            return Err("圖塊重複提交".into());
        }
        for role in [Role::Plane, Role::Tower] {
            if self
                .base
                .batch(role)
                .ok_or("缺少3D來源")?
                .quads
                .iter()
                .any(|q| q.order >= 96)
            {
                return Err("3D與圖塊交錯排序尚未封閉".into());
            }
        }
        let plan = CallPlan::prepare_st0(m.bus.ram(), m.bus.scratch(), &m.cpu.state, m.bus.biu())?;
        let view = tileview::View::prepare(m.bus.ram(), m.bus.gpu.vram(), gfx, plan.tiles(), pack)?;
        let costs = plan.costs().to_vec();
        plan.commit(m)?;
        self.tiles = Some(Arc::new(view));
        Ok(costs)
    }
    pub fn actors(&mut self, m: &mut Machine, pack: &Path) -> Result<Vec<u32>> {
        if self.tiles.is_none() {
            return Err("人物之前尚缺圖塊".into());
        }
        self.base.actors_in_order(m, pack, 104, 160)
    }
    pub fn primitives(&mut self, m: &mut Machine, pack: &Path) -> Result<Vec<u32>> {
        let tiles = self.tiles.as_ref().ok_or("未完成圖塊")?;
        if word(m.bus.ram(), 0x97940) != tiles.tiles.len() as u32
            || word(m.bus.ram(), 0x9792c) != 48
        {
            return Err("圖塊後出現未知精靈或模式命令".into());
        }
        self.base.primitives(m, pack)
    }
    pub fn seal(&mut self, m: &Machine) -> Result<Bundle> {
        if self.tiles.is_none() {
            return Err("缺少本場圖塊，不發布".into());
        }
        let base = self.base.seal(m)?;
        Ok(Bundle {
            base,
            tiles: self.tiles.as_ref().unwrap().clone(),
        })
    }
}
impl Bundle {
    pub fn render(&self, r: &[u8], v: &[u16], pack: &Path, scale: usize) -> Result<Canvas> {
        if ![1, 4].contains(&scale) {
            return Err("共同場倍率未封閉".into());
        }
        let base = &self.base.base;
        let view = planeview::View::from_quads(r, v, base.three.quads().to_vec(), pack)?;
        let mut c = Canvas {
            width: 256 * scale,
            height: 240 * scale,
            words: vec![base.clip.background; 256 * 240 * scale * scale],
        };
        view.paint(&mut c, scale)?;
        self.tiles.paint_background(&mut c, scale)?;
        self.base.actors.paint(&mut c, scale)?;
        self.tiles.paint_foreground(&mut c, scale)?;
        base.hud.paint(&mut c, scale)?;
        base.clip.apply(&mut c, scale)?;
        Ok(c)
    }
}
