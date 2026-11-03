//! Direct ST0 procedural-plane presentation, spec026 §16.9.15.
//! GPL-3.0-or-later. Own planes/primitives/paint contracts, no guest ISA or GP0.
use super::planes;
use crate::texture_audit;
use psgolem_core::gpu::source::VramSource;
use psgolem_core::{
    cpu::State,
    paint::{self, Draw, Point, ProjectedDraw},
    scene::{Canvas, Image, Material, MaterialKey},
};
use sha2::{Digest, Sha256};
use std::{collections::HashMap, path::Path};

type Result<T> = std::result::Result<T, String>;

pub struct View {
    pub materials: Vec<Material>,
    pub images: Vec<Option<Image>>,
    pub native: Vec<Draw>,
    pub projected: Vec<ProjectedDraw>,
    pub hd_quads: usize,
    pub reliable_quads: usize,
}

impl View {
    pub fn prepare(
        ram: &[u8],
        vram: &(impl VramSource + ?Sized),
        entry: &State,
        pack: &Path,
    ) -> Result<Self> {
        if ram.len() != 2 << 20 || vram.word_count() != 1024 * 512 {
            return Err("平面呈現來源大小不符".into());
        }
        if ram[0x6c39c..0x6c39e] != [0, 0] || ram[0x6c3a0..0x6c3a2] != [0, 0] {
            return Err("平面呈現的非零捲動尚未封閉".into());
        }
        let plan = planes::Plan::prepare(ram, entry)?;
        Self::from_quads(ram, vram, plan.quads, pack)
    }
    /// Internal source bundle only; no old pool XY/GP0/framebuffer input.
    pub(super) fn from_quads(
        ram: &[u8],
        vram: &(impl VramSource + ?Sized),
        quads: Vec<planes::Quad>,
        pack: &Path,
    ) -> Result<Self> {
        if ram.len() != 2 << 20 || vram.word_count() != 1024 * 512 || quads.len() > 152 {
            return Err("共同3D畫布來源大小或容量不符".into());
        }
        if ram[0x6c39c..0x6c39e] != [0, 0] || ram[0x6c3a0..0x6c3a2] != [0, 0] {
            return Err("共同3D畫布非零繪圖偏移尚未封閉".into());
        }
        let mut materials = Vec::new();
        let mut ids = HashMap::new();
        let mut draws = Vec::new();
        let mut reliable_quads = 0;
        for q in quads {
            if q.page != 15 || ![0, 2, 0x15].contains(&q.flags) || q.order >= 512 {
                return Err("平面呈現的頁／旗標／排序未封閉".into());
            }
            let min: [usize; 2] =
                std::array::from_fn(|i| q.uv.iter().map(|p| p[i] as usize).min().unwrap());
            let max: [usize; 2] =
                std::array::from_fn(|i| q.uv.iter().map(|p| p[i] as usize).max().unwrap());
            let origin: [usize; 2] = std::array::from_fn(|i| ((min[i] + max[i]) / 2 / 128) * 128);
            let quarter = (0..2).all(|i| {
                max[i] - min[i] <= 128
                    && min[i] as i32 >= origin[i] as i32 - 1
                    && max[i] <= origin[i] + 128
            });
            let (origin, size) = if quarter {
                (origin, 128)
            } else {
                ([0, 0], 256)
            };
            let x = 15 * 64 + origin[0] / 4;
            let y = origin[1];
            if x + size / 4 > 1024 || y + size > 512 || q.clut_index >= 512 {
                return Err("平面材質來源越界".into());
            }
            let at = 0x3c104 + q.clut_index as usize * 2;
            let clut = u16::from_le_bytes([ram[at], ram[at + 1]]) as usize;
            let cx = (clut & 63) * 16;
            let cy = clut >> 6;
            if !(240..512).contains(&cy) {
                return Err("平面色盤可能與原生畫布alias".into());
            }
            let palette: [u16; 16] = std::array::from_fn(|i| vram.source_word(cy * 1024 + cx + i));
            let mut packed = Vec::with_capacity(size * size / 2);
            for row in 0..size {
                for at in (y + row) * 1024 + x..(y + row) * 1024 + x + size / 4 {
                    let w = vram.source_word(at);
                    packed.extend_from_slice(&w.to_le_bytes());
                }
            }
            let mut hash = Sha256::new();
            hash.update((size as u32 / 4).to_le_bytes());
            hash.update((size as u32).to_le_bytes());
            hash.update(&packed);
            let key = MaterialKey {
                upload: hash.finalize().into(),
                clut: Sha256::digest(
                    palette
                        .iter()
                        .flat_map(|v| v.to_le_bytes())
                        .collect::<Vec<_>>(),
                )
                .into(),
            };
            let material = *ids.entry(key).or_insert_with(|| {
                let i = materials.len();
                materials.push(Material {
                    key,
                    width: size,
                    height: size,
                    packed,
                    palette,
                });
                i
            });
            let raw = q.flags & 4 == 0;
            let blend = if q.flags & 1 != 0 { 0 } else { -1 };
            let native = std::array::from_fn(|i| Point {
                x: q.native[i][0] as i32,
                y: q.native[i][1] as i32,
                u: q.uv[i][0] as i32 - origin[0] as i32,
                v: q.uv[i][1] as i32 - origin[1] as i32,
                color: q.points[i].color,
            });
            let points = q.points.map(|mut p| {
                p.uv[0] -= origin[0] as i32;
                p.uv[1] -= origin[1] as i32;
                p
            });
            reliable_quads += usize::from(points.iter().all(|p| p.depth > 0.0));
            draws.push((
                q.order,
                q.handle,
                Draw {
                    material,
                    points: native,
                    raw,
                    blend,
                },
                ProjectedDraw {
                    material,
                    points,
                    raw,
                    blend,
                },
            ));
        }
        draws.sort_by(|a, b| a.0.cmp(&b.0).then(b.1.cmp(&a.1)));
        let images = texture_audit::load_optional_images(
            pack,
            &materials.iter().map(|m| m.key).collect::<Vec<_>>(),
        )?;
        let hd_quads = draws
            .iter()
            .filter(|d| images[d.2.material].is_some())
            .count();
        let (native, projected) = draws.into_iter().map(|(_, _, n, p)| (n, p)).unzip();
        Ok(Self {
            materials,
            images,
            native,
            projected,
            hd_quads,
            reliable_quads,
        })
    }

    pub fn render(&self, scale: usize) -> Result<Canvas> {
        if ![1, 4].contains(&scale) {
            return Err("平面呈現僅支援原尺寸或四倍".into());
        }
        let mut canvas = Canvas {
            width: 256 * scale,
            height: 240 * scale,
            words: vec![0; 256 * 240 * scale * scale],
        };
        self.paint(&mut canvas, scale)?;
        Ok(canvas)
    }
    pub(super) fn paint(&self, canvas: &mut Canvas, scale: usize) -> Result<()> {
        if ![1, 4].contains(&scale) || canvas.width != 256 * scale || canvas.height != 240 * scale {
            return Err("共同3D畫布尺寸不符".into());
        }
        if scale == 1 {
            let images = (0..self.materials.len()).map(|_| None).collect::<Vec<_>>();
            paint::draw(canvas, 1, &self.materials, &images, &self.native)?;
        } else {
            paint::draw_projected(canvas, 4, &self.materials, &self.images, &self.projected)?;
        }
        Ok(())
    }
}
