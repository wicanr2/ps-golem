use super::*;
impl Gpu {
    pub(super) fn draw_sprite_command(&mut self, cb: &[u32]) {
        let cmd = cb[0] >> 24;
        let textured = cmd & 4 != 0;
        let raw = cmd & 1 != 0;
        let color = cb[0] & 0xffffff;
        let mut i = 2;
        let mut u = 0;
        let mut v = 0;
        if textured {
            u = cb[i] as u8;
            v = (cb[i] >> 8) as u8;
            self.update_clut(cb[i] >> 16 & 0xffff, self.tex_mode);
            i += 1;
        }
        let (w, h) = match cmd >> 3 & 3 {
            0 => ((cb[i] & 1023) as i32, (cb[i] >> 16 & 511) as i32),
            1 => (1, 1),
            2 => (8, 8),
            _ => (16, 16),
        };
        let x = sx11((sx11(cb[1] & 0xffff) + self.off_x) as u32);
        let y = sx11((sx11(cb[1] >> 16) + self.off_y) as u32);
        let mode = self.blend_mode(cmd);
        self.draw_sprite(x, y, w, h, u, v, color, textured, textured && !raw, mode);
    }
    #[allow(clippy::too_many_arguments)]
    fn draw_sprite(
        &mut self,
        xa: i32,
        ya: i32,
        w: i32,
        h: i32,
        mut u: u8,
        mut v: u8,
        color: u32,
        textured: bool,
        mut modulate: bool,
        mode: i32,
    ) {
        modulate &= color & 0xffffff != 0x808080;
        let r = color & 255;
        let g = color >> 8 & 255;
        let b = color >> 16 & 255;
        let fill = (0x8000 | r >> 3 | (g >> 3) << 5 | (b >> 3) << 10) as u16;
        let (mut xs, mut xb, mut ys, mut yb) = (xa, xa + w, ya, ya + h);
        let (mut ui, mut vi) = (1, 1);
        if textured {
            if self.sprite_flip & 0x1000 != 0 {
                ui = -1;
                u |= 1;
            }
            if self.sprite_flip & 0x2000 != 0 {
                vi = -1;
            }
        }
        if xs < self.clip_x0 {
            if textured {
                u = u.wrapping_add(((self.clip_x0 - xs) * ui) as u8);
            }
            xs = self.clip_x0;
        }
        if ys < self.clip_y0 {
            if textured {
                v = v.wrapping_add(((self.clip_y0 - ys) * vi) as u8);
            }
            ys = self.clip_y0;
        }
        xb = xb.min(self.clip_x1 + 1);
        yb = yb.min(self.clip_y1 + 1);
        for y in ys..yb {
            let mut ur = u;
            if !self.line_skip(y as u32) {
                for x in xs..xb {
                    if textured {
                        let mut t = self.get_texel(self.tex_mode, ur as u32, v as u32);
                        if t != 0 {
                            if modulate {
                                t = self.mod_texel(2, 3, t, r, g, b);
                            }
                            self.plot(x, y, t, mode, true);
                        }
                        ur = (ur as i32 + ui) as u8;
                    } else {
                        self.plot(x, y, fill, mode, false);
                    }
                }
            }
            if textured {
                v = (v as i32 + vi) as u8;
            }
        }
    }
    pub(super) fn draw_line_command(&mut self, cb: &[u32]) {
        let cmd = cb[0] >> 24;
        let gouraud = cmd & 16 != 0;
        let mut p = [Point::default(); 2];
        p[0] = Point {
            x: sx11(cb[1] & 0xffff) + self.off_x,
            y: sx11(cb[1] >> 16) + self.off_y,
            r: cb[0] & 255,
            g: cb[0] >> 8 & 255,
            b: cb[0] >> 16 & 255,
        };
        let k = if gouraud {
            p[1].r = cb[2] & 255;
            p[1].g = cb[2] >> 8 & 255;
            p[1].b = cb[2] >> 16 & 255;
            3
        } else {
            p[1].r = p[0].r;
            p[1].g = p[0].g;
            p[1].b = p[0].b;
            2
        };
        p[1].x = sx11(cb[k] & 0xffff) + self.off_x;
        p[1].y = sx11(cb[k] >> 16) + self.off_y;
        if cmd & 8 != 0 {
            self.pline_prev = p[1];
            self.pline_cmd = cmd;
            self.pline_n = 0;
            self.in_cmd = POLYLINE;
        }
        self.line_checked(p, gouraud, self.blend_mode(cmd));
    }
    pub(super) fn polyline_word(&mut self, v: u32) {
        let gouraud = self.pline_cmd & 16 != 0;
        if self.pline_n == 0 && v & 0xf000f000 == 0x50005000 {
            self.in_cmd = NONE;
            return;
        }
        self.pline_group[self.pline_n] = v;
        self.pline_n += 1;
        if self.pline_n < (if gouraud { 2 } else { 1 }) {
            return;
        }
        let mut p = [self.pline_prev; 2];
        let mut k = 0;
        if gouraud {
            let c = self.pline_group[0];
            p[1].r = c & 255;
            p[1].g = c >> 8 & 255;
            p[1].b = c >> 16 & 255;
            k = 1;
        }
        let w = self.pline_group[k];
        p[1].x = sx11(w & 0xffff) + self.off_x;
        p[1].y = sx11(w >> 16) + self.off_y;
        self.pline_n = 0;
        self.pline_prev = p[1];
        self.line_checked(p, gouraud, self.blend_mode(self.pline_cmd));
    }
    fn line_checked(&mut self, mut p: [Point; 2], gouraud: bool, mode: i32) {
        let dx = (p[1].x - p[0].x).abs();
        let dy = (p[1].y - p[0].y).abs();
        if dx >= 1024 || dy >= 512 {
            return;
        }
        let k = dx.max(dy);
        if p[0].x > p[1].x && k != 0 {
            p.swap(0, 1);
        }
        let (mut sx, mut sy) = (0i64, 0i64);
        let (mut sr, mut sg, mut sb) = (0i32, 0i32, 0i32);
        if k != 0 {
            sx = line_divide((p[1].x - p[0].x) as i64, k);
            sy = line_divide((p[1].y - p[0].y) as i64, k);
            if gouraud {
                sr = (p[1].r.wrapping_sub(p[0].r).wrapping_shl(12) as i32) / k;
                sg = (p[1].g.wrapping_sub(p[0].g).wrapping_shl(12) as i32) / k;
                sb = (p[1].b.wrapping_sub(p[0].b).wrapping_shl(12) as i32) / k;
            }
        }
        let mut cx = ((p[0].x as i64 as u64).wrapping_shl(32) | 1 << 31).wrapping_sub(1024);
        let mut cy = (p[0].y as i64 as u64).wrapping_shl(32) | 1 << 31;
        if sy < 0 {
            cy = cy.wrapping_sub(1024);
        }
        let (mut cr, mut cg, mut cb) = (0u32, 0u32, 0u32);
        if gouraud {
            cr = p[0].r << 12 | 1 << 11;
            cg = p[0].g << 12 | 1 << 11;
            cb = p[0].b << 12 | 1 << 11;
        }
        for _ in 0..=k {
            let x = (cx >> 32) as i32 & 2047;
            let y = (cy >> 32) as i32 & 2047;
            if !self.line_skip(y as u32) {
                let (r, g, b) = if gouraud {
                    (cr >> 12 & 255, cg >> 12 & 255, cb >> 12 & 255)
                } else {
                    (p[0].r, p[0].g, p[0].b)
                };
                let pix = if self.dtd {
                    0x8000
                        | self.dither[(y & 3) as usize][r as usize][(x & 3) as usize] as u16
                        | (self.dither[(y & 3) as usize][g as usize][(x & 3) as usize] as u16) << 5
                        | (self.dither[(y & 3) as usize][b as usize][(x & 3) as usize] as u16) << 10
                } else {
                    (0x8000 | r >> 3 | (g >> 3) << 5 | (b >> 3) << 10) as u16
                };
                if x >= self.clip_x0 && x <= self.clip_x1 && y >= self.clip_y0 && y <= self.clip_y1
                {
                    self.plot(x, y, pix, mode, false);
                }
            }
            cx = cx.wrapping_add(sx as u64);
            cy = cy.wrapping_add(sy as u64);
            if gouraud {
                cr = cr.wrapping_add(sr as u32);
                cg = cg.wrapping_add(sg as u32);
                cb = cb.wrapping_add(sb as u32);
            }
        }
    }
}
fn line_divide(delta: i64, k: i32) -> i64 {
    let mut d = (delta as u64).wrapping_shl(32) as i64;
    if d < 0 {
        d -= k as i64 - 1;
    }
    if d > 0 {
        d += k as i64 - 1;
    }
    d / (k as i64)
}
