//! Typed native compatibility preparation, 026 §16.17, GPL-3.0-or-later.
//! No GP0 parsing. Cache sampling writes no pixels; native color is on demand.
use super::{
    polygon::{Flags, Vertex},
    transfer::Rect,
    Gpu, NONE,
};
use std::sync::OnceLock;
#[derive(Clone, Copy, Debug, Default)]
pub struct NativeVertex {
    pub x: i16,
    pub y: i16,
    pub u: u8,
    pub v: u8,
    pub color: [u8; 3],
    // Ignored protocol padding remains part of the observable FIFO endpoint.
    pub color_pad: u8,
    pub uv_pad: u16,
}
#[derive(Clone, Debug)]
pub enum NativeCommand {
    Mode {
        page: u16,
    },
    Polygon {
        vertices: Vec<NativeVertex>,
        page: u16,
        clut: u16,
        raw: bool,
        semi: bool,
    },
    Sprite16 {
        x: i16,
        y: i16,
        u: u8,
        v: u8,
        clut: u16,
        color: [u8; 3],
        raw: bool,
        semi: bool,
    },
}
pub struct PreparedScene {
    pub(super) cache: Gpu,
    input: Gpu,
    commands: Vec<NativeCommand>,
    target: Rect,
    native: OnceLock<Gpu>,
}
impl PreparedScene {
    pub fn native_ready(&self) -> bool {
        self.native.get().is_some()
    }
    pub fn command_count(&self) -> usize {
        self.commands.len()
    }
    pub fn target(&self) -> Rect {
        self.target
    }
    /// Explicit native observation only; repeated reads reuse the owned result.
    pub fn native(&self) -> &Gpu {
        self.native.get_or_init(|| {
            let mut g = self.input.clone();
            g.render_native_commands(&self.commands, false);
            g
        })
    }
}
impl Gpu {
    pub fn prepare_native_scene(
        &self,
        commands: Vec<NativeCommand>,
        target: Rect,
    ) -> Result<PreparedScene, String> {
        if self.in_cmd != NONE
            || self.len != 0
            || self.need != 0
            || commands.is_empty()
            || commands.len() > 4096
            || target.width == 0
            || target.height == 0
            || target.x as u32 + target.width as u32 > 1024
            || target.y as u32 + target.height as u32 > 512
            || self.clip_x0 != target.x as i32
            || self.clip_y0 != target.y as i32
            || self.clip_x1 != (target.x as i32 + target.width as i32 - 1)
            || self.clip_y1 != (target.y as i32 + target.height as i32 - 1)
        {
            return Err("原生場景的命令邊界／目標不符".into());
        }
        if self.tww != 0 || self.twh != 0 || self.twx != 0 || self.twy != 0 {
            return Err("原生場景只支援零貼圖視窗".into());
        }
        let mut page = self.page_x / 64 | self.page_y / 16 | (self.tex_mode << 7);
        for c in &commands {
            if matches!(c,NativeCommand::Polygon{vertices,..}if ![3,4].contains(&vertices.len())) {
                return Err("原生多邊形需三或四頂點".into());
            }
            let clut = match c {
                NativeCommand::Mode { page: p } => {
                    page = *p as u32;
                    continue;
                }
                NativeCommand::Polygon { page: p, clut, .. } => {
                    page = *p as u32;
                    *clut
                }
                NativeCommand::Sprite16 { clut, .. } => *clut,
            };
            // Conservatively include the full texture page and CLUT, including wrapping.
            // A framebuffer feedback scene must use an explicitly compatible fallback.
            let tm = page >> 7 & 3;
            let tx = (page & 15) * 64;
            let ty = (page & 16) * 16;
            let overlaps = |x: u32, y: u32, width: u32, height: u32| -> bool {
                let vertical =
                    y < target.y as u32 + target.height as u32 && y + height > target.y as u32;
                let horizontal = (x < target.x as u32 + target.width as u32
                    && (x + width).min(1024) > target.x as u32)
                    || (x + width > 1024 && x + width - 1024 > target.x as u32);
                vertical && horizontal
            };
            if overlaps(tx, ty, 64 << tm.min(2), 256)
                || (tm < 2
                    && overlaps(
                        (clut as u32 & 63) * 16,
                        (clut as u32 >> 6) & 511,
                        if tm == 0 { 16 } else { 256 },
                        1,
                    ))
            {
                return Err("貼圖或色盤可能與繪圖目標重疊".into());
            }
        }
        let mut cache = self.clone();
        cache.render_native_commands(&commands, true);
        Ok(PreparedScene {
            cache,
            input: self.clone(),
            commands,
            target,
            native: OnceLock::new(),
        })
    }
    fn render_native_commands(&mut self, commands: &[NativeCommand], cache_only: bool) {
        for command in commands {
            match command {
                NativeCommand::Mode { page } => {
                    self.set_page(*page as u32);
                    self.sprite_flip = *page as u32 & 0x3000;
                    self.dtd = page & 0x200 != 0;
                    self.dfe = page & 0x400 != 0;
                    self.tww = 0;
                    self.twh = 0;
                    self.twx = 0;
                    self.twy = 0;
                    self.recalc_window();
                    self.buf[0] = 0xe2000000;
                }
                NativeCommand::Polygon {
                    vertices,
                    page,
                    clut,
                    raw,
                    semi,
                } => {
                    self.set_page(*page as u32);
                    self.update_clut(*clut as u32, self.tex_mode);
                    let flags = Flags {
                        gouraud: !cache_only,
                        textured: true,
                        modulate: !raw && !cache_only,
                        mode: if *semi { self.abr as i32 } else { -1 },
                        tm: self.tex_mode,
                        cache_only,
                    };
                    let points: Vec<_> = vertices
                        .iter()
                        .map(|p| Vertex {
                            x: super::sx11(p.x as u16 as u32) + self.off_x,
                            y: super::sx11(p.y as u16 as u32) + self.off_y,
                            u: p.u as i32,
                            v: p.v as i32,
                            r: p.color[0] as i32,
                            g: p.color[1] as i32,
                            b: p.color[2] as i32,
                        })
                        .collect();
                    self.triangle_checked([points[0], points[1], points[2]], flags);
                    if points.len() == 4 {
                        self.triangle_checked([points[1], points[2], points[3]], flags);
                    }
                    let opcode = 0x34
                        | if points.len() == 4 { 8 } else { 0 }
                        | u32::from(*raw)
                        | u32::from(*semi) * 2;
                    for (i, p) in vertices.iter().enumerate() {
                        self.buf[i * 3] = p.color[0] as u32
                            | ((p.color[1] as u32) << 8)
                            | ((p.color[2] as u32) << 16)
                            | ((if i == 0 { opcode } else { p.color_pad as u32 }) << 24);
                        self.buf[i * 3 + 1] = p.x as u16 as u32 | ((p.y as u16 as u32) << 16);
                        let high = if i == 0 {
                            *clut
                        } else if i == 1 {
                            *page
                        } else {
                            p.uv_pad
                        };
                        self.buf[i * 3 + 2] =
                            p.u as u32 | ((p.v as u32) << 8) | ((high as u32) << 16);
                    }
                }
                NativeCommand::Sprite16 {
                    x,
                    y,
                    u,
                    v,
                    clut,
                    color,
                    raw,
                    semi,
                } => {
                    self.update_clut(*clut as u32, self.tex_mode);
                    let rgb =
                        color[0] as u32 | ((color[1] as u32) << 8) | ((color[2] as u32) << 16);
                    let px = super::sx11((super::sx11(*x as u16 as u32) + self.off_x) as u32);
                    let py = super::sx11((super::sx11(*y as u16 as u32) + self.off_y) as u32);
                    self.draw_sprite(
                        px,
                        py,
                        16,
                        16,
                        *u,
                        *v,
                        rgb,
                        true,
                        !raw,
                        if *semi { self.abr as i32 } else { -1 },
                        cache_only,
                    );
                    self.buf[0] = ((0x7c | u32::from(*raw) | u32::from(*semi) * 2) << 24) | rgb;
                    self.buf[1] = *x as u16 as u32 | ((*y as u16 as u32) << 16);
                    self.buf[2] = *u as u32 | ((*v as u32) << 8) | ((*clut as u32) << 16);
                }
            }
        }
    }
}
