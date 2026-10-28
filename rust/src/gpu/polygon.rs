use super::*;

#[derive(Clone, Copy, Default)]
struct Vertex {
    x: i32,
    y: i32,
    u: i32,
    v: i32,
    r: i32,
    g: i32,
    b: i32,
}
#[derive(Clone, Copy)]
struct Flags {
    gouraud: bool,
    textured: bool,
    modulate: bool,
    mode: i32,
    tm: u32,
}
#[derive(Clone, Copy, Default)]
struct Interp {
    u: u32,
    v: u32,
    r: u32,
    g: u32,
    b: u32,
}
#[derive(Clone, Copy, Default)]
struct Delta {
    ux: u32,
    vx: u32,
    rx: u32,
    gx: u32,
    bx: u32,
    uy: u32,
    vy: u32,
    ry: u32,
    gy: u32,
    by: u32,
}
#[derive(Clone, Copy, Default)]
struct Part {
    coord: [u64; 2],
    step: [u64; 2],
    y: i32,
    bound: i32,
    dec: bool,
}

fn area(ax: i32, ay: i32, bx: i32, by: i32, cx: i32, cy: i32) -> i32 {
    bx.wrapping_sub(ax)
        .wrapping_mul(cy.wrapping_sub(by))
        .wrapping_sub(cx.wrapping_sub(bx).wrapping_mul(by.wrapping_sub(ay)))
}
fn xfp(x: i32) -> i64 {
    ((x as u32 as u64) << 32).wrapping_add(0xfffff800) as i64
}
fn step(dx: i32, dy: i32) -> i64 {
    let mut v = (dx as i64) << 32;
    if v < 0 {
        v -= dy as i64 - 1;
    }
    if v > 0 {
        v += dy as i64 - 1;
    }
    v / (dy as i64)
}
fn add_x(v: &mut Interp, d: &Delta, n: u32, f: Flags) {
    if f.textured {
        v.u = v.u.wrapping_add(d.ux.wrapping_mul(n));
        v.v = v.v.wrapping_add(d.vx.wrapping_mul(n));
    }
    if f.gouraud {
        v.r = v.r.wrapping_add(d.rx.wrapping_mul(n));
        v.g = v.g.wrapping_add(d.gx.wrapping_mul(n));
        v.b = v.b.wrapping_add(d.bx.wrapping_mul(n));
    }
}
fn add_y(v: &mut Interp, d: &Delta, n: u32, f: Flags) {
    if f.textured {
        v.u = v.u.wrapping_add(d.uy.wrapping_mul(n));
        v.v = v.v.wrapping_add(d.vy.wrapping_mul(n));
    }
    if f.gouraud {
        v.r = v.r.wrapping_add(d.ry.wrapping_mul(n));
        v.g = v.g.wrapping_add(d.gy.wrapping_mul(n));
        v.b = v.b.wrapping_add(d.by.wrapping_mul(n));
    }
}
fn deltas(a: Vertex, b: Vertex, c: Vertex, f: Flags) -> Option<Delta> {
    let den = area(a.x, a.y, b.x, b.y, c.x, c.y);
    if den == 0 {
        return None;
    }
    let q = |v: i32| (v.wrapping_mul(4096).wrapping_div(den) as u32) << 12;
    let mut d = Delta::default();
    if f.gouraud {
        d.rx = q(area(a.r, a.y, b.r, b.y, c.r, c.y));
        d.ry = q(area(a.x, a.r, b.x, b.r, c.x, c.r));
        d.gx = q(area(a.g, a.y, b.g, b.y, c.g, c.y));
        d.gy = q(area(a.x, a.g, b.x, b.g, c.x, c.g));
        d.bx = q(area(a.b, a.y, b.b, b.y, c.b, c.y));
        d.by = q(area(a.x, a.b, b.x, b.b, c.x, c.b));
    }
    if f.textured {
        d.ux = q(area(a.u, a.y, b.u, b.y, c.u, c.y));
        d.uy = q(area(a.x, a.u, b.x, b.u, c.x, c.u));
        d.vx = q(area(a.v, a.y, b.v, b.y, c.v, c.y));
        d.vy = q(area(a.x, a.v, b.x, b.v, c.x, c.v));
    }
    Some(d)
}
impl Gpu {
    pub(super) fn draw_polygon(&mut self, cb: &[u32]) {
        let cmd = cb[0] >> 24;
        let textured = cmd & 4 != 0;
        let gouraud = cmd & 16 != 0;
        if textured {
            self.set_page(cb[if gouraud { 5 } else { 4 }] >> 16);
        }
        let f = Flags {
            gouraud,
            textured,
            modulate: textured && cmd & 1 == 0,
            mode: self.blend_mode(cmd),
            tm: self.tex_mode,
        };
        let nv = if cmd & 8 != 0 { 4 } else { 3 };
        let mut vs = [Vertex::default(); 4];
        let mut i = 0;
        for n in 0..nv {
            if n == 0 || gouraud {
                let c = cb[i] & 0xffffff;
                vs[n].r = (c & 255) as i32;
                vs[n].g = (c >> 8 & 255) as i32;
                vs[n].b = (c >> 16 & 255) as i32;
                i += 1;
            } else {
                vs[n].r = vs[0].r;
                vs[n].g = vs[0].g;
                vs[n].b = vs[0].b;
            }
            vs[n].x = sx11(cb[i] & 0xffff) + self.off_x;
            vs[n].y = sx11(cb[i] >> 16) + self.off_y;
            i += 1;
            if textured {
                vs[n].u = (cb[i] & 255) as i32;
                vs[n].v = (cb[i] >> 8 & 255) as i32;
                if n == 0 {
                    self.update_clut(cb[i] >> 16 & 0xffff, f.tm);
                }
                i += 1;
            }
        }
        self.triangle_checked([vs[0], vs[1], vs[2]], f);
        if nv == 4 {
            self.triangle_checked([vs[1], vs[2], vs[3]], f);
        }
    }
    fn triangle_checked(&mut self, v: [Vertex; 3], f: Flags) {
        if (v[2].y - v[0].y).abs() >= 512
            || (v[2].y - v[1].y).abs() >= 512
            || (v[1].y - v[0].y).abs() >= 512
        {
            return;
        }
        if (v[2].x - v[0].x).abs() >= 1024
            || (v[2].x - v[1].x).abs() >= 1024
            || (v[1].x - v[0].x).abs() >= 1024
        {
            return;
        }
        self.triangle(v, f);
    }
    fn triangle(&mut self, mut vs: [Vertex; 3], f: Flags) {
        let mut cv = if vs[1].x <= vs[0].x {
            if vs[2].x <= vs[1].x {
                4
            } else {
                2
            }
        } else if vs[2].x < vs[0].x {
            4
        } else {
            1
        };
        if vs[2].y < vs[1].y {
            vs.swap(2, 1);
            cv = (cv >> 1) & 2 | (cv << 1) & 4 | cv & 1;
        }
        if vs[1].y < vs[0].y {
            vs.swap(1, 0);
            cv = (cv >> 1) & 1 | (cv << 1) & 2 | cv & 4;
        }
        if vs[2].y < vs[1].y {
            vs.swap(2, 1);
            cv = (cv >> 1) & 2 | (cv << 1) & 4 | cv & 1;
        }
        let core = cv >> 1;
        if vs[0].y == vs[2].y {
            return;
        }
        let Some(d) = deltas(vs[0], vs[1], vs[2], f) else {
            return;
        };
        let c = vs[core];
        let init = |v: i32| ((v as u32).wrapping_shl(12).wrapping_add(2048)).wrapping_shl(12);
        let mut ig = Interp {
            r: init(c.r),
            g: init(c.g),
            b: init(c.b),
            ..Interp::default()
        };
        if f.textured {
            ig.u = init(c.u);
            ig.v = init(c.v);
        }
        add_x(&mut ig, &d, c.x.wrapping_neg() as u32, f);
        add_y(&mut ig, &d, c.y.wrapping_neg() as u32, f);
        let base = xfp(vs[0].x);
        let bs = step(vs[2].x - vs[0].x, vs[2].y - vs[0].y);
        let (us, right) = if vs[1].y == vs[0].y {
            (0, vs[1].x > vs[0].x)
        } else {
            let s = step(vs[1].x - vs[0].x, vs[1].y - vs[0].y);
            (s, s > bs)
        };
        let ls = if vs[2].y == vs[1].y {
            0
        } else {
            step(vs[2].x - vs[1].x, vs[2].y - vs[1].y)
        };
        let vo = if core != 0 { 1 } else { 0 };
        let vp = if core == 2 { 3 } else { 0 };
        let rf = right as usize;
        let mut parts = [Part::default(); 2];
        {
            let t = &mut parts[vo];
            t.y = vs[vo].y;
            t.bound = vs[1 ^ vo].y;
            t.coord[rf] = xfp(vs[vo].x) as u64;
            t.step[rf] = us as u64;
            t.coord[1 - rf] =
                base.wrapping_add(((vs[vo].y - vs[0].y) as i64).wrapping_mul(bs)) as u64;
            t.step[1 - rf] = bs as u64;
            t.dec = vo != 0;
        }
        {
            let t = &mut parts[vo ^ 1];
            t.y = vs[1 ^ vp].y;
            t.bound = vs[2 ^ vp].y;
            t.coord[rf] = xfp(vs[1 ^ vp].x) as u64;
            t.step[rf] = ls as u64;
            t.coord[1 - rf] =
                base.wrapping_add(((vs[1 ^ vp].y - vs[0].y) as i64).wrapping_mul(bs)) as u64;
            t.step[1 - rf] = bs as u64;
            t.dec = vp != 0;
        }
        for t in parts {
            let mut yi = t.y;
            let mut lc = t.coord[0];
            let mut rc = t.coord[1];
            if t.dec {
                while yi > t.bound {
                    yi -= 1;
                    lc = lc.wrapping_sub(t.step[0]);
                    rc = rc.wrapping_sub(t.step[1]);
                    let y = sx11(yi as u32);
                    if y < self.clip_y0 {
                        break;
                    }
                    if y > self.clip_y1 {
                        continue;
                    }
                    self.span(
                        yi,
                        ((lc as i64) >> 32) as i32,
                        ((rc as i64) >> 32) as i32,
                        ig,
                        &d,
                        f,
                    );
                }
            } else {
                while yi < t.bound {
                    let y = sx11(yi as u32);
                    if y > self.clip_y1 {
                        break;
                    }
                    if y >= self.clip_y0 {
                        self.span(
                            yi,
                            ((lc as i64) >> 32) as i32,
                            ((rc as i64) >> 32) as i32,
                            ig,
                            &d,
                            f,
                        );
                    }
                    yi += 1;
                    lc = lc.wrapping_add(t.step[0]);
                    rc = rc.wrapping_add(t.step[1]);
                }
            }
        }
    }
    #[allow(clippy::too_many_arguments)]
    fn span(&mut self, y: i32, start: i32, bound: i32, mut ig: Interp, d: &Delta, f: Flags) {
        if self.line_skip(y as u32) {
            return;
        }
        let mut adj = start;
        let mut w = bound - start;
        let mut x = sx11(start as u32);
        if x < self.clip_x0 {
            let n = self.clip_x0 - x;
            adj += n;
            x += n;
            w -= n;
        }
        if x + w > self.clip_x1 + 1 {
            w = self.clip_x1 + 1 - x;
        }
        if w <= 0 {
            return;
        }
        add_x(&mut ig, d, adj as u32, f);
        add_y(&mut ig, d, y as u32, f);
        while w > 0 {
            let r = ig.r >> 24;
            let g = ig.g >> 24;
            let b = ig.b >> 24;
            let mut dx = x as u32 & 3;
            let mut dy = y as u32 & 3;
            if f.textured {
                let mut t = self.get_texel(f.tm, ig.u >> 24, ig.v >> 24);
                if t != 0 {
                    if f.modulate {
                        if !self.dtd {
                            dx = 3;
                            dy = 2;
                        }
                        t = self.mod_texel(dy, dx, t, r, g, b);
                    }
                    self.plot(x, y, t, f.mode, true);
                }
            } else {
                let pix = if f.gouraud && self.dtd {
                    0x8000
                        | self.dither[dy as usize][r as usize][dx as usize] as u16
                        | (self.dither[dy as usize][g as usize][dx as usize] as u16) << 5
                        | (self.dither[dy as usize][b as usize][dx as usize] as u16) << 10
                } else {
                    (0x8000 | r >> 3 | (g >> 3) << 5 | (b >> 3) << 10) as u16
                };
                self.plot(x, y, pix, f.mode, false);
            }
            x += 1;
            w -= 1;
            add_x(&mut ig, d, 1, f);
        }
    }
}
