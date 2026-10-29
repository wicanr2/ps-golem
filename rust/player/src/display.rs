//! Native 4:3 presentation, independent of the VRAM storage dimensions.
use sdl2::{
    pixels::PixelFormatEnum,
    rect::Rect,
    render::{Canvas, TextureCreator},
    video::{Window, WindowContext},
};
use std::{collections::BTreeMap, io::Cursor, path::Path};
pub fn fit(w: u32, h: u32) -> Rect {
    let (fw, fh) = if w as u64 * 3 <= h as u64 * 4 {
        (w, (w * 3 / 4).max(1))
    } else {
        ((h * 4 / 3).max(1), h)
    };
    Rect::new(((w - fw) / 2) as i32, ((h - fh) / 2) as i32, fw, fh)
}
pub fn png(path: &Path, w: u32, h: u32, rgba: &[u8]) -> Result<(), String> {
    let mut e = png::Encoder::new(
        std::io::BufWriter::new(std::fs::File::create(path).map_err(|e| e.to_string())?),
        w,
        h,
    );
    e.set_color(png::ColorType::Rgba);
    e.set_depth(png::BitDepth::Eight);
    e.write_header()
        .map_err(|e| e.to_string())?
        .write_image_data(rgba)
        .map_err(|e| e.to_string())
}
pub struct Font {
    rgba: Vec<u8>,
    w: u32,
    h: u32,
    glyphs: BTreeMap<char, [u32; 5]>,
}
impl Font {
    pub fn load() -> Result<Self, String> {
        let decoder = png::Decoder::new(Cursor::new(include_bytes!(
            "../../../frontend/ebiten/ui-font.png"
        )));
        let mut r = decoder.read_info().map_err(|e| e.to_string())?;
        let mut bytes = vec![0; r.output_buffer_size()];
        let info = r.next_frame(&mut bytes).map_err(|e| e.to_string())?;
        if info.color_type != png::ColorType::Grayscale || info.bit_depth != png::BitDepth::Eight {
            return Err("Unexpected UI font atlas format".into());
        }
        let rgba = bytes[..info.buffer_size()]
            .iter()
            .flat_map(|a| [255, 255, 255, *a])
            .collect();
        let v: serde_json::Value =
            serde_json::from_str(include_str!("../../../frontend/ebiten/ui-font.json"))
                .map_err(|e| e.to_string())?;
        let mut glyphs = BTreeMap::new();
        for (k, g) in v["glyphs"].as_object().ok_or("Invalid font glyphs")? {
            let vals = ["x", "y", "w", "h", "advance"].map(|n| g[n].as_u64().unwrap_or(0) as u32);
            if vals[0] + vals[2] > info.width || vals[1] + vals[3] > info.height {
                return Err("Font bounds".into());
            }
            if let Some(c) = k.chars().next() {
                glyphs.insert(c, vals);
            }
        }
        Ok(Self {
            rgba,
            w: info.width,
            h: info.height,
            glyphs,
        })
    }
    pub fn texture<'a>(
        &self,
        tc: &'a TextureCreator<WindowContext>,
    ) -> Result<sdl2::render::Texture<'a>, String> {
        let mut t = tc
            .create_texture_static(PixelFormatEnum::RGBA32, self.w, self.h)
            .map_err(|e| e.to_string())?;
        t.set_blend_mode(sdl2::render::BlendMode::Blend);
        t.update(None, &self.rgba, self.w as usize * 4)
            .map_err(|e| e.to_string())?;
        Ok(t)
    }
    pub fn draw(
        &self,
        c: &mut Canvas<Window>,
        t: &sdl2::render::Texture<'_>,
        text: &str,
        x: i32,
        y: i32,
    ) -> Result<(), String> {
        let mut dx = x;
        for ch in text.chars() {
            if let Some(&[sx, sy, w, h, adv]) = self.glyphs.get(&ch).or(self.glyphs.get(&'?')) {
                c.copy(
                    t,
                    Rect::new(sx as i32, sy as i32, w, h),
                    Rect::new(dx, y, w, h),
                )?;
                dx += adv as i32;
            }
        }
        Ok(())
    }
}
#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn storage_is_not_aspect() {
        assert_eq!(fit(1920, 1080), Rect::new(240, 0, 1440, 1080));
        assert_eq!(fit(800, 800), Rect::new(0, 100, 800, 600));
        assert_eq!(fit(960, 720), Rect::new(0, 0, 960, 720));
        let f = Font::load().unwrap();
        assert!(f.glyphs.contains_key(&'手'));
        assert!(f.glyphs.contains_key(&'把'));
    }
}
