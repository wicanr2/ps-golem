//! Actor-only material and draw preparation, 026 §16.12, GPL-3.0-or-later.
//! Extracted without changing the verified NO3 material/source contract.
use super::actor::ActorPlan;
use crate::texture_audit;
use psgolem_core::{
    paint::{self, Draw, Point},
    scene::{Canvas, Image, Material, MaterialKey},
};
use sha2::{Digest, Sha256};
use std::{collections::HashMap, path::Path};
type Result<T> = std::result::Result<T, String>;
fn span(r: &[u8], a: u32, n: usize) -> Result<usize> {
    let p = (a & 0x1fffffff) as usize;
    if p >= r.len() || n > r.len() - p {
        Err("HD來源越界".into())
    } else {
        Ok(p)
    }
}
fn half(r: &[u8], a: u32) -> Result<u16> {
    let p = span(r, a, 2)?;
    Ok(u16::from_le_bytes(r[p..p + 2].try_into().unwrap()))
}
fn word(r: &[u8], a: u32) -> Result<u32> {
    let p = span(r, a, 4)?;
    Ok(u32::from_le_bytes(r[p..p + 4].try_into().unwrap()))
}
pub struct ActorView {
    pub materials: Vec<Material>,
    pub draws: Vec<Draw>,
    pub images: Vec<Option<Image>>,
    pub hd_actor_quads: usize,
    pub native_actor_quads: usize,
    pub actor_entities: Vec<usize>,
    pub orders: Vec<usize>,
}
impl ActorView {
    pub fn prepare_at(
        r: &[u8],
        vram: &[u16],
        actors: &ActorPlan,
        pack: &Path,
        left: i32,
        top: i32,
    ) -> Result<Self> {
        if r.len() != 2 << 20 || vram.len() != 1024 * 512 {
            return Err("人物畫布來源大小不符".into());
        }
        if actors
            .quads
            .iter()
            .any(|q| q.order < 104 || q.page & 0x780 != 0 || q.page & 16 == 0)
        {
            return Err("HD人物排序、depth／dither或來源頁不在已驗範圍".into());
        }
        let mut materials = Vec::new();
        let mut ids = HashMap::new();
        let mut staged = Vec::new();
        for (ordinal, q) in actors.quads.iter().enumerate() {
            let a = 0x800733d8 + q.entity as u32 * 188;
            let bank = half(r, a + 0x54)?;
            let frame = half(r, a + 0x56)?;
            let table = if bank & 0x8000 == 0 {
                word(r, 0x800a3b80 + bank as u32 * 4)?
            } else {
                word(r, word(r, 0x8003c788)? + (bank as u32 & 0x7fff) * 4)?
            };
            let fd = word(r, table + (frame as u32 & 0x7fff) * 4)?;
            let n = half(r, fd)?;
            let mut palette = [0; 16];
            let cx = (q.clut as usize & 63) * 16;
            let cy = q.clut as usize >> 6;
            if cy >= 512 || cy < 240 {
                return Err("HD人物CLUT可能與畫布alias".into());
            }
            palette.copy_from_slice(&vram[cy * 1024 + cx..cy * 1024 + cx + 16]);
            let (width, height, packed, origin) = if n & 0x8000 != 0 {
                let pool = match bank {
                    13 => word(r, 0x8013c004)?,
                    0x8001 => word(r, 0x8003c7a0)?,
                    b if b & 0x8000 != 0 && b != 0x8010 => word(r, 0x8003c7a4)?,
                    _ => 0x8013c020,
                };
                let source = word(r, pool + (n as u32 & 0x7fff) * 4)?;
                let at = span(r, source, 4)?;
                let width = r[at] as usize;
                let height = r[at + 1] as usize;
                if width == 0 || width % 4 != 0 || height == 0 {
                    return Err("HD packed形狀不符".into());
                }
                let start = span(r, source + 4, width * height / 2)?;
                let packed = r[start..start + width * height / 2].to_vec();
                let cache = 0x80097d1c + half(r, a + 0x5a)? as u32 * 40;
                let x = half(r, cache)? as usize;
                let y = half(r, cache + 2)? as usize;
                if x + width / 4 > 1024 || y + height > 512 {
                    return Err("HD packed顯存來源越界".into());
                }
                if (q.page as usize & 15) * 64 != x & !63
                    || (q.page as usize >> 4 & 1) * 256 != y & !255
                {
                    return Err("HD packed頁與cache來源不同".into());
                }
                for yy in 0..height {
                    for xx in 0..width / 4 {
                        let i = (yy * width / 4 + xx) * 2;
                        let v = u16::from_le_bytes(packed[i..i + 2].try_into().unwrap());
                        if vram[(y + yy) * 1024 + x + xx] != v {
                            return Err("HD packed來源與顯存不同".into());
                        }
                    }
                }
                (
                    width,
                    height,
                    packed,
                    [((x & 63) * 4) as i32, (y & 255) as i32],
                )
            } else {
                let x = (q.page as usize & 15) * 64;
                let y = (q.page as usize >> 4 & 1) * 256;
                if x + 64 > 1024 {
                    return Err("HD普通頁越界".into());
                }
                // Original fixed atlases are uploaded as 128x128 quadrants. Exact byte
                // identity and CLUT retain the existing material key, never a visual guess.
                let u0 = q.vertices.iter().map(|p| p.u as usize).min().unwrap();
                let u1 = q.vertices.iter().map(|p| p.u as usize).max().unwrap();
                let v0 = q.vertices.iter().map(|p| p.v as usize).min().unwrap();
                let v1 = q.vertices.iter().map(|p| p.v as usize).max().unwrap();
                let ox = ((u0 + u1) / 2 / 128) * 128;
                let oy = ((v0 + v1) / 2 / 128) * 128;
                let quarter = u1 - u0 <= 128
                    && v1 - v0 <= 128
                    && u0 as i32 >= ox as i32 - 1
                    && u1 <= ox + 128
                    && v0 as i32 >= oy as i32 - 1
                    && v1 <= oy + 128;
                let (ox, oy, size) = if quarter { (ox, oy, 128) } else { (0, 0, 256) };
                let mut packed = Vec::with_capacity(size * size / 2);
                for yy in 0..size {
                    for &w in &vram[(y + oy + yy) * 1024 + x + ox / 4
                        ..(y + oy + yy) * 1024 + x + ox / 4 + size / 4]
                    {
                        packed.extend_from_slice(&w.to_le_bytes());
                    }
                }
                (size, size, packed, [ox as i32, oy as i32])
            };
            let mut h = Sha256::new();
            h.update((width as u32 / 4).to_le_bytes());
            h.update((height as u32).to_le_bytes());
            h.update(&packed);
            let colors: Vec<_> = palette.iter().flat_map(|w| w.to_le_bytes()).collect();
            let key = MaterialKey {
                upload: h.finalize().into(),
                clut: Sha256::digest(&colors).into(),
            };
            let index = if let Some(&i) = ids.get(&key) {
                i
            } else {
                let i = materials.len();
                ids.insert(key, i);
                materials.push(Material {
                    key,
                    width,
                    height,
                    packed,
                    palette,
                });
                i
            };
            let points = q.vertices.map(|v| Point {
                x: v.x as i32 - left,
                y: v.y as i32 - top,
                u: v.u as i32 - origin[0],
                v: v.v as i32 - origin[1],
                color: v.color,
            });
            staged.push((
                q.order,
                ordinal,
                q.entity,
                Draw {
                    material: index,
                    points,
                    raw: q.command & 1 != 0,
                    blend: if q.command & 2 != 0 {
                        ((q.page >> 5) & 3) as i32
                    } else {
                        -1
                    },
                },
            ));
        }
        staged.sort_by(|a, b| a.0.cmp(&b.0).then(b.1.cmp(&a.1)));
        let images = texture_audit::load_optional_images(
            pack,
            &materials.iter().map(|m| m.key).collect::<Vec<_>>(),
        )?;
        let hd_actor_quads = staged
            .iter()
            .filter(|(_, _, _, d)| images[d.material].is_some())
            .count();
        let native_actor_quads = staged.len() - hd_actor_quads;
        let mut actor_entities = Vec::new();
        let mut orders = Vec::new();
        let draws = staged
            .into_iter()
            .map(|(order, _, e, d)| {
                orders.push(order);
                actor_entities.push(e);
                d
            })
            .collect();
        Ok(Self {
            materials,
            draws,
            images,
            hd_actor_quads,
            native_actor_quads,
            actor_entities,
            orders,
        })
    }
    pub fn paint(&self, canvas: &mut Canvas, scale: usize) -> Result<()> {
        paint::draw(canvas, scale, &self.materials, &self.images, &self.draws)
    }
}
