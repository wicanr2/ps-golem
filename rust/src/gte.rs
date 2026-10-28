//! Native GTE, spec 003 and 026 §5. GPL-3.0-or-later.
//! Port of ps-golem internal/gte/gte.go at 4951d1e26d2fdde57d92c37ee91129616a5ec471.
//! Original reference: beetle-psx-libretro mednafen/psx/gte.c,
//! e57860d82faa953dc0fc10f201fd583739bc2f7d, GPL-2.0-or-later.
//! Architectural integer state only; presentation observers are attached later.
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct Gte {
    dr: [u32; 32],
    cr: [u32; 32],
    flags: u32,
}
#[derive(Clone, Copy)]
struct Fields {
    sf: u32,
    mx: usize,
    v: usize,
    cv: usize,
    lm: bool,
}
const ERROR_MASK: u32 = 0x7f87e000;
const CR_MASK: [u32; 32] = [
    !0, !0, !0, !0, 0xffff, !0, !0, !0, !0, !0, !0, !0, 0xffff, !0, !0, !0, !0, !0, !0, !0, 0xffff,
    !0, !0, !0, !0, !0, 0xffff, 0xffff, !0, 0xffff, 0xffff, !0,
];
const fn unr_table() -> [u8; 257] {
    let mut table = [0; 257];
    let mut divisor = 0x8000u32;
    while divisor < 0x10000 {
        let mut xa = 512u32;
        let mut i = 1;
        while i < 5 {
            xa = xa.wrapping_mul((1024u32 * 512).wrapping_sub((divisor >> 7) * xa)) >> 18;
            i += 1;
        }
        table[((divisor >> 7) & 255) as usize] = ((xa + 1) >> 1).wrapping_sub(0x101) as u8;
        divisor += 0x80;
    }
    table[256] = table[255];
    table
}
const UNR: [u8; 257] = unr_table();
fn lo(v: u32) -> i32 {
    v as i16 as i32
}
fn hi(v: u32) -> i32 {
    (v >> 16) as i16 as i32
}
fn sign44(v: i64) -> i64 {
    v.wrapping_shl(20) >> 20
}
impl Gte {
    pub fn new() -> Self {
        Self::default()
    }
    fn ir(&self, i: usize) -> i32 {
        lo(self.dr[8 + i])
    }
    fn set_ir(&mut self, i: usize, v: i32) {
        self.dr[8 + i] = v as i16 as i32 as u32;
    }
    fn mac(&self, i: usize) -> i32 {
        self.dr[24 + i] as i32
    }
    fn set_mac(&mut self, i: usize, v: i32) {
        self.dr[24 + i] = v as u32;
    }
    fn z(&self, i: usize) -> u32 {
        self.dr[16 + i] as u16 as u32
    }
    fn rgb(&self, i: usize) -> i32 {
        (self.dr[6] >> (8 * i)) as u8 as i32
    }
    fn vector(&self, v: usize, i: usize) -> i32 {
        match i {
            0 => lo(self.dr[v * 2]),
            1 => hi(self.dr[v * 2]),
            _ => lo(self.dr[v * 2 + 1]),
        }
    }
    pub fn write(&mut self, reg: usize, mut v: u32) {
        assert!(reg < 64);
        if reg >= 32 {
            let i = reg - 32;
            let mask = CR_MASK[i];
            v &= mask;
            self.cr[i] = v | (self.cr[i] & !mask);
            if i == 31 {
                self.cr[31] = v & 0x7ffff000;
                if v & ERROR_MASK != 0 {
                    self.cr[31] |= 1 << 31;
                }
            }
        } else {
            match reg {
                1 | 3 | 5 => self.dr[reg] = lo(v) as u32,
                7 | 16..=19 => self.dr[reg] = v as u16 as u32,
                8..=11 => self.set_ir(reg - 8, v as i32),
                14 => {
                    self.dr[14] = v;
                    self.dr[15] = v;
                }
                15 => {
                    self.dr[15] = v;
                    self.dr[12] = self.dr[13];
                    self.dr[13] = self.dr[14];
                    self.dr[14] = v;
                }
                28 => {
                    for i in 0..3 {
                        self.set_ir(1 + i, (((v >> (5 * i)) & 31) as i32) << 7);
                    }
                }
                29 | 31 => {}
                30 => {
                    self.dr[30] = v;
                    self.dr[31] = (v ^ 0u32.wrapping_sub(v >> 31)).leading_zeros();
                }
                _ => self.dr[reg] = v,
            }
        }
    }
    pub fn read(&self, reg: usize) -> u32 {
        assert!(reg < 64);
        if reg >= 32 {
            let i = reg - 32;
            return match i {
                4 | 12 | 20 | 26 | 27 | 29 | 30 => lo(self.cr[i]) as u32,
                _ => self.cr[i],
            };
        }
        match reg {
            1 | 3 | 5 | 8..=11 => lo(self.dr[reg]) as u32,
            7 | 16..=19 => self.dr[reg] as u16 as u32,
            28 | 29 => {
                (self.ir(1) >> 7).clamp(0, 31) as u32
                    | ((self.ir(2) >> 7).clamp(0, 31) as u32) << 5
                    | ((self.ir(3) >> 7).clamp(0, 31) as u32) << 10
            }
            _ => self.dr[reg],
        }
    }
    fn a_mv(&mut self, i: usize, v: i64) -> i64 {
        if v >= 1 << 43 {
            self.flags |= 1 << (30 - i);
        }
        if v < -(1 << 43) {
            self.flags |= 1 << (27 - i);
        }
        sign44(v)
    }
    fn i44(&mut self, i: usize, v: i64) -> i64 {
        if v >= 0x7ffffffffff {
            self.flags |= 1 << (30 - i);
        }
        if v < -0x80000000000 {
            self.flags |= 1 << (27 - i);
        }
        sign44(v)
    }
    fn f(&mut self, v: i64) -> i64 {
        if v < i32::MIN as i64 {
            self.flags |= 1 << 15;
        }
        if v > i32::MAX as i64 {
            self.flags |= 1 << 16;
        }
        v
    }
    fn lim_b(&mut self, i: usize, v: i32, lm: bool) -> i32 {
        let low = if lm { 0 } else { -32768 };
        if v < low || v > 32767 {
            self.flags |= 1 << (24 - i);
        }
        v.clamp(low, 32767)
    }
    fn lim_b_ptz(&mut self, i: usize, v: i32, ftv: i32, lm: bool) -> i32 {
        if !(-32768..=32767).contains(&ftv) {
            self.flags |= 1 << (24 - i);
        }
        v.clamp(if lm { 0 } else { -32768 }, 32767)
    }
    fn lim_c(&mut self, i: usize, v: i32) -> u32 {
        if v & !255 != 0 {
            self.flags |= 1 << (21 - i);
        }
        v.clamp(0, 255) as u32
    }
    fn lim_d(&mut self, v: i32, unchained: bool) -> i32 {
        if !unchained {
            if self.flags & (1 << 15) != 0 {
                self.flags |= 1 << 18;
                return 0;
            }
            if self.flags & (1 << 16) != 0 {
                self.flags |= 1 << 18;
                return 65535;
            }
        }
        if !(0..=65535).contains(&v) {
            self.flags |= 1 << 18;
        }
        v.clamp(0, 65535)
    }
    fn lim_g(&mut self, i: usize, v: i32) -> i32 {
        if !(-1024..=1023).contains(&v) {
            self.flags |= 1 << (14 - i);
        }
        v.clamp(-1024, 1023)
    }
    fn lim_h(&mut self, v: i32) -> i32 {
        if !(0..=4096).contains(&v) {
            self.flags |= 1 << 12;
        }
        v.clamp(0, 4096)
    }
    fn mac_to_ir(&mut self, lm: bool) {
        for i in 1..4 {
            let v = self.lim_b(i - 1, self.mac(i), lm);
            self.set_ir(i, v);
        }
    }
    fn mac_to_rgb(&mut self) {
        let packed = self.lim_c(0, self.mac(1) >> 4)
            | (self.lim_c(1, self.mac(2) >> 4) << 8)
            | (self.lim_c(2, self.mac(3) >> 4) << 16)
            | ((self.rgb(3) as u32) << 24);
        self.dr[20] = self.dr[21];
        self.dr[21] = self.dr[22];
        self.dr[22] = packed;
    }
    fn matrix_row(&self, mx: usize, row: usize) -> [i32; 3] {
        if mx == 3 {
            if row == 0 {
                return [-(self.rgb(0) << 4), self.rgb(0) << 4, self.ir(0)];
            }
            let c = lo(self.cr[row]);
            return [c, c, c];
        }
        let m = mx * 8;
        match row {
            0 => [lo(self.cr[m]), hi(self.cr[m]), lo(self.cr[m + 1])],
            1 => [hi(self.cr[m + 1]), lo(self.cr[m + 2]), hi(self.cr[m + 2])],
            _ => [lo(self.cr[m + 3]), hi(self.cr[m + 3]), lo(self.cr[m + 4])],
        }
    }
    fn translation(&self, cv: usize, i: usize) -> i64 {
        if cv == 3 {
            0
        } else {
            (self.cr[cv * 8 + 5 + i] as i32 as i64) << 12
        }
    }
    fn multiply(&mut self, mx: usize, v: usize, cv: usize, sf: u32, lm: bool) {
        let vec = if v == 3 {
            [self.ir(1), self.ir(2), self.ir(3)]
        } else {
            [self.vector(v, 0), self.vector(v, 1), self.vector(v, 2)]
        };
        for i in 0..3 {
            let row = self.matrix_row(mx, i);
            let mul = [
                row[0].wrapping_mul(vec[0]),
                row[1].wrapping_mul(vec[1]),
                row[2].wrapping_mul(vec[2]),
            ];
            let mut tmp = self.a_mv(i, self.translation(cv, i) + mul[0] as i64);
            if cv == 2 {
                self.lim_b(i, (tmp >> sf) as i32, false);
                tmp = 0;
            }
            tmp = self.a_mv(i, tmp + mul[1] as i64);
            tmp = self.a_mv(i, tmp + mul[2] as i64);
            self.set_mac(1 + i, (tmp >> sf) as i32);
        }
        self.mac_to_ir(lm);
    }
    fn multiply_pt(&mut self, v: usize, sf: u32, lm: bool) {
        let mut tmp = [0i64; 3];
        for i in 0..3 {
            let row = self.matrix_row(0, i);
            tmp[i] = self.translation(0, i);
            for j in 0..3 {
                tmp[i] = self.a_mv(i, tmp[i] + row[j].wrapping_mul(self.vector(v, j)) as i64);
            }
            self.set_mac(1 + i, (tmp[i] >> sf) as i32);
        }
        for i in 1..3 {
            let value = self.lim_b(i - 1, self.mac(i), lm);
            self.set_ir(i, value);
        }
        let value = self.lim_b_ptz(2, self.mac(3), (tmp[2] >> 12) as i32, lm);
        self.set_ir(3, value);
        self.dr[16] = self.dr[17];
        self.dr[17] = self.dr[18];
        self.dr[18] = self.dr[19];
        self.dr[19] = self.lim_d((tmp[2] >> 12) as i32, true) as u16 as u32;
    }
    fn divide(&mut self, mut dividend: u32, mut divisor: u32) -> u32 {
        if divisor * 2 > dividend {
            let shift = (divisor as u16).leading_zeros();
            dividend <<= shift;
            divisor <<= shift;
            let d = (divisor | 0x8000) as u16;
            let x = 0x101 + UNR[(((d as u32 & 0x7fff) + 0x40) >> 7) as usize] as i32;
            let tmp = (-(d as i32) * x + 0x80) >> 8;
            let reciprocal = (x * (131072 + tmp) + 0x80) >> 8;
            return (((dividend as u64) * (reciprocal as i64 as u64) + 32768) >> 16).min(0x1ffff)
                as u32;
        }
        self.flags |= 1 << 17;
        0x1ffff
    }
    fn rtp(&mut self, v: usize, f: Fields, dq: bool) {
        self.multiply_pt(v, f.sf, f.lm);
        let h = self.divide(self.cr[26] as u16 as u32, self.z(3)) as i64;
        let value = (self.f(self.cr[24] as i32 as i64 + self.ir(1) as i64 * h) >> 16) as i32;
        self.set_mac(0, value);
        let x = self.lim_g(0, self.mac(0));
        let value = (self.f(self.cr[25] as i32 as i64 + self.ir(2) as i64 * h) >> 16) as i32;
        self.set_mac(0, value);
        let y = self.lim_g(1, self.mac(0));
        self.dr[15] = x as u16 as u32 | ((y as u16 as u32) << 16);
        self.dr[12] = self.dr[13];
        self.dr[13] = self.dr[14];
        self.dr[14] = self.dr[15];
        if dq {
            let result = self.cr[28] as i32 as i64 + lo(self.cr[27]) as i64 * h;
            let value = self.f(result) as i32;
            self.set_mac(0, value);
            let value = self.lim_h((result >> 12) as i32);
            self.set_ir(0, value);
        }
    }
    fn nclip(&mut self) {
        let [a, b, c] = [self.dr[12], self.dr[13], self.dr[14]];
        let sum = lo(a) as i64 * (hi(b) - hi(c)) as i64
            + lo(b) as i64 * (hi(c) - hi(a)) as i64
            + lo(c) as i64 * (hi(a) - hi(b)) as i64;
        let value = self.f(sum) as i32;
        self.set_mac(0, value);
    }
    fn depth_cue(&mut self, base: [i64; 3], f: Fields) {
        for i in 0..3 {
            let fc = (self.cr[21 + i] as i32 as i64) << 12;
            let value = (self.i44(i, fc - base[i]) >> f.sf) as i32;
            self.set_mac(1 + i, value);
            let diff = self.lim_b(i, self.mac(1 + i), false);
            let term = self.ir(0).wrapping_mul(diff) as i64;
            let value = (self.i44(i, base[i] + term) >> f.sf) as i32;
            self.set_mac(1 + i, value);
        }
        self.mac_to_ir(f.lm);
        self.mac_to_rgb();
    }
    fn dpcs(&mut self, f: Fields, fifo: bool) {
        let color = self.dr[if fifo { 20 } else { 6 }];
        let base = std::array::from_fn(|i| ((((color >> (8 * i)) as u8 as i32) << 4) as i64) << 12);
        self.depth_cue(base, f);
    }
    fn dcpl(&mut self, f: Fields) {
        let base = std::array::from_fn(|i| (self.rgb(i) << 4).wrapping_mul(self.ir(1 + i)) as i64);
        self.depth_cue(base, f);
    }
    fn intpl(&mut self, f: Fields) {
        let base = std::array::from_fn(|i| (self.ir(1 + i) as i64) << 12);
        self.depth_cue(base, f);
    }
    fn color_times_ir(&mut self, f: Fields) {
        for i in 1..4 {
            self.set_mac(i, (self.rgb(i - 1) << 4).wrapping_mul(self.ir(i)) >> f.sf);
        }
        self.mac_to_ir(f.lm);
        self.mac_to_rgb();
    }
    fn light_color(&mut self, v: usize, f: Fields) {
        self.multiply(1, v, 3, f.sf, f.lm);
        self.multiply(2, 3, 1, f.sf, f.lm);
    }
    fn nc(&mut self, v: usize, f: Fields) {
        self.light_color(v, f);
        self.mac_to_rgb();
    }
    fn ncc(&mut self, v: usize, f: Fields) {
        self.light_color(v, f);
        self.color_times_ir(f);
    }
    fn ncd(&mut self, v: usize, f: Fields) {
        self.light_color(v, f);
        self.dcpl(f);
    }
    fn avsz(&mut self, sum: u32, zsf: i64) {
        let avg = zsf * sum as i64;
        self.f(avg);
        self.set_mac(0, avg as i32);
        self.dr[7] = self.lim_d(self.mac(0) >> 12, false) as u16 as u32;
    }
    /// Execute one COP2 command and return the existing 003/008 latency.
    pub fn execute(&mut self, cmd: u32) -> u32 {
        let f = Fields {
            sf: if cmd & (1 << 19) != 0 { 12 } else { 0 },
            mx: ((cmd >> 17) & 3) as usize,
            v: ((cmd >> 15) & 3) as usize,
            cv: ((cmd >> 13) & 3) as usize,
            lm: cmd & (1 << 10) != 0,
        };
        self.flags = 0;
        let cycles = match cmd & 63 {
            0 | 1 => {
                self.rtp(0, f, true);
                15
            }
            6 => {
                self.nclip();
                8
            }
            0x0c => {
                let a = self.ir(1);
                let b = self.ir(2);
                let c = self.ir(3);
                let x = lo(self.cr[0]);
                let y = lo(self.cr[2]);
                let z = lo(self.cr[4]);
                self.set_mac(1, y.wrapping_mul(c).wrapping_sub(z.wrapping_mul(b)) >> f.sf);
                self.set_mac(2, z.wrapping_mul(a).wrapping_sub(x.wrapping_mul(c)) >> f.sf);
                self.set_mac(3, x.wrapping_mul(b).wrapping_sub(y.wrapping_mul(a)) >> f.sf);
                self.mac_to_ir(f.lm);
                6
            }
            0x10 => {
                self.dpcs(f, false);
                8
            }
            0x11 => {
                self.intpl(f);
                8
            }
            0x12 => {
                self.multiply(f.mx, f.v, f.cv, f.sf, f.lm);
                8
            }
            0x13 => {
                self.ncd(0, f);
                19
            }
            0x14 => {
                self.multiply(2, 3, 1, f.sf, f.lm);
                self.dcpl(f);
                13
            }
            0x16 => {
                for v in 0..3 {
                    self.ncd(v, f);
                }
                44
            }
            0x1a | 0x29 => {
                self.dcpl(f);
                8
            }
            0x1b => {
                self.ncc(0, f);
                17
            }
            0x1c => {
                self.multiply(2, 3, 1, f.sf, f.lm);
                self.color_times_ir(f);
                11
            }
            0x1e => {
                self.nc(0, f);
                14
            }
            0x20 => {
                for v in 0..3 {
                    self.nc(v, f);
                }
                30
            }
            0x28 => {
                for i in 1..4 {
                    let ir = self.ir(i);
                    self.set_mac(i, ir.wrapping_mul(ir) >> f.sf);
                }
                self.mac_to_ir(f.lm);
                5
            }
            0x2a => {
                for _ in 0..3 {
                    self.dpcs(f, true);
                }
                17
            }
            0x2d => {
                self.avsz(self.z(1) + self.z(2) + self.z(3), lo(self.cr[29]) as i64);
                5
            }
            0x2e => {
                self.avsz(
                    self.z(0) + self.z(1) + self.z(2) + self.z(3),
                    lo(self.cr[30]) as i64,
                );
                5
            }
            0x30 => {
                self.rtp(0, f, false);
                self.rtp(1, f, false);
                self.rtp(2, f, true);
                23
            }
            0x3d => {
                for i in 1..4 {
                    self.set_mac(i, self.ir(0).wrapping_mul(self.ir(i)) >> f.sf);
                }
                self.mac_to_ir(f.lm);
                self.mac_to_rgb();
                5
            }
            0x3e => {
                for i in 1..4 {
                    let v =
                        ((self.mac(i) as i64) << f.sf) + self.ir(0).wrapping_mul(self.ir(i)) as i64;
                    let value = (self.i44(i - 1, v) >> f.sf) as i32;
                    self.set_mac(i, value);
                }
                self.mac_to_ir(f.lm);
                self.mac_to_rgb();
                5
            }
            0x3f => {
                for v in 0..3 {
                    self.ncc(v, f);
                }
                39
            }
            _ => 1,
        };
        if self.flags & ERROR_MASK != 0 {
            self.flags |= 1 << 31;
        }
        self.cr[31] = self.flags;
        cycles
    }
}

#[cfg(test)]
mod tests;
