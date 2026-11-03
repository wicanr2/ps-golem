//! ST0 3D + HUD visible-pool handoff, 026 §16.11, GPL-3.0-or-later.
//! Original page/UV data is bound before atomic compatibility submission.
use super::{
    field3d::{self, Epoch, Role},
    hudst0::{half, Hud},
    planeview,
    primitiveclock::PrimitiveClockPlan,
};
use psgolem_core::{cpu::Bus, machine::Machine, scene::Canvas};
use std::{path::Path, sync::Arc};
type Result<T> = std::result::Result<T, String>;
fn word(r: &[u8], p: usize) -> u32 {
    u32::from_le_bytes(r[p..p + 4].try_into().unwrap())
}
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct Clip {
    pub left: usize,
    pub top: usize,
    pub right: usize,
    pub bottom: usize,
    pub background: u16,
}
impl Clip {
    pub fn read(r: &[u8]) -> Result<Self> {
        if r.len() != 2 << 20 {
            return Err("HUD環境來源大小不符".into());
        }
        let b = word(r, 0x6c37c)
            .checked_sub(0x80000000)
            .ok_or("HUD環境指標不符")? as usize;
        if b + 96 > r.len() {
            return Err("HUD環境越界".into());
        }
        let env = &r[b + 4..b + 96];
        let x = half(env, 0) as i16 as i32;
        let y = half(env, 2) as i16 as i32;
        let w = half(env, 4) as i16 as i32;
        let h = half(env, 6) as i16 as i32;
        let left = x - half(env, 8) as i16 as i32;
        let top = y - half(env, 10) as i16 as i32;
        if env[22] != 0
            || env[23] == 0
            || env[24] != 1
            || w != 256
            || h <= 0
            || left != 0
            || top < 0
            || top + h > 240
        {
            return Err("HUD畫布環境分支未封閉".into());
        }
        let background =
            (env[25] as u16 >> 3) | ((env[26] as u16 >> 3) << 5) | ((env[27] as u16 >> 3) << 10);
        Ok(Self {
            left: left as usize,
            top: top as usize,
            right: (left + w) as usize,
            bottom: (top + h) as usize,
            background,
        })
    }
    pub fn apply(&self, c: &mut Canvas, scale: usize) -> Result<()> {
        if ![1, 4].contains(&scale) || c.width != 256 * scale || c.height != 240 * scale {
            return Err("HUD裁切畫布尺寸不符".into());
        }
        for y in 0..c.height {
            for x in 0..c.width {
                if x < self.left * scale
                    || x >= self.right * scale
                    || y < self.top * scale
                    || y >= self.bottom * scale
                {
                    c.words[y * c.width + x] = 0;
                }
            }
        }
        Ok(())
    }
}
#[derive(Clone, Default)]
pub struct Coordinator {
    three: field3d::Coordinator,
    hud: Option<Arc<Hud>>,
    clip: Option<Clip>,
    submitted: bool,
    pub visible_pool: usize,
}
pub struct Bundle {
    pub three: field3d::Bundle,
    pub hud: Arc<Hud>,
    pub clip: Clip,
    pub visible_pool: usize,
}
impl Bundle {
    pub fn render(&self, r: &[u8], v: &[u16], pack: &Path, scale: usize) -> Result<Canvas> {
        if ![1, 4].contains(&scale) {
            return Err("HUD共同畫布倍率未封閉".into());
        }
        let view = planeview::View::from_quads(r, v, self.three.quads().to_vec(), pack)?;
        let mut c = Canvas {
            width: 256 * scale,
            height: 240 * scale,
            words: vec![self.clip.background; 256 * 240 * scale * scale],
        };
        view.paint(&mut c, scale)?;
        self.hud.paint(&mut c, scale)?;
        self.clip.apply(&mut c, scale)?;
        Ok(c)
    }
}
impl Coordinator {
    pub(super) fn check_epoch(&self, m: &Machine) -> Result<Epoch> {
        self.three.check(m)
    }
    pub(super) fn batch(&self, role: Role) -> Option<&field3d::Batch> {
        self.three.batch(role)
    }
    pub(super) fn after_scroll(&self, m: &Machine) -> Result<Epoch> {
        self.three.after_scroll(m)
    }
    pub fn epoch(&self) -> Option<Epoch> {
        self.three.epoch()
    }
    pub fn mark(&self) -> String {
        format!(
            "{:?} {:?} {:?} {} {}",
            self.three,
            self.hud.as_ref().map(Arc::as_ptr),
            self.clip,
            self.submitted,
            self.visible_pool
        )
    }
    pub fn begin(&mut self, m: &mut Machine) -> Result<Vec<u32>> {
        let c = self.three.begin(m)?;
        self.hud = None;
        self.clip = None;
        self.submitted = false;
        self.visible_pool = 0;
        Ok(c)
    }
    pub fn plane(&mut self, m: &mut Machine) -> Result<Vec<u32>> {
        if self.submitted {
            return Err("HUD交接後不能再更新平面".into());
        }
        self.three.plane(m)
    }
    pub fn tower(&mut self, m: &mut Machine) -> Result<Vec<u32>> {
        if self.submitted {
            return Err("HUD交接後不能再更新鐘塔".into());
        }
        self.three.tower(m)
    }
    pub fn scroll(&mut self, m: &Machine) -> Result<()> {
        if self.submitted {
            return Err("HUD交接後不能重做捲動".into());
        }
        self.three.scroll(m)
    }
    pub fn primitives(&mut self, m: &mut Machine, pack: &Path) -> Result<Vec<u32>> {
        self.three.after_scroll(m)?;
        if self.submitted || m.cpu.pc() != 0x800edc94 {
            return Err("HUD通用圖元交接階段不符或重複".into());
        }
        if word(m.bus.ram(), 0xbd284) != 0 {
            return Err("條件矩形繪圖尚未接入HUD共同場".into());
        }
        let hud = Hud::prepare(m.bus.ram(), &m.bus.gpu, pack)?;
        let clip = Clip::read(m.bus.ram())?;
        let min_order = *hud.orders.iter().min().ok_or("缺少HUD排序")?;
        let mut owners = vec![false; 1280];
        for role in [Role::Plane, Role::Tower] {
            for q in &self.three.batch(role).ok_or("缺少3D來源")?.quads {
                if q.order >= min_order || owners[q.handle] {
                    return Err("HUD與3D排序或槽位衝突".into());
                }
                owners[q.handle] = true;
            }
        }
        for &id in &hud.records {
            if owners[id] {
                return Err("HUD槽位與3D碰撞".into());
            }
            owners[id] = true;
        }
        let r = m.bus.ram();
        let mut visible = 0;
        for i in 0..1280 {
            let p = 0x86fec + i * 52;
            if r[p + 7] != 0 && half(r, p + 50) & 8 == 0 {
                if !owners[i] {
                    return Err(format!("可見池尚有未接管來源 {i}"));
                }
                visible += 1;
            }
        }
        let plan = PrimitiveClockPlan::prepare(r, m.bus.scratch(), &m.cpu.state, m.bus.biu())?;
        let costs = plan.costs().to_vec();
        plan.commit(m)?;
        self.hud = Some(Arc::new(hud));
        self.clip = Some(clip);
        self.submitted = true;
        self.visible_pool = visible;
        Ok(costs)
    }
    pub fn seal(&mut self, m: &Machine) -> Result<Bundle> {
        if !self.submitted || self.hud.is_none() || self.clip.is_none() {
            return Err("HUD尚未完成交接，不發布共同場".into());
        }
        let three = self.three.seal(m)?;
        Ok(Bundle {
            three,
            hud: self.hud.as_ref().unwrap().clone(),
            clip: self.clip.unwrap(),
            visible_pool: self.visible_pool,
        })
    }
}
