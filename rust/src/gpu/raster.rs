use super::*;

pub(super) fn blend(mode: i32, bg: u16, fore: u16) -> u16 {
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
impl Gpu {
    pub(super) fn blend_mode(&self, cmd: u32) -> i32 {
        if cmd & 2 != 0 {
            self.abr as i32
        } else {
            -1
        }
    }
    pub(super) fn plot(&mut self, x: i32, y: i32, mut fore: u16, mode: i32, textured: bool) {
        let at = ((((y as u32) & 511) << 10) | ((x as u32) & 1023)) as usize;
        let bg = self.pixels[at];
        if mode >= 0 && fore & 0x8000 != 0 {
            fore = blend(mode, bg, fore);
        }
        if self.mask_and != 0 && bg & 0x8000 != 0 {
            return;
        }
        if !textured {
            fore &= 0x7fff;
        }
        self.pixels[at] = fore | self.mask_or;
    }
    pub(super) fn update_clut(&mut self, raw: u32, tm: u32) {
        if tm >= 2 {
            return;
        }
        let n = if tm == 1 { 256 } else { 16 };
        let addr = raw & 0x7fff;
        if self.clut_address == addr && self.clut_count >= n {
            return;
        }
        let y = raw >> 6 & 511;
        let x = (raw & 63) << 4;
        for i in 0..n {
            self.clut[i as usize] = self.texel(x + i, y);
        }
        self.clut_address = addr;
        self.clut_count = n;
    }
    pub(super) fn get_texel(&mut self, tm: u32, u: u32, v: u32) -> u16 {
        let tm = tm.min(2);
        let ue = (u & self.twx_and) + self.twx_add;
        let fx = (ue >> (2 - tm)) & 1023;
        let fy = (v & self.twy_and) + self.twy_add;
        let gro = fy * 1024 + fx;
        let tag = gro & !3;
        let slot = if tm == 0 {
            (gro >> 2) & 3 | (gro >> 8) & 0xfc
        } else {
            (gro >> 2) & 7 | (gro >> 7) & 0xf8
        } as usize;
        if self.texture[slot].tag != tag {
            for i in 0..4 {
                self.texture[slot].data[i] = self.texel((fx & !3) + i as u32, fy);
            }
            self.texture[slot].tag = tag;
        }
        let w = self.texture[slot].data[(gro & 3) as usize];
        match tm {
            0 => self.clut[((w >> ((ue & 3) * 4)) & 15) as usize],
            1 => self.clut[((w >> ((ue & 1) * 8)) & 255) as usize],
            _ => w,
        }
    }
    pub(super) fn mod_texel(&self, y: u32, x: u32, t: u16, r: u32, g: u32, b: u32) -> u16 {
        let ir = ((t as u32 & 31) * r) >> 4;
        let ig = ((t as u32 & 0x3e0) * g) >> 9;
        let ib = ((t as u32 & 0x7c00) * b) >> 14;
        t & 0x8000
            | self.dither[y as usize][ir as usize][x as usize] as u16
            | (self.dither[y as usize][ig as usize][x as usize] as u16) << 5
            | (self.dither[y as usize][ib as usize][x as usize] as u16) << 10
    }
}
