//! Direct environment batch, 026 §16.15, GPL-3.0-or-later.
//! Own native GPU environment/fill contract, no GP0/GP1 parsing or RAM list.
use super::{transfer::Rect, Gpu, NONE};
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct DrawEnvironment {
    pub area: Rect,
    pub offset: [i16; 2],
    pub page: u16,
    pub color: [u8; 3],
}
impl Gpu {
    pub fn apply_draw_environment(&mut self, e: DrawEnvironment) -> Result<(), String> {
        let r = e.area;
        if self.in_cmd != NONE
            || self.len != 0
            || self.need != 0
            || r.width == 0
            || r.height == 0
            || r.width >= 1024
            || r.height >= 512
            || r.x % 16 != 0
            || r.width % 16 != 0
            || r.x as u32 + r.width as u32 > 1024
            || r.y as u32 + r.height as u32 > 512
            || e.offset.iter().any(|&n| !(-1024..=1023).contains(&n))
            || e.page > 0x3fff
        {
            return Err("繪圖環境、填色邊界或GPU parser不符".into());
        }
        let mut g = self.clone();
        g.clip_x0 = r.x as i32;
        g.clip_y0 = r.y as i32;
        g.clip_x1 = (r.x + r.width - 1) as i32;
        g.clip_y1 = (r.y + r.height - 1) as i32;
        g.off_x = e.offset[0] as i32;
        g.off_y = e.offset[1] as i32;
        g.set_page(e.page as u32);
        g.sprite_flip = e.page as u32 & 0x3000;
        g.dtd = e.page & 0x200 != 0;
        g.dfe = e.page & 0x400 != 0;
        g.tww = 0;
        g.twh = 0;
        g.twx = 0;
        g.twy = 0;
        g.recalc_window();
        g.mask_or = 0;
        g.mask_and = 0;
        let c = e.color;
        let value = (c[0] as u16 >> 3) | ((c[1] as u16 >> 3) << 5) | ((c[2] as u16 >> 3) << 10);
        for y in r.y..r.y + r.height {
            if !g.line_skip(y as u32) {
                let at = y as usize * 1024 + r.x as usize;
                g.pixels[at..at + r.width as usize].fill(value);
            }
        }
        g.buf[0] = 0x02000000 | c[0] as u32 | ((c[1] as u32) << 8) | ((c[2] as u32) << 16);
        g.buf[1] = r.x as u32 | ((r.y as u32) << 16);
        g.buf[2] = r.width as u32 | ((r.height as u32) << 16);
        g.dma = 2;
        *self = g;
        Ok(())
    }
}
