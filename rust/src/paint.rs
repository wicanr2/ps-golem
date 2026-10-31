use crate::scene::{Canvas, Image, ImagePixels, Material};

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
    let q = |v: i32| (((v as i64 * 4096) / den as i64) as u32) << 12;
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
impl Painter<'_> {
    fn triangle_checked(&mut self, v: [Vertex; 3], f: Flags) {
        if (v[2].y - v[0].y).abs() >= 512 * self.scale
            || (v[2].y - v[1].y).abs() >= 512 * self.scale
            || (v[1].y - v[0].y).abs() >= 512 * self.scale
        {
            return;
        }
        if (v[2].x - v[0].x).abs() >= 1024 * self.scale
            || (v[2].x - v[1].x).abs() >= 1024 * self.scale
            || (v[1].x - v[0].x).abs() >= 1024 * self.scale
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
            ig.u = ((c.u as u32)
                .wrapping_shl(12)
                .wrapping_add(2048 / self.scale as u32))
            .wrapping_shl(12);
            ig.v = ((c.v as u32)
                .wrapping_shl(12)
                .wrapping_add(2048 / self.scale as u32))
            .wrapping_shl(12);
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
                    let y = yi;
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
                    let y = yi;
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
        let mut adj = start;
        let mut w = bound - start;
        let mut x = start;
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
            let mut t = self.sample(ig.u, ig.v);
            if t != 0 {
                if f.modulate {
                    let colors = [ig.r >> 24, ig.g >> 24, ig.b >> 24];
                    let mut color = t & 0x8000;
                    for (c, shift) in colors.into_iter().zip([0, 5, 10]) {
                        color |= ((((t >> shift) & 31) as u32 * c / 128).min(31) as u16) << shift;
                    }
                    t = color;
                }
                let at = y as usize * self.canvas.width + x as usize;
                self.canvas.words[at] = if f.mode >= 0 && t & 0x8000 != 0 {
                    blend(f.mode, self.canvas.words[at], t)
                } else {
                    t
                };
            }
            x += 1;
            w -= 1;
            add_x(&mut ig, d, 1, f);
        }
    }
}

