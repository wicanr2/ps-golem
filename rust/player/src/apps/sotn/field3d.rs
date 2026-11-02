//! Staged two-source ST0 frame work, 026 §16.10, GPL-3.0-or-later.
//! Raw typed producers -> epoch/ownership/scroll -> one ordered bundle.
//! No original ISA, GP0 parsing, old pool XY or native framebuffer input.
use super::{
    camera::Sources,
    frame::{EntryRegisters, FramePrepPlan},
    frameclock::FrameClockPlan,
    planelife::PlanePlan,
    planes::Quad,
    tower::TowerPlan,
};
use psgolem_core::{cpu::Bus, machine::Machine};
use sha2::{Digest, Sha256};
type Result<T> = std::result::Result<T, String>;
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct Epoch {
    pub counter: u32,
    pub draw_buffer: u32,
    pub ordering_start: u32,
}
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Role {
    Plane,
    Tower,
}
impl Role {
    fn index(self) -> usize {
        match self {
            Self::Plane => 0,
            Self::Tower => 1,
        }
    }
}
#[derive(Clone, Debug)]
pub struct Batch {
    pub role: Role,
    pub quads: Vec<Quad>,
    pub hidden: Vec<usize>,
}
#[derive(Clone, Debug)]
pub struct Bundle {
    pub epoch: Epoch,
    quads: Vec<Quad>,
    pub hidden: Vec<usize>,
    pub plane_quads: usize,
    pub tower_faces: usize,
}
impl Bundle {
    pub fn quads(&self) -> &[Quad] {
        &self.quads
    }
}
#[derive(Clone, Debug, Default)]
pub struct Coordinator {
    epoch: Option<Epoch>,
    batches: [Option<Batch>; 2],
    scrolled: bool,
    sealed: bool,
    pub dropped_incomplete: u64,
}
fn w(r: &[u8], p: usize) -> u32 {
    u32::from_le_bytes(r[p..p + 4].try_into().unwrap())
}
fn current(m: &Machine) -> Epoch {
    let r = m.bus.ram();
    Epoch {
        counter: w(r, 0x3c998),
        draw_buffer: w(r, 0x6c37c),
        ordering_start: w(r, 0x13556c),
    }
}
impl Coordinator {
    pub fn epoch(&self) -> Option<Epoch> {
        self.epoch
    }
    pub fn batch(&self, role: Role) -> Option<&Batch> {
        self.batches[role.index()].as_ref()
    }
    pub fn begin(&mut self, m: &mut Machine) -> Result<Vec<u32>> {
        Sources::new(m.bus.ram())?;
        if format!("{:x}", Sha256::digest(&m.bus.ram()[0x1ba70c..0x1baeb0]))
            != "de1e58a6352e4714bb50592730d2f9a83c534d5d51a512795750853664eb9fda"
        {
            return Err("共同場的ST0平面來源不符".into());
        }
        if self.epoch.is_some_and(|k| k != current(m)) {
            return Err("共同場前一Epoch與機器不符".into());
        }
        let s = &m.cpu.state;
        let r = FramePrepPlan::prepare(
            m.bus.ram(),
            EntryRegisters {
                pc: s.pc,
                sp: s.gpr[29],
                s0: s.gpr[16],
                s1: s.gpr[17],
            },
        )?
        .resources();
        let plan = FrameClockPlan::prepare(m.bus.ram(), s, m.bus.biu())?;
        let key = Epoch {
            counter: r.frame_counter,
            draw_buffer: r.draw_buffer,
            ordering_start: r.ordering_start,
        };
        let costs = plan.costs().to_vec();
        plan.commit(m)?;
        if self.epoch.is_some() && !self.sealed {
            self.dropped_incomplete += 1;
        }
        self.epoch = Some(key);
        self.batches = [None, None];
        self.scrolled = false;
        self.sealed = false;
        Ok(costs)
    }
    fn check(&self, m: &Machine) -> Result<Epoch> {
        let key = self.epoch.ok_or("共同場尚未開始")?;
        if key != current(m) || self.sealed {
            return Err("共同場Epoch過期或已發布".into());
        }
        Ok(key)
    }
    /// Preflight on a detached clone; no admission error can follow Machine commit.
    pub fn admit(&mut self, key: Epoch, batch: Batch) -> Result<()> {
        if self.epoch != Some(key)
            || self.sealed
            || self.scrolled
            || self.batches[batch.role.index()].is_some()
        {
            return Err("共同場來源重複、過期或階段不符".into());
        }
        let mut handles = batch.hidden.clone();
        for q in &batch.quads {
            if q.page != 15
                || ![0, 2, 0x15].contains(&q.flags)
                || q.order >= 512
                || q.clut_index >= 512
                || q.points.iter().enumerate().any(|(i, p)| {
                    !p.depth.is_finite()
                        || p.position.iter().any(|v| !v.is_finite())
                        || p.uv != q.uv[i].map(|v| v as i32)
                })
            {
                return Err("共同場幾何／材質域未封閉".into());
            }
            handles.push(q.handle);
        }
        let count = match batch.role {
            Role::Plane => 112,
            Role::Tower => 40,
        };
        if ![0, count].contains(&handles.len()) || handles.iter().any(|&h| h >= 1280) {
            return Err("共同場來源槽數或範圍不符".into());
        }
        handles.sort_unstable();
        if handles.windows(2).any(|p| p[0] == p[1]) {
            return Err("共同場來源槽重複".into());
        }
        for old in self.batches.iter().flatten() {
            if old
                .hidden
                .iter()
                .chain(old.quads.iter().map(|q| &q.handle))
                .any(|h| handles.binary_search(h).is_ok())
            {
                return Err("共同場跨來源槽碰撞".into());
            }
        }
        let i = batch.role.index();
        self.batches[i] = Some(batch);
        Ok(())
    }
    pub fn plane(&mut self, m: &mut Machine) -> Result<Vec<u32>> {
        let key = self.check(m)?;
        if m.cpu.gpr(4) != 0x8007cc98 {
            return Err("共同場平面來源物件不符".into());
        }
        let p = PlanePlan::from_machine(m)?;
        let batch = Batch {
            role: Role::Plane,
            quads: p.geometry.quads.clone(),
            hidden: p.geometry.hidden_handles.clone(),
        };
        let mut next = self.clone();
        next.admit(key, batch)?;
        let costs = p.costs().to_vec();
        p.commit(m)?;
        *self = next;
        Ok(costs)
    }
    pub fn tower(&mut self, m: &mut Machine) -> Result<Vec<u32>> {
        let key = self.check(m)?;
        if m.cpu.gpr(4) != 0x8007d278 {
            return Err("共同場鐘塔來源物件不符".into());
        }
        let p = TowerPlan::prepare(m)?;
        let quads = p
            .data()
            .draws
            .iter()
            .map(|d| Quad {
                handle: d.handle,
                native: d.native,
                points: d.points,
                vertices: d.face.vertices,
                camera: None,
                uv: d.face.uv,
                clut_index: d.face.clut_index,
                order: d.face.order,
                page: d.page,
                flags: d.flags,
            })
            .collect();
        let mut next = self.clone();
        next.admit(
            key,
            Batch {
                role: Role::Tower,
                quads,
                hidden: p.data().hidden_handles.clone(),
            },
        )?;
        let costs = p.costs().to_vec();
        p.commit(m)?;
        *self = next;
        Ok(costs)
    }
    pub(super) fn after_scroll(&self, m: &Machine) -> Result<Epoch> {
        let key = self.check(m)?;
        if !self.scrolled || self.batches.iter().any(Option::is_none) {
            return Err("共同場尚未完成捲動與來源".into());
        }
        Ok(key)
    }
    pub fn scroll(&mut self, m: &Machine) -> Result<()> {
        self.check(m)?;
        if self.scrolled || self.batches.iter().any(Option::is_none) {
            return Err("共同場捲動重複或來源不足".into());
        }
        let sources = Sources::new(m.bus.ram())?;
        let mut next = self.clone();
        // The source API also validates the post-scroll PC and S0=1024.
        sources.mesh_scroll_after_event(&m.cpu.state, 0, 0)?;
        for batch in next.batches.iter_mut().flatten() {
            for q in &mut batch.quads {
                let delta = sources.mesh_scroll_after_event(&m.cpu.state, q.handle, q.flags)?;
                for (i, p) in q.points.iter_mut().enumerate() {
                    for axis in 0..2 {
                        q.native[i][axis] = q.native[i][axis].wrapping_add(delta[axis]);
                        p.position[axis] = if p.depth > 0.0 {
                            p.position[axis] + delta[axis] as f64
                        } else {
                            q.native[i][axis] as f64
                        };
                    }
                }
            }
        }
        next.scrolled = true;
        *self = next;
        Ok(())
    }
    pub fn seal(&mut self, m: &Machine) -> Result<Bundle> {
        let key = self.check(m)?;
        if m.cpu.pc() != 0x800e3754 || !self.scrolled || self.batches.iter().any(Option::is_none) {
            return Err("共同場尚未到完整排序發布邊界".into());
        }
        let mut quads = Vec::new();
        let mut hidden = Vec::new();
        for batch in self.batches.iter().flatten() {
            quads.extend(batch.quads.clone());
            hidden.extend_from_slice(&batch.hidden);
        }
        quads.sort_by(|a, b| a.order.cmp(&b.order).then(b.handle.cmp(&a.handle)));
        hidden.sort_unstable();
        let bundle = Bundle {
            epoch: key,
            quads,
            hidden,
            plane_quads: self.batches[0].as_ref().unwrap().quads.len(),
            tower_faces: self.batches[1].as_ref().unwrap().quads.len(),
        };
        self.sealed = true;
        Ok(bundle)
    }
}
