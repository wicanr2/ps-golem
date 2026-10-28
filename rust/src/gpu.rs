//! Native PS1 command renderer, 026 §2. Pure Rust, no Go/C++ renderer calls.
//! Port of ps-golem internal/gpu at f3ef194; its raster algorithms reference
//! beetle-psx-libretro e57860d82faa953dc0fc10f201fd583739bc2f7d,
//! mednafen/psx/gpu*.c and gpu_common.h, GPL-2.0-or-later.
//! Project license: GPL-3.0-or-later. Video scheduler and HD are separate.
mod polygon;
mod primitives;
mod video;
pub use video::Frame;
mod raster;
#[cfg(test)]
mod tests;

pub const WIDTH: usize = 1024;
pub const HEIGHT: usize = 512;
const NONE: u8 = 0;
const FB_WRITE: u8 = 1;
const FB_READ: u8 = 2;
const POLYLINE: u8 = 3;

#[derive(Clone, Copy, Default)]
struct TextureLine {
    tag: u32,
    data: [u16; 4],
}
#[derive(Clone, Copy, Default)]
struct Point {
    x: i32,
    y: i32,
    r: u32,
    g: u32,
    b: u32,
}

/// Detached, owned raster state. No game-specific addresses or callbacks.
#[derive(Clone)]
pub struct Gpu {
    pixels: Vec<u16>,
    buf: [u32; 16],
    len: usize,
    need: usize,
    in_cmd: u8,
    fb_x: u32,
    fb_y: u32,
    fb_w: u32,
    fb_h: u32,
    cur_x: u32,
    cur_y: u32,
    pline_cmd: u32,
    pline_prev: Point,
    pline_group: [u32; 2],
    pline_n: usize,
    page_x: u32,
    page_y: u32,
    tex_mode: u32,
    abr: u32,
    sprite_flip: u32,
    dtd: bool,
    dfe: bool,
    tww: u32,
    twh: u32,
    twx: u32,
    twy: u32,
    twx_and: u32,
    twx_add: u32,
    twy_and: u32,
    twy_add: u32,
    clip_x0: i32,
    clip_y0: i32,
    clip_x1: i32,
    clip_y1: i32,
    off_x: i32,
    off_y: i32,
    mask_or: u16,
    mask_and: u16,
    tex_disable: bool,
    tex_allow: bool,
    clut: [u16; 256],
    clut_address: u32,
    clut_count: u32,
    texture: [TextureLine; 256],
    dither: [[[u8; 4]; 512]; 4],
    display_off: bool,
    dma: u32,
    display_x: u32,
    display_y: u32,
    horiz_start: u32,
    horiz_end: u32,
    vert_start: u32,
    vert_end: u32,
    display_mode: u32,
    irq: bool,
    read_buffer: u32,
    field_readout: bool,
    video: video::VideoTiming,
}

fn sx11(v: u32) -> i32 {
    (v.wrapping_shl(21) as i32) >> 21
}

