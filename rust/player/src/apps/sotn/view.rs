//! Direct source-to-HD scene+actor connection, spec026 §16.7.20.
//! GPL-3.0-or-later. No GP0 packets, native framebuffer or CPU execution.
use super::{actor::ActorPlan, scene};
use crate::texture_audit;
use psgolem_core::{
    paint::{self, Draw, Point},
    scene::{Canvas, Image, Material, MaterialKey, Scene},
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
pub struct View {
    pub background: Scene,
    pub materials: Vec<Material>,
    pub draws: Vec<Draw>,
    pub images: Vec<Option<Image>>,
    pub background_images: Vec<Image>,
    pub hd_actor_quads: usize,
    pub native_actor_quads: usize,
    pub actor_entities: Vec<usize>,
}
impl View {
    pub fn prepare(
        r: &[u8],
        vram: &[u16],
        graphics: &[u8],
        actors: &ActorPlan,
        pack: &Path,
    ) -> Result<Self> {
        let background = scene::build(r, vram, graphics, 0, 256)?;
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
                x: v.x as i32,
                y: v.y as i32,
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
        let background_images = texture_audit::load_scene_images(
            pack,
            &background
                .materials
                .iter()
                .map(|m| m.key)
                .collect::<Vec<_>>(),
        )?;
        let hd_actor_quads = staged
            .iter()
            .filter(|(_, _, _, d)| images[d.material].is_some())
            .count();
        let native_actor_quads = staged.len() - hd_actor_quads;
        let mut actor_entities = Vec::new();
        let draws = staged
            .into_iter()
            .map(|(_, _, e, d)| {
                actor_entities.push(e);
                d
            })
            .collect();
        Ok(Self {
            background,
            materials,
            draws,
            images,
            background_images,
            hd_actor_quads,
            native_actor_quads,
            actor_entities,
        })
    }
    pub fn render(&self, scale: usize) -> Result<Canvas> {
        let mut canvas = self.background.render(
            scale,
            if scale == 4 {
                Some(&self.background_images)
            } else {
                None
            },
        )?;
        paint::draw(
            &mut canvas,
            scale,
            &self.materials,
            &self.images,
            &self.draws,
        )?;
        Ok(canvas)
    }
}
// Read-only saved-source diagnostic, 026 §16.7.20. Never initializes a Machine.
pub fn render_file(path: &std::path::Path) -> Result<(Canvas, Canvas, serde_json::Value)> {
    use serde_json::{json, Value};
    use std::{fs, io::Read, path::PathBuf};
    let mut b = Vec::new();
    fs::File::open(path)
        .map_err(|e| e.to_string())?
        .take(16385)
        .read_to_end(&mut b)
        .map_err(|e| e.to_string())?;
    if b.len() > 16384 {
        return Err("預覽描述超過16KiB".into());
    }
    let v: Value = serde_json::from_slice(&b).map_err(|e| e.to_string())?;
    let obj = v.as_object().ok_or("預覽描述須為物件")?;
    let allowed = [
        "version", "ram", "vram", "scratch", "graphics", "textures", "output", "sha256",
    ];
    if obj.len() != allowed.len()
        || obj.keys().any(|k| !allowed.contains(&k.as_str()))
        || v["version"] != 1
    {
        return Err("預覽描述欄位或版本不符".into());
    }
    let root = path.parent().unwrap_or(std::path::Path::new("."));
    let get = |key: &str| -> Result<PathBuf> {
        let p = PathBuf::from(v[key].as_str().ok_or("預覽路徑須為字串")?);
        Ok(if p.is_absolute() { p } else { root.join(p) })
    };
    let hashes = v["sha256"].as_object().ok_or("預覽來源需要sha256")?;
    if hashes.len() != 5
        || hashes
            .keys()
            .any(|k| !["ram", "vram", "scratch", "graphics", "manifest"].contains(&k.as_str()))
    {
        return Err("預覽來源SHA欄位不符".into());
    }
    let mut loaded = HashMap::new();
    for (key, len) in [
        ("ram", 2 << 20),
        ("vram", 1024 * 512 * 2),
        ("scratch", 1024),
        ("graphics", 0x40000),
    ] {
        let mut bytes = Vec::new();
        fs::File::open(get(key)?)
            .map_err(|e| e.to_string())?
            .take(len as u64 + 1)
            .read_to_end(&mut bytes)
            .map_err(|e| e.to_string())?;
        let digest = format!("{:x}", Sha256::digest(&bytes));
        if bytes.len() != len || hashes[key].as_str() != Some(digest.as_str()) {
            return Err(format!("預覽來源大小或SHA不符：{key}"));
        }
        loaded.insert(key, bytes);
    }
    let pack = get("textures")?;
    let manifest = pack.join("manifest.json");
    let metadata = fs::metadata(&manifest).map_err(|e| e.to_string())?;
    if metadata.len() > 4 << 20 {
        return Err("預覽manifest過大".into());
    }
    let digest = format!(
        "{:x}",
        Sha256::digest(fs::read(&manifest).map_err(|e| e.to_string())?)
    );
    if hashes["manifest"].as_str() != Some(digest.as_str()) {
        return Err("預覽manifest SHA不符".into());
    }
    let vram: Vec<_> = loaded["vram"]
        .chunks_exact(2)
        .map(|b| u16::from_le_bytes([b[0], b[1]]))
        .collect();
    let actor = ActorPlan::prepare(&loaded["ram"], &loaded["scratch"])?;
    let view = View::prepare(&loaded["ram"], &vram, &loaded["graphics"], &actor, &pack)?;
    let native = view.render(1)?;
    let hd = view.render(4)?;
    let prefix = get("output")?;
    if !prefix.parent().is_some_and(|p| p.is_dir()) {
        return Err("預覽輸出目錄不存在".into());
    }
    for (suffix, c) in [("native.png", &native), ("HD.png", &hd)] {
        let mut rgba = c.rgba();
        for p in rgba.chunks_exact_mut(4) {
            p[3] = 255;
        }
        let mut e = png::Encoder::new(
            fs::File::create(prefix.with_extension(suffix)).map_err(|e| e.to_string())?,
            c.width as u32,
            c.height as u32,
        );
        e.set_color(png::ColorType::Rgba);
        e.set_depth(png::BitDepth::Eight);
        e.write_header()
            .map_err(|e| e.to_string())?
            .write_image_data(&rgba)
            .map_err(|e| e.to_string())?;
    }
    let report = json!({"scope":"saved-source direct HD preview; not gameplay","native_width":native.width,"native_height":native.height,"HD_width":hd.width,"HD_height":hd.height,"background_HD_materials":view.background_images.len(),"typed_actor_quads":view.draws.len(),"HD_actor_quads":view.hd_actor_quads,"native_actor_fallback_quads":view.native_actor_quads,"original_CPU_executed":false,"GP0_submitted":false,"native_framebuffer_input":false,"HUD_text_effects_complete":false,"full_HD_complete":false,"speed_pass":false});
    fs::write(
        prefix.with_extension("json"),
        serde_json::to_vec_pretty(&report).map_err(|e| e.to_string())?,
    )
    .map_err(|e| e.to_string())?;
    Ok((native, hd, report))
}
