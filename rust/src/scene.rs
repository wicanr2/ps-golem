//! Typed CPU scene canvas, spec026 §16.4, GPL-3.0-or-later.
//! Data and direct pixel writes only. No Machine, guest CPU or GP0 parser.
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash)]
pub struct MaterialKey {
    pub upload: [u8; 32],
    pub clut: [u8; 32],
}
#[derive(Clone, Debug)]
pub struct Material {
    pub key: MaterialKey,
    pub width: usize,
    pub height: usize,
    pub packed: Vec<u8>,
    pub palette: [u16; 16],
}
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Sprite {
    pub material: usize,
    pub x: i32,
    pub y: i32,
    pub u: usize,
    pub v: usize,
    pub width: usize,
    pub height: usize,
}
#[derive(Clone, Debug)]
pub struct Scene {
    pub left: i32,
    pub width: usize,
    pub height: usize,
    pub materials: Vec<Material>,
    pub sprites: Vec<Sprite>,
}
#[derive(Clone, Debug)]
pub enum ImagePixels {
    Indexed(Vec<u8>),
    Colors(Vec<u16>),
}
#[derive(Clone, Debug)]
pub struct Image {
    pub width: usize,
    pub height: usize,
    pub scale: usize,
    pub pixels: ImagePixels,
}
pub struct Canvas {
    pub width: usize,
    pub height: usize,
    pub words: Vec<u16>,
}
impl Canvas {
    pub fn rgba(&self) -> Vec<u8> {
        let mut result = vec![0; self.words.len() * 4];
        for (pixel, &word) in result.chunks_exact_mut(4).zip(&self.words) {
            if word == 0 {
                continue;
            }
            for (c, shift) in pixel[..3].iter_mut().zip([0, 5, 10]) {
                let v = ((word >> shift) & 31) as u8;
                *c = (v << 3) | (v >> 2);
            }
            pixel[3] = 255;
        }
        result
    }
}
impl Scene {
    pub fn render(&self, scale: usize, images: Option<&[Image]>) -> Result<Canvas, String> {
        if ![1, 4].contains(&scale)
            || self.width == 0
            || self.width > 1024
            || self.height == 0
            || self.height > 512
            || !(-1024..=1024).contains(&self.left)
            || self.materials.len() > 512
            || self.sprites.len() > 8192
            || (scale == 4) != images.is_some()
            || images.is_some_and(|v| v.len() != self.materials.len())
        {
            return Err("場景／縮放或完整HD圖像不符".into());
        }
        for (i, m) in self.materials.iter().enumerate() {
            if m.width == 0
                || m.width > 4096
                || m.width % 2 != 0
                || m.height == 0
                || m.height > 512
                || m.packed.len() != m.width * m.height / 2
            {
                return Err("原材質形狀不符".into());
            }
            if let Some(images) = images {
                let im = &images[i];
                let n = m.width * scale * m.height * scale;
                if im.scale != scale || im.width != m.width * scale || im.height != m.height * scale
                {
                    return Err("HD材質形狀不符".into());
                }
                match &im.pixels {
                    ImagePixels::Indexed(v) if v.len() == n && v.iter().all(|&x| x < 16) => (),
                    ImagePixels::Colors(v) if v.len() == n => (),
                    _ => return Err("HD色字或索引不符".into()),
                }
            }
        }
        for sprite in &self.sprites {
            let m = self.materials.get(sprite.material).ok_or("圖塊材質超界")?;
            if !(-1048576..=1048576).contains(&sprite.x)
                || !(-1048576..=1048576).contains(&sprite.y)
                || sprite.width == 0
                || sprite.width > 1024
                || sprite.height == 0
                || sprite.height > 512
                || sprite
                    .u
                    .checked_add(sprite.width)
                    .is_none_or(|n| n > m.width)
                || sprite
                    .v
                    .checked_add(sprite.height)
                    .is_none_or(|n| n > m.height)
            {
                return Err("圖塊來源或位置超界".into());
            }
        }
        let width = self.width * scale;
        let height = self.height * scale;
        let mut words = vec![0; width * height];
        for s in &self.sprites {
            let m = &self.materials[s.material];
            let dx = (s.x - self.left) * scale as i32;
            let dy = s.y * scale as i32;
            let x0 = dx.max(0);
            let y0 = dy.max(0);
            let x1 = (dx + s.width as i32 * scale as i32).min(width as i32);
            let y1 = (dy + s.height as i32 * scale as i32).min(height as i32);
            for y in y0..y1 {
                for x in x0..x1 {
                    let u = s.u * scale + (x - dx) as usize;
                    let v = s.v * scale + (y - dy) as usize;
                    let word = if let Some(images) = images {
                        match &images[s.material].pixels {
                            ImagePixels::Indexed(p) => {
                                m.palette[p[v * images[s.material].width + u] as usize]
                            }
                            ImagePixels::Colors(p) => p[v * images[s.material].width + u],
                        }
                    } else {
                        let at = v * m.width + u;
                        let index = (m.packed[at / 2] >> ((at & 1) * 4)) & 15;
                        m.palette[index as usize]
                    };
                    if word != 0 {
                        words[y as usize * width + x as usize] = word;
                    }
                }
            }
        }
        Ok(Canvas {
            width,
            height,
            words,
        })
    }
}
#[cfg(test)]
mod tests {
    use super::*;
    fn scene() -> Scene {
        Scene {
            left: 0,
            width: 2,
            height: 1,
            materials: vec![Material {
                key: MaterialKey {
                    upload: [0; 32],
                    clut: [0; 32],
                },
                width: 2,
                height: 1,
                packed: vec![0x21],
                palette: {
                    let mut p = [0; 16];
                    p[1] = 31;
                    p[2] = 0x8000;
                    p
                },
            }],
            sprites: vec![Sprite {
                material: 0,
                x: 0,
                y: 0,
                u: 0,
                v: 0,
                width: 2,
                height: 1,
            }],
        }
    }
    #[test]
    fn opaque_black_order_and_transparency() {
        let mut s = scene();
        assert_eq!(s.render(1, None).unwrap().words, vec![31, 0x8000]);
        s.sprites.push(Sprite {
            x: 1,
            width: 1,
            ..s.sprites[0].clone()
        });
        assert_eq!(s.render(1, None).unwrap().words, vec![31, 31]);
        s.materials[0].palette[1] = 0;
        assert_eq!(
            s.render(1, None).unwrap().rgba(),
            vec![0, 0, 0, 0, 0, 0, 0, 255]
        );
    }
    #[test]
    fn clipping_and_full_hd_image_requirement() {
        let mut s = scene();
        s.sprites[0].x = -1;
        assert_eq!(s.render(1, None).unwrap().words, vec![0x8000, 0]);
        assert!(s.render(4, None).is_err());
        let im = Image {
            width: 8,
            height: 4,
            scale: 4,
            pixels: ImagePixels::Colors(vec![0x8000; 32]),
        };
        let c = s.render(4, Some(&[im])).unwrap();
        assert_eq!(c.words.iter().filter(|&&w| w == 0x8000).count(), 16);
    }
    #[test]
    fn reject_bad_source_and_hd_indices_before_canvas() {
        let mut s = scene();
        s.sprites[0].u = usize::MAX;
        assert!(s.render(1, None).is_err());
        s.sprites[0].u = 0;
        let im = Image {
            width: 8,
            height: 4,
            scale: 4,
            pixels: ImagePixels::Indexed(vec![16; 32]),
        };
        assert!(s.render(4, Some(&[im])).is_err());
        s.materials[0].packed.clear();
        assert!(s.render(1, None).is_err());
    }
}
