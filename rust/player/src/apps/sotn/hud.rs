//! Typed SotN screen HUD source, 026 §16.8.1, GPL-3.0-or-later.
//! Own IDA9.4 evidence: DRA sub_800EDC94, native record pool 80086FEC.
//! Reads game records and native texels; never reads generated GP0 packets.
use crate::texture_audit;
use psgolem_core::{
    paint::{self, Draw, Point},
    scene::{Canvas, Image, ImagePixels, Material, MaterialKey},
};
use sha2::{Digest, Sha256};
use std::path::Path;
type Result<T> = std::result::Result<T, String>;
fn half(b: &[u8], p: usize) -> u16 {
    u16::from_le_bytes([b[p], b[p + 1]])
}
fn word(b: &[u8], p: usize) -> u32 {
    u32::from_le_bytes(b[p..p + 4].try_into().unwrap())
}
pub struct Hud {
    pub materials: Vec<Material>,
    pub images: Vec<Option<Image>>,
    pub draws: Vec<Draw>,
    pub records: Vec<usize>,
    pub cluts: Vec<u16>,
    pub pages: Vec<u16>,
    pub clear_texels: usize,
    pub deferred_records: usize,
}
impl Hud {
    pub fn source(archive: &[u8]) -> Result<&[u8]> {
        if archive.len() != 270336
            || format!("{:x}", Sha256::digest(archive))
                != "6f8c832dd1986aca1ed04247d3bd7977aae4ddf5cb3ac569097f6c0b1123ac74"
        {
            return Err("HUD原始archive身份不符".into());
        }
        Ok(&archive[229376..229376 + 8192])
    }
    pub fn prepare(r: &[u8], v: &[u16], source: &[u8], pack: &Path) -> Result<Self> {
        if r.len() != 2 << 20
            || v.len() != 1024 * 512
            || source.len() != 8192
            || word(r, 0x3c0f8) != 0
            || word(r, 0x973ec) != 0
        {
            return Err("HUD來源態不在支援範圍".into());
        }
        let mut h = Sha256::new();
        h.update(32u32.to_le_bytes());
        h.update(128u32.to_le_bytes());
        h.update(source);
        let canonical: [u8; 32] = h.finalize().into();
        if format!("{:x}", Sha256::digest(source))
            != "7e26b0ee9a9708bb6e747147ea03ceaf04a0dcb94b05770e83c18bcfbb40fb47"
        {
            return Err("HUD原始貼圖身份不符".into());
        }
        let mut loaded = texture_audit::load_optional_images(
            pack,
            &[MaterialKey {
                upload: canonical,
                clut: [0; 32],
            }],
        )?;
        let mut base = loaded.pop().flatten().ok_or("缺少已定案HUD索引圖像")?;
        let pixels = match &mut base.pixels {
            ImagePixels::Indexed(p) => p,
            _ => return Err("HUD動態遮罩需要索引圖像".into()),
        };
        if ![2, 4, 8].contains(&base.scale)
            || base.width != 128 * base.scale
            || base.height != 128 * base.scale
            || pixels.len() != base.width * base.height
        {
            return Err("HUD圖像形狀不符".into());
        }
        let mut packed = Vec::with_capacity(8192);
        for y in 256..384 {
            for &w in &v[y * 1024 + 960..y * 1024 + 992] {
                packed.extend_from_slice(&w.to_le_bytes());
            }
        }
        let mut clear_texels = 0;
        for y in 0..128 {
            for x in 0..128 {
                let shift = (x % 2) * 4;
                let old = (source[y * 64 + x / 2] >> shift) & 15;
                let now = (packed[y * 64 + x / 2] >> shift) & 15;
                if now != old {
                    if now != 0 {
                        return Err("HUD貼圖有未支援的非零改寫".into());
                    }
                    clear_texels += 1;
                    for yy in y * base.scale..(y + 1) * base.scale {
                        for xx in x * base.scale..(x + 1) * base.scale {
                            pixels[yy * 128 * base.scale + xx] = 0;
                        }
                    }
                }
            }
        }
        let mut result = Self {
            materials: Vec::new(),
            images: Vec::new(),
            draws: Vec::new(),
            records: Vec::new(),
            cluts: Vec::new(),
            pages: Vec::new(),
            clear_texels,
            deferred_records: 0,
        };
        let mut selected = Vec::new();
        for i in 0..1280 {
            let p = 0x86fec + i * 52;
            let kind = r[p + 7];
            let flags = half(r, p + 50);
            if kind == 0 || flags & 8 != 0 {
                continue;
            }
            if kind != 4 || half(r, p + 38) != 496 || flags & 0x2000 == 0 {
                result.deferred_records += 1;
                continue;
            }
            if flags & !0x2005 != 0
                || half(r, p + 26) != 31
                || !(-512..=512).contains(&(half(r, p + 8) as i16 as i32))
                || !(-512..=512).contains(&(half(r, p + 10) as i16 as i32))
            {
                return Err("HUD圖元模式或頁面不支援".into());
            }
            let ci = half(r, p + 14) as usize;
            if ci >= 512 {
                return Err("HUD色盤索引超界".into());
            }
            let clut = half(r, 0x3c104 + ci * 2);
            let cx = (clut as usize & 63) * 16;
            let cy = clut as usize >> 6;
            if cy < 240 || cy >= 256 {
                return Err("HUD色盤區域不支援".into());
            }
            let palette: [u16; 16] = v[cy * 1024 + cx..cy * 1024 + cx + 16].try_into().unwrap();
            if palette[0] != 0 {
                return Err("HUD清除後色盤零號必須透明".into());
            }
            let mut points = [Point {
                x: 0,
                y: 0,
                u: 0,
                v: 0,
                color: [0; 3],
            }; 4];
            for (j, point) in points.iter_mut().enumerate() {
                let a = p + j * 12;
                let u = r[a + 12] as i32;
                let vv = r[a + 13] as i32;
                if u > 128 || vv > 128 {
                    return Err("HUD貼圖座標超界".into());
                }
                *point = Point {
                    x: half(r, a + 8) as i16 as i32,
                    y: half(r, a + 10) as i16 as i32,
                    u,
                    v: vv,
                    color: [r[a + 4], r[a + 5], r[a + 6]],
                };
            }
            if points.iter().any(|p| p.x.abs() > 512 || p.y.abs() > 512) {
                return Err("HUD座標超界".into());
            }
            selected.push((i, flags, clut, palette, points));
        }
        if selected.is_empty() || selected.len() > 32 {
            return Err("HUD數量不在支援範圍".into());
        }
        // Same OT bucket prepends each source in ascending pool order.
        selected.reverse();
        for (i, flags, clut, palette, points) in selected {
            let index = result.materials.len();
            let mut h = Sha256::new();
            h.update(32u32.to_le_bytes());
            h.update(128u32.to_le_bytes());
            h.update(&packed);
            let bytes: Vec<_> = palette.iter().flat_map(|w| w.to_le_bytes()).collect();
            result.materials.push(Material {
                key: MaterialKey {
                    upload: h.finalize().into(),
                    clut: Sha256::digest(bytes).into(),
                },
                width: 128,
                height: 128,
                packed: packed.clone(),
                palette,
            });
            result.images.push(Some(base.clone()));
            result.records.push(i);
            result.cluts.push(clut);
            result.pages.push(31);
            result.draws.push(Draw {
                material: index,
                points,
                raw: flags & 4 == 0,
                blend: if flags & 1 != 0 { 0 } else { -1 },
            });
        }
        Ok(result)
    }
    pub fn paint(&self, c: &mut Canvas, scale: usize) -> Result<()> {
        paint::draw(c, scale, &self.materials, &self.images, &self.draws)
    }
}
