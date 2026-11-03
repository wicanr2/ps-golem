//! ST0 screen HUD input, 026 §16.11, GPL-3.0-or-later.
//! Semantic pool records + original upload; no generated packets/framebuffer.
use crate::texture_audit;
use psgolem_core::gpu::source::VramSource;
use psgolem_core::{
    paint::{self, Draw, Point},
    scene::{Canvas, Image, Material, MaterialKey},
};
use sha2::{Digest, Sha256};
use std::path::Path;
type Result<T> = std::result::Result<T, String>;
pub fn half(r: &[u8], p: usize) -> u16 {
    u16::from_le_bytes(r[p..p + 2].try_into().unwrap())
}
fn word(r: &[u8], p: usize) -> u32 {
    u32::from_le_bytes(r[p..p + 4].try_into().unwrap())
}
pub struct Hud {
    pub materials: Vec<Material>,
    pub images: Vec<Option<Image>>,
    pub draws: Vec<Draw>,
    pub records: Vec<usize>,
    pub orders: Vec<u16>,
    pub cluts: Vec<u16>,
    pub HD_hits: usize,
}
impl Hud {
    pub fn prepare(r: &[u8], v: &(impl VramSource + ?Sized), pack: &Path) -> Result<Self> {
        if r.len() != 2 << 20
            || v.word_count() != 1024 * 512
            || word(r, 0x3c0f8) != 0
            || word(r, 0x973ec) != 0
            || format!("{:x}", Sha256::digest(&r[0x1ba70c..0x1baeb0]))
                != "de1e58a6352e4714bb50592730d2f9a83c534d5d51a512795750853664eb9fda"
        {
            return Err("序章HUD來源態不符".into());
        }
        let mut packed = Vec::with_capacity(8192);
        for y in 256..384 {
            for at in y * 1024 + 704..y * 1024 + 736 {
                let w = v.source_word(at);
                packed.extend_from_slice(&w.to_le_bytes());
            }
        }
        let mut h = Sha256::new();
        h.update(32u32.to_le_bytes());
        h.update(128u32.to_le_bytes());
        h.update(&packed);
        let upload: [u8; 32] = h.finalize().into();
        if format!(
            "{:x}",
            Sha256::digest(
                [
                    32u32.to_le_bytes().as_slice(),
                    128u32.to_le_bytes().as_slice(),
                    &packed
                ]
                .concat()
            )
        ) != "6326873f58316d3b8076e611389a1e43428491b370b8320d6734c08af4708883"
        {
            return Err("序章HUD原上傳被未知改寫".into());
        }
        let mut selected = Vec::new();
        for i in 0..1280 {
            let p = 0x86fec + i * 52;
            let flags = half(r, p + 50);
            let order = half(r, p + 38);
            if r[p + 7] == 0
                || flags & 8 != 0
                || r[p + 7] != 4
                || half(r, p + 26) != 27
                || ![495, 496].contains(&order)
            {
                continue;
            }
            if flags != 0x2000 {
                return Err("序章HUD旗標未封閉".into());
            }
            let ci = half(r, p + 14) as usize;
            if ci >= 512 {
                return Err("序章HUD色盤索引越界".into());
            }
            let clut = half(r, 0x3c104 + ci * 2) as usize;
            let cx = (clut & 63) * 16;
            let cy = clut >> 6;
            if !(240..256).contains(&cy) {
                return Err("序章HUD色盤可能與畫布alias".into());
            }
            let palette: [u16; 16] = std::array::from_fn(|i| v.source_word(cy * 1024 + cx + i));
            let points = std::array::from_fn(|j| {
                let a = p + j * 12;
                Point {
                    x: half(r, a + 8) as i16 as i32,
                    y: half(r, a + 10) as i16 as i32,
                    u: r[a + 12] as i32,
                    v: r[a + 13] as i32,
                    color: r[a + 4..a + 7].try_into().unwrap(),
                }
            });
            if points
                .iter()
                .any(|p: &Point| p.x.abs() > 512 || p.y.abs() > 512 || p.u > 128 || p.v > 128)
            {
                return Err("序章HUD座標或UV越界".into());
            }
            selected.push((order, i, clut as u16, palette, points));
        }
        if selected.is_empty() || selected.len() > 32 {
            return Err("序章HUD數量未封閉".into());
        }
        selected.sort_by(|a, b| a.0.cmp(&b.0).then(b.1.cmp(&a.1)));
        let mut out = Self {
            materials: Vec::new(),
            images: Vec::new(),
            draws: Vec::new(),
            records: Vec::new(),
            orders: Vec::new(),
            cluts: Vec::new(),
            HD_hits: 0,
        };
        for (order, id, clut, palette, points) in selected {
            let i = out.materials.len();
            let bytes: Vec<_> = palette.iter().flat_map(|w| w.to_le_bytes()).collect();
            out.materials.push(Material {
                key: MaterialKey {
                    upload,
                    clut: Sha256::digest(bytes).into(),
                },
                width: 128,
                height: 128,
                packed: packed.clone(),
                palette,
            });
            out.records.push(id);
            out.orders.push(order);
            out.cluts.push(clut);
            out.draws.push(Draw {
                material: i,
                points,
                raw: true,
                blend: -1,
            });
        }
        out.images = texture_audit::load_optional_images(
            pack,
            &out.materials.iter().map(|m| m.key).collect::<Vec<_>>(),
        )?;
        out.HD_hits = out.images.iter().filter(|i| i.is_some()).count();
        Ok(out)
    }
    pub fn paint(&self, c: &mut Canvas, scale: usize) -> Result<()> {
        paint::draw(c, scale, &self.materials, &self.images, &self.draws)
    }
}