impl Default for Gpu {
    fn default() -> Self {
        Self::new()
    }
}
impl Gpu {
    pub fn new() -> Self {
        let mut g = Self {
            pixels: vec![0; WIDTH * HEIGHT],
            buf: [0; 16],
            len: 0,
            need: 0,
            in_cmd: NONE,
            fb_x: 0,
            fb_y: 0,
            fb_w: 0,
            fb_h: 0,
            cur_x: 0,
            cur_y: 0,
            pline_cmd: 0,
            pline_prev: Point::default(),
            pline_group: [0; 2],
            pline_n: 0,
            page_x: 0,
            page_y: 0,
            tex_mode: 0,
            abr: 0,
            sprite_flip: 0,
            dtd: false,
            dfe: false,
            tww: 0,
            twh: 0,
            twx: 0,
            twy: 0,
            twx_and: 0,
            twx_add: 0,
            twy_and: 0,
            twy_add: 0,
            clip_x0: 0,
            clip_y0: 0,
            clip_x1: 0,
            clip_y1: 0,
            off_x: 0,
            off_y: 0,
            mask_or: 0,
            mask_and: 0,
            tex_disable: false,
            tex_allow: false,
            clut: [0; 256],
            clut_address: u32::MAX,
            clut_count: 0,
            texture: [TextureLine {
                tag: u32::MAX,
                data: [0; 4],
            }; 256],
            dither: [[[0; 4]; 512]; 4],
            display_off: true,
            dma: 0,
            display_x: 0,
            display_y: 0,
            horiz_start: 0,
            horiz_end: 0,
            vert_start: 0,
            vert_end: 0,
            display_mode: 0,
            irq: false,
            read_buffer: 0,
            field_readout: false,
            video: video::VideoTiming::default(),
        };
        let table = [
            [-4, 0, -3, 1],
            [2, -2, 3, -1],
            [-3, 1, -4, 0],
            [3, -1, 2, -2],
        ];
        for y in 0..4 {
            for x in 0..4 {
                for v in 0..512 {
                    g.dither[y][v][x] = (((v as i32 + table[y][x]) >> 3).clamp(0, 31)) as u8;
                }
            }
        }
        g.soft_reset();
        g
    }
    fn soft_reset(&mut self) {
        self.irq = false;
        self.invalidate_cache();
        self.dma = 0;
        self.len = 0;
        self.need = 0;
        self.in_cmd = NONE;
        self.display_off = true;
        self.display_x = 0;
        self.display_y = 0;
        self.display_mode = 0;
        self.horiz_start = 0x200;
        self.horiz_end = 0xc00;
        self.vert_start = 0x10;
        self.vert_end = 0x100;
        self.page_x = 0;
        self.page_y = 0;
        self.sprite_flip = 0;
        self.abr = 0;
        self.tex_mode = 0;
        self.dtd = false;
        self.dfe = false;
        self.tww = 0;
        self.twh = 0;
        self.twx = 0;
        self.twy = 0;
        self.recalc_window();
        self.clip_x0 = 0;
        self.clip_y0 = 0;
        self.clip_x1 = 0;
        self.clip_y1 = 0;
        self.off_x = 0;
        self.off_y = 0;
        self.mask_or = 0;
        self.mask_and = 0;
        self.tex_disable = false;
        self.tex_allow = false;
    }
    fn invalidate_texture(&mut self) {
        for line in &mut self.texture {
            line.tag = u32::MAX;
        }
    }
    fn invalidate_cache(&mut self) {
        self.clut_address = u32::MAX;
        self.clut_count = 0;
        self.invalidate_texture();
    }
    pub fn vram(&self) -> &[u16] {
        &self.pixels
    }
    pub fn set_field(&mut self, field: bool) {
        self.field_readout = field;
    }
    pub fn gp1(&mut self, word: u32) {
        let cmd = word >> 24;
        let v = word & 0xffffff;
        match cmd {
            0 => self.soft_reset(),
            1 => {
                self.len = 0;
                self.need = 0;
                self.in_cmd = NONE;
            }
            2 => self.irq = false,
            3 => self.display_off = v & 1 != 0,
            4 => self.dma = v & 3,
            5 => {
                self.display_x = v & 0x3fe;
                self.display_y = (v >> 10) & 511;
            }
            6 => {
                self.horiz_start = v & 4095;
                self.horiz_end = (v >> 12) & 4095;
            }
            7 => {
                self.vert_start = v & 1023;
                self.vert_end = (v >> 10) & 1023;
            }
            8 => self.display_mode = v & 255,
            9 => self.tex_allow = v & 1 != 0,
            0x10 => match v & 15 {
                2 => {
                    self.read_buffer = (self.read_buffer & 0xfff00000)
                        | self.tww
                        | (self.twh << 5)
                        | (self.twx << 10)
                        | (self.twy << 15)
                }
                3 => {
                    self.read_buffer = (self.read_buffer & 0xfff00000)
                        | (self.clip_y0 as u32) << 10
                        | self.clip_x0 as u32
                }
                4 => {
                    self.read_buffer = (self.read_buffer & 0xfff00000)
                        | (self.clip_y1 as u32) << 10
                        | self.clip_x1 as u32
                }
                5 => {
                    self.read_buffer = (self.read_buffer & 0xffc00000)
                        | (self.off_x as u32 & 2047)
                        | (self.off_y as u32 & 2047) << 11
                }
                7 => self.read_buffer = 2,
                8 => self.read_buffer = 0,
                _ => (),
            },
            _ => (),
        }
    }
    pub fn status(&self) -> u32 {
        let mut r = (((self.display_mode << 1) & 0x7f) | ((self.display_mode >> 6) & 1)) << 16;
        r |= (self.display_mode & 128) << 7 | self.dma << 29;
        r |= (self.video.read_line & 1) << 31;
        if !self.video.field {
            r |= 1 << 13;
        }
        if self.dma & 2 != 0 {
            r |= 1 << 25;
        }
        if self.irq {
            r |= 1 << 24;
        }
        if self.display_off {
            r |= 1 << 23;
        }
        if self.in_cmd == NONE && self.len == 0 {
            r |= 1 << 26;
        }
        if self.in_cmd == FB_READ {
            r |= 1 << 27;
        }
        if self.in_cmd != POLYLINE
            && (self.len == 0 || !(self.in_cmd == FB_READ || self.in_cmd == FB_WRITE))
        {
            r |= 1 << 28;
        }
        r |= self.page_x >> 6 | self.page_y >> 4 | self.abr << 5 | self.tex_mode << 7;
        if self.dtd {
            r |= 1 << 9;
        }
        if self.dfe {
            r |= 1 << 10;
        }
        if self.mask_or != 0 {
            r |= 1 << 11;
        }
        if self.mask_and != 0 {
            r |= 1 << 12;
        }
        if self.tex_disable {
            r |= 1 << 15;
        }
        r
    }
    pub fn read_data(&mut self) -> u32 {
        if self.in_cmd != FB_READ {
            return self.read_buffer;
        }
        let mut r = 0;
        for i in 0..2 {
            r |= (self.texel(self.cur_x, self.cur_y) as u32) << (16 * i);
            self.cur_x += 1;
            if self.cur_x == self.fb_x + self.fb_w {
                if self.cur_y + 1 == self.fb_y + self.fb_h {
                    self.in_cmd = NONE;
                } else {
                    self.cur_y += 1;
                    self.cur_x = self.fb_x;
                }
            }
        }
        self.read_buffer = r;
        r
    }
    /// Presentation traces omit CPU reads; use the real read port to finish them.
    pub fn drain_readback(&mut self) -> usize {
        let mut n = 0;
        while self.in_cmd == FB_READ {
            self.read_data();
            n += 1;
            assert!(n <= WIDTH * HEIGHT);
        }
        n
    }
    pub fn gp0(&mut self, v: u32) {
        if self.in_cmd == FB_WRITE {
            self.upload_word(v);
            return;
        }
        if self.in_cmd == POLYLINE {
            self.polyline_word(v);
            return;
        }
        if self.len == 0 {
            self.need = command_len(v >> 24);
        }
        self.buf[self.len] = v;
        self.len += 1;
        if self.len < self.need {
            return;
        }
        let cb = self.buf;
        let n = self.len;
        self.len = 0;
        self.need = 0;
        self.execute(&cb[..n]);
    }
    fn execute(&mut self, cb: &[u32]) {
        let cmd = cb[0] >> 24;
        match cmd {
            1 => self.invalidate_cache(),
            2 => self.fill(cb),
            0x1f => self.irq = true,
            0x20..=0x3f => self.draw_polygon(cb),
            0x40..=0x5f => self.draw_line_command(cb),
            0x60..=0x7f => self.draw_sprite_command(cb),
            0x80..=0x9f => self.copy(cb),
            0xa0..=0xbf => self.upload_start(cb),
            0xc0..=0xdf => self.read_start(cb),
            0xe1 => {
                self.set_page(cb[0]);
                self.sprite_flip = cb[0] & 0x3000;
                self.dtd = cb[0] >> 9 & 1 != 0;
                self.dfe = cb[0] >> 10 & 1 != 0;
            }
            0xe2 => {
                self.tww = cb[0] & 31;
                self.twh = cb[0] >> 5 & 31;
                self.twx = cb[0] >> 10 & 31;
                self.twy = cb[0] >> 15 & 31;
                self.recalc_window();
            }
            0xe3 => {
                self.clip_x0 = (cb[0] & 1023) as i32;
                self.clip_y0 = (cb[0] >> 10 & 1023) as i32;
            }
            0xe4 => {
                self.clip_x1 = (cb[0] & 1023) as i32;
                self.clip_y1 = (cb[0] >> 10 & 1023) as i32;
            }
            0xe5 => {
                self.off_x = sx11(cb[0] & 2047);
                self.off_y = sx11(cb[0] >> 11 & 2047);
            }
            0xe6 => {
                self.mask_or = if cb[0] & 1 != 0 { 0x8000 } else { 0 };
                self.mask_and = if cb[0] & 2 != 0 { 0x8000 } else { 0 };
            }
            _ => (),
        }
    }
    fn set_page(&mut self, w: u32) {
        let x = (w & 15) * 64;
        let y = (w & 16) * 16;
        let mode = w >> 7 & 3;
        self.abr = w >> 5 & 3;
        if (mode == 0) != (self.tex_mode == 0) || x != self.page_x || y != self.page_y {
            self.invalidate_texture();
        }
        let disable = self.tex_allow && w >> 11 & 1 != 0;
        if disable != self.tex_disable {
            self.invalidate_texture();
        }
        self.tex_disable = disable;
        self.page_x = x;
        self.page_y = y;
        self.tex_mode = mode;
        self.recalc_window();
    }
    fn recalc_window(&mut self) {
        let tm = self.tex_mode.min(2);
        self.twx_and = !(self.tww << 3);
        self.twy_and = !(self.twh << 3);
        self.twx_add = ((self.twx & self.tww) << 3) + (self.page_x << (2 - tm));
        self.twy_add = ((self.twy & self.twh) << 3) + self.page_y;
    }
    fn texel(&self, x: u32, y: u32) -> u16 {
        self.pixels[(((y & 511) << 10) | (x & 1023)) as usize]
    }
    fn put(&mut self, x: u32, y: u32, v: u16) {
        self.pixels[(((y & 511) << 10) | (x & 1023)) as usize] = v;
    }
    fn line_skip(&self, y: u32) -> bool {
        self.display_mode & 0x24 == 0x24
            && !self.dfe
            && (y & 1) == ((self.display_y + self.field_readout as u32) & 1)
    }
    fn fill(&mut self, cb: &[u32]) {
        let c = cb[0];
        let fill =
            ((c & 255) >> 3 | ((c >> 8 & 255) >> 3) << 5 | ((c >> 16 & 255) >> 3) << 10) as u16;
        let x = cb[1] & 0x3f0;
        let y = cb[1] >> 16 & 1023;
        let w = ((cb[2] & 1023) + 15) & !15;
        let h = cb[2] >> 16 & 511;
        for dy in 0..h {
            let yy = (y + dy) & 511;
            if self.line_skip(yy) {
                continue;
            }
            for dx in 0..w {
                self.put(x + dx, yy, fill);
            }
        }
    }
    fn copy(&mut self, cb: &[u32]) {
        let sx = cb[1] & 1023;
        let sy = cb[1] >> 16 & 1023;
        let dx = cb[2] & 1023;
        let dy = cb[2] >> 16 & 1023;
        let mut w = cb[3] & 1023;
        let mut h = cb[3] >> 16 & 511;
        if w == 0 {
            w = 1024;
        }
        if h == 0 {
            h = 512;
        }
        self.invalidate_texture();
        let mut temp = [0; 128];
        for y in 0..h {
            let mut x = 0;
            while x < w {
                let n = (w - x).min(128);
                for i in 0..n {
                    temp[i as usize] = self.texel(sx + x + i, sy + y);
                }
                for i in 0..n {
                    if self.texel(dx + x + i, dy + y) & self.mask_and == 0 {
                        self.put(dx + x + i, dy + y, temp[i as usize] | self.mask_or);
                    }
                }
                x += 128;
            }
        }
    }
    fn upload_start(&mut self, cb: &[u32]) {
        self.fb_x = cb[1] & 1023;
        self.fb_y = cb[1] >> 16 & 1023;
        self.fb_w = cb[2] & 1023;
        self.fb_h = cb[2] >> 16 & 511;
        if self.fb_w == 0 {
            self.fb_w = 1024;
        }
        if self.fb_h == 0 {
            self.fb_h = 512;
        }
        self.cur_x = self.fb_x;
        self.cur_y = self.fb_y;
        self.invalidate_texture();
        self.in_cmd = FB_WRITE;
    }
    fn upload_word(&mut self, mut v: u32) {
        for _ in 0..2 {
            if self.texel(self.cur_x, self.cur_y) & self.mask_and == 0 {
                self.put(self.cur_x, self.cur_y, v as u16 | self.mask_or);
            }
            self.cur_x += 1;
            if self.cur_x == self.fb_x + self.fb_w {
                self.cur_x = self.fb_x;
                self.cur_y += 1;
                if self.cur_y == self.fb_y + self.fb_h {
                    self.in_cmd = NONE;
                    return;
                }
            }
            v >>= 16;
        }
    }
    fn read_start(&mut self, cb: &[u32]) {
        self.fb_x = cb[1] & 1023;
        self.fb_y = cb[1] >> 16 & 1023;
        self.fb_w = cb[2] & 1023;
        self.fb_h = cb[2] >> 16 & 1023;
        if self.fb_w == 0 {
            self.fb_w = 1024;
        }
        if self.fb_h > 512 {
            self.fb_h &= 511;
        }
        self.cur_x = self.fb_x;
        self.cur_y = self.fb_y;
        self.invalidate_texture();
        if self.fb_w != 0 && self.fb_h != 0 {
            self.in_cmd = FB_READ;
        }
    }
}
fn command_len(cmd: u32) -> usize {
    match cmd {
        2 => 3,
        0x20..=0x3f => {
            let nv = if cmd & 8 != 0 { 4 } else { 3 };
            let per = 1 + (cmd & 4 != 0) as usize + (cmd & 16 != 0) as usize;
            1 + nv * per - (cmd & 16 != 0) as usize
        }
        0x40..=0x5f => {
            if cmd & 16 != 0 {
                4
            } else {
                3
            }
        }
        0x60..=0x7f => 2 + (cmd & 4 != 0) as usize + (cmd & 24 == 0) as usize,
        0x80..=0x9f => 4,
        0xa0..=0xdf => 3,
        _ => 1,
    }
}
