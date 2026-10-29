//! 024 section6: validated external artwork, safe geometry and outside blending.
//! GPL-3.0-or-later. Port of own Go poster.go/layout.go at d3e321d.
use crate::{
    preferences::{valid_id, Choice},
    worker::Image,
};
use sdl2::{
    pixels::PixelFormatEnum,
    rect::{FRect, Rect},
    render::{BlendMode, Canvas, ScaleMode, Texture, TextureCreator},
    video::{Window, WindowContext},
};
use serde_json::Value;
use std::{
    collections::BTreeSet,
    fs,
    io::{Cursor, Read},
    path::Path,
    sync::Arc,
};
struct Picture {
    w: u32,
    h: u32,
    rgba: Vec<u8>,
}
struct Entry {
    id: String,
    name: String,
    images: [Picture; 2],
}
pub struct Pack {
    entries: Vec<Entry>,
}
fn read(root: &Path, name: &str, limit: usize) -> Result<Vec<u8>, String> {
    if name.is_empty() || name == "." || name == ".." || name.contains('/') || name.contains('\\') {
        return Err("無效海報檔名".into());
    }
    let path = root.join(name);
    let meta = fs::symlink_metadata(&path).map_err(|e| e.to_string())?;
    if !meta.is_file() || meta.len() > limit as u64 {
        return Err("海報檔案超限或不是一般檔案".into());
    }
    let mut bytes = Vec::new();
    fs::File::open(path)
        .map_err(|e| e.to_string())?
        .take(limit as u64 + 1)
        .read_to_end(&mut bytes)
        .map_err(|e| e.to_string())?;
    if bytes.len() > limit {
        return Err("海報檔案超限".into());
    }
    Ok(bytes)
}
fn keys(v: &Value, allowed: &[&str]) -> Result<(), String> {
    let m = v.as_object().ok_or("海報清單項目須為物件")?;
    if m.keys().any(|k| !allowed.contains(&k.as_str())) {
        return Err("海報清單含未知欄位".into());
    }
    Ok(())
}
impl Pack {
    pub fn load(root: &Path) -> Result<Self, String> {
        let v: Value = serde_json::from_slice(&read(root, "manifest.json", 64 * 1024)?)
            .map_err(|e| e.to_string())?;
        keys(&v, &["version", "posters"])?;
        let entries = v["posters"].as_array().ok_or("海報數量無效")?;
        if v["version"].as_u64() != Some(1) || entries.is_empty() || entries.len() > 8 {
            return Err("海報版本或數量無效".into());
        }
        let mut seen = BTreeSet::new();
        let mut names = Vec::new();
        for entry in entries {
            keys(entry, &["id", "name", "left", "right"])?;
            let id = entry["id"].as_str().ok_or("無效海報代號")?;
            if !valid_id(id) || !seen.insert(id) {
                return Err("海報代號無效或重複".into());
            }
            let name = match entry.get("name") {
                None | Some(Value::Null) => "",
                Some(Value::String(s)) => s,
                _ => return Err("海報名稱須為字串".into()),
            };
            if name.chars().count() > 64 {
                return Err("海報名稱超限".into());
            }
            let sides = ["left", "right"].map(|k| entry[k].as_str().ok_or("缺少海報PNG"));
            let (left, right) = (sides[0].clone()?, sides[1].clone()?);
            for file in [left, right] {
                if !file.ends_with(".png")
                    || file.contains('/')
                    || file.contains('\\')
                    || file.len() < 5
                {
                    return Err("海報須為同目錄PNG".into());
                }
            }
            names.push((
                id.to_string(),
                name.to_string(),
                [left.to_string(), right.to_string()],
            ));
        }
        let mut total = 0usize;
        let mut decoded = Vec::new();
        for (id, name, files) in names {
            let mut pictures = Vec::new();
            for file in files {
                let bytes = read(root, &file, 8 * 1024 * 1024)?;
                let mut d = png::Decoder::new(Cursor::new(bytes));
                d.set_transformations(
                    png::Transformations::EXPAND | png::Transformations::STRIP_16,
                );
                let mut r = d.read_info().map_err(|e| e.to_string())?;
                let (w, h) = (r.info().width, r.info().height);
                if w == 0 || h == 0 || w > 2048 || h > 2160 {
                    return Err("海報尺寸超限".into());
                }
                total += w as usize * h as usize * 4;
                if total > 32 * 1024 * 1024 {
                    return Err("海報解碼圖像超過32MiB".into());
                }
                let mut buffer = vec![0; r.output_buffer_size()];
                let info = r.next_frame(&mut buffer).map_err(|e| e.to_string())?;
                let raw = &buffer[..info.buffer_size()];
                let rgba = match info.color_type {
                    png::ColorType::Rgba => raw.to_vec(),
                    png::ColorType::Rgb => raw
                        .chunks_exact(3)
                        .flat_map(|v| [v[0], v[1], v[2], 255])
                        .collect(),
                    png::ColorType::Grayscale => raw.iter().flat_map(|&v| [v, v, v, 255]).collect(),
                    png::ColorType::GrayscaleAlpha => raw
                        .chunks_exact(2)
                        .flat_map(|v| [v[0], v[0], v[0], v[1]])
                        .collect(),
                    _ => return Err("無法展開PNG色彩格式".into()),
                };
                pictures.push(Picture { w, h, rgba });
            }
            let images = pictures.try_into().map_err(|_| "海報必須成對")?;
            decoded.push(Entry { id, name, images });
        }
        Ok(Self { entries: decoded })
    }
}
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct Layout {
    pub canvas: Rect,
    pub game: Rect,
    pub zoomed: bool,
}
pub fn zoom_safe(f: &Image) -> bool {
    if f.w == 0 || f.h == 0 || f.rgba.len() != f.w as usize * f.h as usize * 4 {
        return false;
    }
    let rows = f.h.div_ceil(32);
    for range in [0..rows, f.h - rows..f.h] {
        for y in range {
            for x in 0..f.w {
                let p = ((y * f.w + x) * 4) as usize;
                if f.rgba[p] | f.rgba[p + 1] | f.rgba[p + 2] != 0 {
                    return false;
                }
            }
        }
    }
    true
}
pub fn geometry(w: u32, h: u32, zoom: bool) -> Layout {
    let (w, h) = (w.max(1), h.max(1));
    let (cw, ch) = if w as u64 * 9 <= h as u64 * 16 {
        (w, (w * 9 / 16).max(1))
    } else {
        ((h * 16 / 9).max(1), h)
    };
    let canvas = Rect::new(((w - cw) / 2) as i32, ((h - ch) / 2) as i32, cw, ch);
    let base = crate::display::fit(cw, ch);
    let (gw, gh) = if zoom {
        (base.width() * 16 / 15, base.height() * 16 / 15)
    } else {
        (base.width(), base.height())
    };
    let game = Rect::new(
        canvas.x() + (cw / 2) as i32 - (gw / 2) as i32,
        canvas.y() + (ch / 2) as i32 - (gh / 2) as i32,
        gw.max(1),
        gh.max(1),
    );
    Layout {
        canvas,
        game,
        zoomed: zoom,
    }
}
struct Art<'a> {
    id: String,
    name: String,
    textures: [Texture<'a>; 2],
}
struct Blend<'a> {
    image: Arc<Vec<u8>>,
    layout: Layout,
    source: (u32, u32),
    textures: Vec<(Rect, Texture<'a>)>,
}
pub struct Renderer<'a> {
    art: Vec<Art<'a>>,
    tc: &'a TextureCreator<WindowContext>,
    blend: Option<Blend<'a>>,
}
fn texture<'a>(
    tc: &'a TextureCreator<WindowContext>,
    w: u32,
    h: u32,
    rgba: &[u8],
    linear: bool,
) -> Result<Texture<'a>, String> {
    let mut t = tc
        .create_texture_static(PixelFormatEnum::RGBA32, w, h)
        .map_err(|e| e.to_string())?;
    t.set_blend_mode(BlendMode::Blend);
    t.set_scale_mode(if linear {
        ScaleMode::Linear
    } else {
        ScaleMode::Nearest
    });
    t.update(None, rgba, w as usize * 4)
        .map_err(|e| e.to_string())?;
    Ok(t)
}
fn cover(c: &mut Canvas<Window>, t: &mut Texture<'_>, r: Rect, soft: bool) -> Result<(), String> {
    let q = t.query();
    let scale = (r.width() as f64 / q.width as f64).max(r.height() as f64 / q.height as f64);
    let (w, h) = (q.width as f64 * scale, q.height as f64 * scale);
    c.set_clip_rect(r);
    t.set_color_mod(
        if soft { 230 } else { 255 },
        if soft { 230 } else { 255 },
        if soft { 230 } else { 255 },
    );
    c.copy_f(
        t,
        None,
        FRect::new(
            (r.x() as f64 + (r.width() as f64 - w) / 2.) as f32,
            (r.y() as f64 + (r.height() as f64 - h) / 2.) as f32,
            w as f32,
            h as f32,
        ),
    )
}
fn strip(f: &Image, l: Layout, side: usize, band: u32) -> Vec<u8> {
    let mut out = vec![0; band as usize * l.canvas.height() as usize * 4];
    let sx = l.game.width() as f64 / f.w as f64;
    let sy = l.game.height() as f64 / f.h as f64;
    let blur = 5. / sx;
    for y in 0..l.canvas.height() {
        for x in 0..band {
            let distance = if side == 0 {
                band as f64 - x as f64 - 0.5
            } else {
                x as f64 + 0.5
            };
            let px = if side == 0 {
                distance / sx
            } else {
                f.w as f64 - distance / sx
            };
            let py = (l.canvas.y() as f64 + y as f64 + 0.5 - l.game.y() as f64) / sy;
            let mut sum = [0u32; 4];
            for dy in -1..=1 {
                for dx in -1..=1 {
                    let ix = (px + dx as f64 * blur).floor() as i32;
                    let iy = (py + dy as f64 * blur).floor() as i32;
                    if ix >= 0 && iy >= 0 && ix < f.w as i32 && iy < f.h as i32 {
                        let p = ((iy as u32 * f.w + ix as u32) * 4) as usize;
                        for k in 0..4 {
                            sum[k] += f.rgba[p + k] as u32;
                        }
                    }
                }
            }
            let p = ((y * band + x) * 4) as usize;
            if sum[3] > 0 {
                for k in 0..3 {
                    out[p + k] = (sum[k] as f64 * 255. / sum[3] as f64)
                        .round()
                        .clamp(0., 255.) as u8;
                }
                out[p + 3] = (sum[3] as f64 / 9. * (1. - distance / band as f64).clamp(0., 1.))
                    .round()
                    .clamp(0., 255.) as u8;
            }
        }
    }
    out
}
impl<'a> Renderer<'a> {
    pub fn new(tc: &'a TextureCreator<WindowContext>, pack: Pack) -> Result<Self, String> {
        let mut art = Vec::new();
        for entry in pack.entries {
            let [left, right] = entry.images;
            let textures = [
                texture(tc, left.w, left.h, &left.rgba, true)?,
                texture(tc, right.w, right.h, &right.rgba, true)?,
            ];
            art.push(Art {
                id: entry.id,
                name: entry.name,
                textures,
            });
        }
        Ok(Self {
            art,
            tc,
            blend: None,
        })
    }
    pub fn next(&self, id: &str) -> String {
        match self.art.iter().position(|a| a.id == id) {
            Some(i) if i + 1 == self.art.len() => String::new(),
            Some(i) => self.art[i + 1].id.clone(),
            None => self.art[0].id.clone(),
        }
    }
    pub fn selected(&self, id: &str) -> bool {
        self.art.iter().any(|a| a.id == id)
    }
    pub fn name(&self, id: &str) -> &str {
        self.art
            .iter()
            .find(|a| a.id == id)
            .map(|a| a.name.as_str())
            .unwrap_or("關閉")
    }
    pub fn draw(
        &mut self,
        c: &mut Canvas<Window>,
        f: &Image,
        game_texture: &Texture<'_>,
        choice: &Choice,
    ) -> Result<Option<Layout>, String> {
        let Some(index) = self.art.iter().position(|a| a.id == choice.id) else {
            self.blend = None;
            return Ok(None);
        };
        let (w, h) = c.output_size()?;
        let l = geometry(w, h, choice.zoom && zoom_safe(f));
        let side_width = (l.game.x() - l.canvas.x()).max(0) as u32;
        if side_width > 0 {
            cover(
                c,
                &mut self.art[index].textures[0],
                Rect::new(l.canvas.x(), l.canvas.y(), side_width, l.canvas.height()),
                choice.soft,
            )?;
            let right = (l.canvas.right() - l.game.right()).max(0) as u32;
            if right > 0 {
                cover(
                    c,
                    &mut self.art[index].textures[1],
                    Rect::new(l.game.right(), l.canvas.y(), right, l.canvas.height()),
                    choice.soft,
                )?;
            }
            if choice.soft {
                let band = side_width.min((l.canvas.width() * 48 / 1920).max(1));
                let update = self.blend.as_ref().is_none_or(|b| {
                    !Arc::ptr_eq(&b.image, &f.rgba) || b.layout != l || b.source != (f.w, f.h)
                });
                if update {
                    let mut textures = Vec::new();
                    for side in 0..2 {
                        let x = if side == 0 {
                            l.game.x() - band as i32
                        } else {
                            l.game.right()
                        };
                        let bytes = strip(f, l, side, band);
                        textures.push((
                            Rect::new(x, l.canvas.y(), band, l.canvas.height()),
                            texture(self.tc, band, l.canvas.height(), &bytes, false)?,
                        ));
                    }
                    self.blend = Some(Blend {
                        image: f.rgba.clone(),
                        layout: l,
                        source: (f.w, f.h),
                        textures,
                    });
                }
                if let Some(b) = &self.blend {
                    for (r, t) in &b.textures {
                        c.set_clip_rect(*r);
                        c.copy(t, None, *r)?;
                    }
                }
            } else {
                self.blend = None;
            }
        }
        c.set_clip_rect(l.canvas);
        c.copy(game_texture, None, l.game)?;
        c.set_clip_rect(None);
        Ok(Some(l))
    }
}
#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn hud_protects_zoom_and_geometry() {
        let mut bytes = vec![0; 256 * 240 * 4];
        for y in 8..232 {
            for x in 0..256 {
                bytes[(y * 256 + x) * 4] = 255;
            }
        }
        let mut f = Image {
            field: 0,
            w: 256,
            h: 240,
            rgba: Arc::new(bytes),
            movie: None,
        };
        assert!(zoom_safe(&f));
        let l = geometry(1920, 1080, true);
        assert_eq!(l.game, Rect::new(192, -36, 1536, 1152));
        assert_eq!(
            geometry(1920, 1080, false).game,
            Rect::new(240, 0, 1440, 1080)
        );
        assert_eq!(
            geometry(1280, 960, false).canvas,
            Rect::new(0, 120, 1280, 720)
        );
        Arc::make_mut(&mut f.rgba)[7 * 256 * 4] = 1;
        assert!(!zoom_safe(&f));
    }
    #[test]
    fn reject_unknown_fields_and_duplicate_ids() {
        let root =
            std::env::temp_dir().join(format!("psgolem-poster-loader-{}", std::process::id()));
        fs::create_dir(&root).unwrap();
        fs::write(
            root.join("manifest.json"),
            br#"{"version":1,"unknown":1,"posters":[]}"#,
        )
        .unwrap();
        assert!(Pack::load(&root).err().unwrap().contains("未知"));
        fs::write(root.join("manifest.json"),br#"{"version":1,"posters":[{"id":"a","left":"x.png","right":"y.png"},{"id":"a","left":"x.png","right":"y.png"}]}"#).unwrap();
        assert!(Pack::load(&root).err().unwrap().contains("重複"));
        fs::remove_dir_all(root).unwrap();
    }
}
