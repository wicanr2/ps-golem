//! Player ST0 dispatch and whole-field publication, 026 §16.21.
//! GPL-3.0-or-later. Only READY producers are admitted; native mode runs original.
use super::{
    fieldscene::Coordinator,
    order::OrderPlan,
    present::{Frames, Picture},
};
use psgolem_core::{
    cpu::Bus,
    disc::Disc,
    gpu::{deferred::SceneToken, transfer::Rect},
    machine::Machine,
};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::{
    path::{Path, PathBuf},
    sync::Arc,
};
type Result<T> = std::result::Result<T, String>;
pub struct PlayerScene {
    pack: PathBuf,
    gfx: Vec<u8>,
    coordinator: Coordinator,
    frames: Frames,
    pending: Option<Picture>,
    owners: [Option<SceneToken>; 2],
    active: bool,
    enabled: bool,
    last_epoch: Option<u32>,
    pub steps: u64,
    pub submissions: u64,
    pub hd_fields: u64,
    pub native_fields: u64,
    pub errors: Vec<Value>,
    pub mode_events: Vec<Value>,
    pub last_scene: Option<u32>,
    pub phase_failures: u64,
    pub ownership_drops: u64,
    pub last_error: Option<Value>,
}
impl PlayerScene {
    pub fn load(disc: &Disc, pack: &Path) -> Result<Self> {
        if disc.boot_file() != "SLPM_860.23"
            || !pack.is_dir()
            || !pack.join("manifest.json").is_file()
        {
            return Err("月下HD需要已驗光碟及素材目錄".into());
        }
        let gfx = disc.read_file_limit("ST/ST0/F_ST0.BIN", 1024 * 1024)?;
        if format!("{:x}", Sha256::digest(&gfx))
            != "63340ba46ee070511824bafa5d10c7465d58618323a44965e543c73495ab3c63"
        {
            return Err("ST0原圖像來源不符".into());
        }
        Ok(Self {
            pack: pack.into(),
            gfx,
            coordinator: Coordinator::default(),
            frames: Frames::default(),
            pending: None,
            owners: [None, None],
            active: false,
            enabled: false,
            last_epoch: None,
            steps: 0,
            submissions: 0,
            hd_fields: 0,
            native_fields: 0,
            errors: Vec::new(),
            mode_events: Vec::new(),
            last_scene: None,
            phase_failures: 0,
            ownership_drops: 0,
            last_error: None,
        })
    }
    fn reset(&mut self) {
        self.coordinator = Coordinator::default();
        self.frames = Frames::default();
        self.pending = None;
        self.owners = [None, None];
        self.active = false;
        self.last_epoch = None;
        self.last_scene = None;
    }
    pub fn set_mode(&mut self, m: &mut Machine, enabled: bool) {
        if self.enabled != enabled {
            self.reset();
            if !enabled {
                m.bus.gpu.resolve_native_pixels();
            }
            self.enabled = enabled;
            if self.mode_events.len() < 64 {
                self.mode_events
                    .push(json!({"field":m.field(),"cycles":m.cycles(),"HD":enabled}));
            }
        }
    }
    fn supported(m: &Machine) -> bool {
        let r = m.bus.ram();
        format!("{:x}", Sha256::digest(&r[0x180000..0x180024]))
            == "ce14842f7f0101760916814b497603a5db6241409581fff963d5fb5fd2154a1b"
    }
    /// True means a complete modeled phase advanced the machine already.
    pub fn step(&mut self, m: &mut Machine) -> bool {
        if !self.enabled {
            return false;
        }
        let pc = m.cpu.pc();
        if pc == 0x800e3b80 {
            if !Self::supported(m) {
                self.reset();
                return false;
            }
            self.active = true;
        }
        if !self.active {
            return false;
        }
        let phase = match pc {
            0x800e3b80 => "begin",
            0x801ba70c if m.cpu.gpr(4) == 0x8007cc98 => "plane",
            0x801baeb0 if m.cpu.gpr(4) == 0x8007d278 => "tower",
            0x800ecc10 => "tiles",
            0x800eb998 => "actors",
            0x800edc94 => "primitives",
            0x800e3754 => "order",
            0x800145d0 if [0x8003cb28, 0x8005431c].contains(&m.cpu.gpr(4)) => "environment",
            0x800145d0 if [0x8003cf7c, 0x80054770].contains(&m.cpu.gpr(4)) => "submit",
            0x80014528 if [0x800130a4, 0x800133b0].contains(&m.cpu.gpr(31)) => "display",
            0x800f352c if m.cpu.gpr(16) == 1024 => "scroll",
            _ => return false,
        };
        let cycles = m.cycles();
        let result = self.phase(m, phase);
        if let Err(error) = result {
            let e = json!({"field":m.field(),"pc":pc,"phase":phase,"error":error});
            self.phase_failures += 1;
            self.last_error = Some(e.clone());
            if self.errors.len() < 64 {
                self.errors.push(e);
            }
            self.reset();
        }
        let consumed = m.cycles() != cycles;
        if consumed {
            self.steps += 1;
        }
        consumed
    }
    fn phase(&mut self, m: &mut Machine, phase: &str) -> Result<()> {
        match phase {
            "begin" => {
                self.coordinator.begin(m)?;
            }
            "plane" => {
                self.coordinator.plane(m)?;
            }
            "tower" => {
                self.coordinator.tower(m)?;
            }
            "scroll" => {
                self.coordinator.scroll(m)?;
            }
            "tiles" => {
                self.coordinator.tiles(m, &self.gfx, &self.pack)?;
            }
            "actors" => {
                self.coordinator.actors(m, &self.pack)?;
            }
            "primitives" => {
                self.coordinator.primitives(m, &self.pack)?;
            }
            "environment" => {
                self.coordinator.environment(m)?;
            }
            "display" => {
                self.coordinator.display(m)?;
            }
            "order" => {
                let bundle = self.coordinator.seal(m)?;
                let epoch = bundle.base.base.three.epoch;
                let origin = if epoch.draw_buffer == 0x8003cb08 {
                    [0, 0]
                } else {
                    [256, 0]
                };
                let canvas = bundle.render(m.bus.ram(), &m.bus.gpu, &self.pack, 4)?;
                let picture = Picture::from_canvas(epoch, m.field(), origin, canvas)?;
                if self
                    .last_epoch
                    .is_some_and(|v| v.checked_add(1) != Some(epoch.counter))
                {
                    self.frames = Frames::default();
                    self.owners = [None, None];
                }
                if self.pending.is_some() {
                    return Err("前場尚未提交".into());
                }
                self.pending = Some(picture);
                self.last_epoch = Some(epoch.counter);
                let p =
                    OrderPlan::prepare(m.bus.ram(), m.bus.scratch(), &m.cpu.state, m.bus.biu())?;
                p.commit(m)?;
            }
            "submit" => {
                let epoch = self.coordinator.epoch().ok_or("沒有場景epoch")?;
                self.coordinator.submit(m)?;
                let x = if epoch.draw_buffer == 0x8003cb08 {
                    0
                } else {
                    256
                };
                let token = m
                    .bus
                    .gpu
                    .scene_token(
                        Rect {
                            x,
                            y: 20,
                            width: 256,
                            height: 207,
                        },
                        Rect {
                            x,
                            y: 0,
                            width: 256,
                            height: 240,
                        },
                    )
                    .ok_or("提交後缺少原生所有權")?;
                self.frames
                    .stage(self.pending.take().ok_or("沒有待提交圖像")?)?;
                self.frames.submit(epoch, m.field())?;
                self.owners[x as usize / 256] = Some(token);
                self.submissions += 1;
            }
            _ => unreachable!(),
        }
        Ok(())
    }
    pub fn image(&mut self, m: &Machine) -> Result<Option<super::super::super::worker::Image>> {
        for i in 0..2 {
            if self.owners[i]
                .as_ref()
                .is_none_or(|t| !m.bus.gpu.scene_token_alive(t))
            {
                if self.owners[i].is_some() {
                    self.ownership_drops += 1;
                }
                self.frames.invalidate([(i * 256) as u32, 0])?;
                self.owners[i] = None;
            }
        }
        let picture = self
            .frames
            .present(m.field(), m.bus.gpu.scanout(), self.enabled)?;
        self.last_scene = picture.as_ref().map(|p| p.epoch.counter);
        if let Some(p) = picture {
            self.hd_fields += 1;
            Ok(Some(super::super::super::worker::Image {
                field: m.field(),
                w: p.width,
                h: p.height,
                rgba: Arc::clone(&p.rgba),
                movie: None,
                scene: Some(p.epoch.counter),
            }))
        } else {
            self.native_fields += 1;
            Ok(None)
        }
    }
    pub fn report(&self) -> Value {
        json!({"modeled_phases":self.steps,"submissions":self.submissions,"HD_fields":self.hd_fields,"native_fields":self.native_fields,"errors":self.errors,"mode_events":self.mode_events,"enabled":self.enabled,"phase_failures":self.phase_failures,"ownership_drops":self.ownership_drops,"last_error":self.last_error})
    }
}