fn blend(mode: i32, bg: u16, fore: u16) -> u16 {
    let b = bg as u32;
    let f = fore as u32;
    match mode {
        0 => {
            let b = b | 0x8000;
            (((f + b) - ((f ^ b) & 0x0421)) >> 1) as u16
        }
        1 => {
            let b = b & !0x8000;
            let sum = f + b;
            let carry = (sum - ((f ^ b) & 0x8421)) & 0x8420;
            ((sum - carry) | (carry - (carry >> 5))) as u16
        }
        2 => {
            let b = b | 0x8000;
            let f = f & !0x8000;
            let d = b.wrapping_sub(f).wrapping_add(0x108420);
            let borrow = d.wrapping_sub((b ^ f) & 0x108420) & 0x108420;
            ((d - borrow) & (borrow - (borrow >> 5))) as u16
        }
        3 => {
            let b = b & !0x8000;
            let f = (f >> 2) & 0x1ce7 | 0x8000;
            let sum = f + b;
            let carry = (sum - ((f ^ b) & 0x8421)) & 0x8420;
            ((sum - carry) | (carry - (carry >> 5))) as u16
        }
        _ => fore,
    }
}
// Typed direct pixels. Source: own gpu/polygon.rs at b3fbaae and render/hd/poly.go;
// beetle-psx-libretro e57860d82faa953dc0fc10f201fd583739bc2f7d,
// mednafen/psx/gpu_polygon.c,gpu_common.h, GPL-2.0-or-later; project GPL-3.0-or-later.
// No command parsing, Machine, guest instructions or native framebuffer input.
#[derive(Clone, Copy, Debug)]
pub struct Point {
    pub x: i32,
    pub y: i32,
    pub u: i32,
    pub v: i32,
    pub color: [u8; 3],
}
pub struct Draw {
    pub material: usize,
    pub points: [Point; 4],
    pub raw: bool,
    pub blend: i32,
}
struct Painter<'a> {
    canvas: &'a mut Canvas,
    material: &'a Material,
    image: Option<&'a Image>,
    scale: i32,
    clip_x0: i32,
    clip_y0: i32,
    clip_x1: i32,
    clip_y1: i32,
}
impl Painter<'_> {
    fn sample(&self, u: u32, v: u32) -> u16 {
        let x = (u >> 24) as usize;
        let y = (v >> 24) as usize;
        if x >= self.material.width || y >= self.material.height {
            return 0;
        }
        if let Some(im) = self.image {
            let ix =
                x as usize * im.scale + (((u & 0xffffff) as u64 * im.scale as u64) >> 24) as usize;
            let iy =
                y as usize * im.scale + (((v & 0xffffff) as u64 * im.scale as u64) >> 24) as usize;
            match &im.pixels {
                ImagePixels::Indexed(p) => self.material.palette[p[iy * im.width + ix] as usize],
                ImagePixels::Colors(p) => p[iy * im.width + ix],
            }
        } else {
            let at = y as usize * self.material.width + x as usize;
            self.material.palette[((self.material.packed[at / 2] >> ((at & 1) * 4)) & 15) as usize]
        }
    }
}
pub fn draw(
    canvas: &mut Canvas,
    scale: usize,
    materials: &[Material],
    images: &[Option<Image>],
    draws: &[Draw],
) -> Result<(), String> {
    if ![1, 4].contains(&scale)
        || canvas.width == 0
        || canvas.height == 0
        || canvas.width > 4096
        || canvas.height > 2048
        || canvas.words.len() != canvas.width * canvas.height
        || materials.len() != images.len()
        || materials.len() > 512
        || draws.len() > 768
    {
        return Err("人物畫布形狀或預算不符".into());
    }
    for (m, image) in materials.iter().zip(images) {
        if m.width == 0
            || m.width > 256
            || m.width % 2 != 0
            || m.height == 0
            || m.height > 256
            || m.packed.len() != m.width * m.height / 2
        {
            return Err("人物原材質形狀不符".into());
        }
        if let Some(im) = image {
            if ![1, 2, 4, 8].contains(&im.scale)
                || im.width != m.width * im.scale
                || im.height != m.height * im.scale
            {
                return Err("人物HD形狀不符".into());
            }
            let n = im.width * im.height;
            match &im.pixels {
                ImagePixels::Indexed(v) if v.len() == n && v.iter().all(|&x| x < 16) => (),
                ImagePixels::Colors(v) if v.len() == n => (),
                _ => return Err("人物HD圖素不符".into()),
            }
        }
    }
    for d in draws {
        if d.material >= materials.len()
            || !(-1..=3).contains(&d.blend)
            || d.points.iter().any(|p| {
                !(-4096..=4096).contains(&p.x)
                    || !(-4096..=4096).contains(&p.y)
                    || !(-256..=512).contains(&p.u)
                    || !(-256..=512).contains(&p.v)
            })
        {
            return Err("人物圖元來源超界".into());
        }
    }
    for d in draws {
        let w = canvas.width;
        let h = canvas.height;
        let mut p = Painter {
            canvas,
            material: &materials[d.material],
            image: if scale == 1 {
                None
            } else {
                images[d.material].as_ref()
            },
            scale: scale as i32,
            clip_x0: 0,
            clip_y0: 0,
            clip_x1: w as i32 - 1,
            clip_y1: h as i32 - 1,
        };
        let vs = d.points.map(|v| Vertex {
            x: v.x * scale as i32,
            y: v.y * scale as i32,
            u: v.u,
            v: v.v,
            r: v.color[0] as i32,
            g: v.color[1] as i32,
            b: v.color[2] as i32,
        });
        let f = Flags {
            gouraud: true,
            textured: true,
            modulate: !d.raw,
            mode: d.blend,
            tm: 0,
        };
        p.triangle_checked([vs[0], vs[1], vs[2]], f);
        p.triangle_checked([vs[1], vs[2], vs[3]], f);
    }
    Ok(())
}

#[cfg(test)]
#[path = "paint_tests.rs"]
mod tests;
