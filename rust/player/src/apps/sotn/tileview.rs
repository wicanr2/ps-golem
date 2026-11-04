//! ST0 typed tiles -> one shared material set, 026 §16.13, GPL-3.0-or-later.
//! Original archive/CLUT identity; no generated packets or framebuffer input.
use super::call::Tile;
use crate::texture_audit;
use psgolem_core::gpu::source::VramSource;
use psgolem_core::{
    paint::{self, Draw, Point},
    scene::{Canvas, Image, Material, MaterialKey},
};
use sha2::{Digest, Sha256};
use std::{
    collections::{BTreeSet, HashMap},
    path::Path,
};
type Result<T> = std::result::Result<T, String>;
fn half(r: &[u8], p: usize) -> u16 {
    u16::from_le_bytes([r[p], r[p + 1]])
}
pub struct View {
    pub materials: Vec<Material>,
    pub images: Vec<Option<Image>>,
    pub background: Vec<Draw>,
    pub foreground: Vec<Draw>,
    pub tiles: Vec<Tile>,
    pub HD_hits: usize,
}
impl View {
    pub fn prepare(
        r: &[u8],
        v: &(impl VramSource + ?Sized),
        gfx: &[u8],
        tiles: &[Tile],
        pack: &Path,
    ) -> Result<Self> {
        Self::prepare_profile(
            r,
            v,
            gfx,
            tiles,
            pack,
            "63340ba46ee070511824bafa5d10c7465d58618323a44965e543c73495ab3c63",
            [160, 96],
        )
    }
    pub fn prepare_no3(
        r: &[u8],
        v: &(impl VramSource + ?Sized),
        gfx: &[u8],
        tiles: &[Tile],
        pack: &Path,
    ) -> Result<Self> {
        Self::prepare_profile(
            r,
            v,
            gfx,
            tiles,
            pack,
            "a0bb78a1260eee5e3bda4bae6cfbbd0a61ddd5604905f9fd6ef833fd03319c66",
            [96, 32],
        )
    }
    fn prepare_profile(
        r: &[u8],
        v: &(impl VramSource + ?Sized),
        gfx: &[u8],
        tiles: &[Tile],
        pack: &Path,
        source_sha: &str,
        orders: [usize; 2],
    ) -> Result<Self> {
        if r.len() != 2 << 20
            || v.word_count() != 1024 * 512
            || tiles.len() > 640
            || format!("{:x}", Sha256::digest(gfx)) != source_sha
        {
            return Err("ST0圖塊來源或數量不符".into());
        }
        let mut out = Self {
            materials: Vec::new(),
            images: Vec::new(),
            background: Vec::new(),
            foreground: Vec::new(),
            tiles: tiles.to_vec(),
            HD_hits: 0,
        };
        let mut ids = HashMap::new();
        let mut positions = BTreeSet::new();
        for t in tiles {
            if t.page > 7
                || t.order != (if t.foreground { orders[0] } else { orders[1] }) + t.page as usize
                || (t.x as i32).abs() > 512
                || (t.y as i32).abs() > 512
                || !positions.insert((t.foreground, t.x, t.y))
            {
                return Err("ST0圖塊頁、排序或位置未封閉".into());
            }
            let u = (t.gfx as usize & 15) * 16;
            let vv = (t.gfx as usize >> 4) * 16;
            let q = vv / 128 * 2 + u / 128;
            let raw = &gfx[t.page as usize * 0x8000 + q * 0x2000
                ..t.page as usize * 0x8000 + (q + 1) * 0x2000];
            let pal = t.palette as usize;
            let bank = pal % 16;
            let at = bank / 4 * 0x8000
                + bank % 2 * 32
                + pal / 16 * 64
                + if bank % 4 < 2 { 0x5c00 } else { 0x7c00 };
            let palette: [u16; 16] = std::array::from_fn(|i| half(gfx, at + i * 2));
            let clut = half(r, 0x3c104 + pal * 2) as usize;
            let cx = (clut & 63) * 16;
            let cy = clut >> 6;
            if !(240..256).contains(&cy)
                || (0..16).any(|i| v.source_word(cy * 1024 + cx + i) != palette[i])
            {
                return Err("ST0色盤不符或可能與畫布alias".into());
            }
            let x = (t.page as usize + 8) * 64 + u / 4;
            for yy in 0..16 {
                for xx in 0..4 {
                    if v.source_word((vv + yy) * 1024 + x + xx)
                        != half(raw, (vv % 128 + yy) * 64 + (u % 128) / 2 + xx * 2)
                    {
                        return Err("ST0原圖塊上傳與archive不同".into());
                    }
                }
            }
            let mut h = Sha256::new();
            h.update(32u32.to_le_bytes());
            h.update(128u32.to_le_bytes());
            h.update(raw);
            let key = MaterialKey {
                upload: h.finalize().into(),
                clut: Sha256::digest(&gfx[at..at + 32]).into(),
            };
            let material = *ids.entry(key).or_insert_with(|| {
                let i = out.materials.len();
                out.materials.push(Material {
                    key,
                    width: 128,
                    height: 128,
                    packed: raw.to_vec(),
                    palette,
                });
                i
            });
            let points = std::array::from_fn(|i| Point {
                x: t.x as i32 + if i & 1 != 0 { 16 } else { 0 },
                y: t.y as i32 + if i & 2 != 0 { 16 } else { 0 },
                u: (u % 128) as i32 + if i & 1 != 0 { 16 } else { 0 },
                v: (vv % 128) as i32 + if i & 2 != 0 { 16 } else { 0 },
                color: [128; 3],
            });
            let draw = Draw {
                material,
                points,
                raw: true,
                blend: -1,
            };
            if t.foreground {
                out.foreground.push(draw);
            } else {
                out.background.push(draw);
            }
        }
        out.images = texture_audit::load_optional_images(
            pack,
            &out.materials.iter().map(|m| m.key).collect::<Vec<_>>(),
        )?;
        out.HD_hits = out
            .background
            .iter()
            .chain(out.foreground.iter())
            .filter(|d| out.images[d.material].is_some())
            .count();
        Ok(out)
    }
    pub fn paint_background(&self, c: &mut Canvas, scale: usize) -> Result<()> {
        paint::draw(c, scale, &self.materials, &self.images, &self.background)
    }
    pub fn paint_foreground(&self, c: &mut Canvas, scale: usize) -> Result<()> {
        paint::draw(c, scale, &self.materials, &self.images, &self.foreground)
    }
}
