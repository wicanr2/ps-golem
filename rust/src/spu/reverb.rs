//! Own Go reverb from internal/spu/spu.go a60af2b; inherited Beetle attribution in parent.
use super::{clamp, Spu};
const RES: [i32; 20] = [
    -1, 2, -10, 35, -103, 266, -616, 1332, -2960, 10246, 10246, -2960, 1332, -616, 266, -103, 35,
    -10, 2, -1,
];
fn down(src: &[i16; 128], off: usize) -> i32 {
    let mut out = 0i32;
    for i in 0..20 {
        out = out.wrapping_add(RES[i] * src[off + i * 2] as i32)
    }
    out = out.wrapping_add(0x4000 * src[off + 19] as i32);
    clamp(out >> 15)
}
fn up(src: &[i16; 64], off: usize) -> i32 {
    let mut out = 0i32;
    for i in 0..20 {
        out = out.wrapping_add(RES[i] * src[off + i] as i32)
    }
    clamp(out >> 14)
}
fn neg(v: i16) -> i32 {
    if v == i16::MIN {
        32767
    } else {
        -(v as i32)
    }
}
fn iiasm(a: i16, s: i16) -> i32 {
    if a == i16::MIN {
        if s == i16::MIN {
            0
        } else {
            s as i32 * -65536
        }
    } else {
        s as i32 * (32768 - a as i32)
    }
}
impl Spu {
    fn rv_offset(&self, offset: u32) -> u32 {
        let mut o = self.reverb_cur + (offset & 0x3ffff);
        o += self.reverb_wa & 0u32.wrapping_sub((o >> 18) & 1);
        o & 0x3ffff
    }
    fn rd_rv(&mut self, raw: u16, extra: i32) -> i16 {
        self.read_ram(self.rv_offset((((raw as i32) << 2) + extra) as u32)) as i16
    }
    fn wr_rv(&mut self, raw: u16, s: i16) {
        if self.control & 128 != 0 {
            self.write_ram(self.rv_offset((raw as u32) << 2), s as u16)
        }
    }
    fn rs(&self, i: usize) -> i16 {
        self.regs[i] as i16
    }
    fn rv_channel(&mut self, ch: usize, down: i32) {
        let r = self.regs;
        let coef = self.rs(0xe7) as i32;
        let input = self.rs(0xfe + ch) as i32;
        let alpha = self.rs(0xe2);
        let ia = clamp(
            (((self.rd_rv(r[0xf0 + ch], 0) as i32 * coef) >> 14) + ((down * input) >> 14)) >> 1,
        ) as i16;
        let ib = clamp(
            (((self.rd_rv(r[0xf8 + (ch ^ 1)], 0) as i32 * coef) >> 14) + ((down * input) >> 14))
                >> 1,
        ) as i16;
        let a = clamp(
            (((ia as i32 * alpha as i32) >> 14)
                + (iiasm(alpha, self.rd_rv(r[0xea + ch], -1)) >> 14))
                >> 1,
        ) as i16;
        let b = clamp(
            (((ib as i32 * alpha as i32) >> 14)
                + (iiasm(alpha, self.rd_rv(r[0xf2 + ch], -1)) >> 14))
                >> 1,
        ) as i16;
        self.wr_rv(r[0xea + ch], a);
        self.wr_rv(r[0xf2 + ch], b);
        let mut acc = 0;
        for (src, coef) in [(0xec, 0xe3), (0xee, 0xe4), (0xf4, 0xe5), (0xf6, 0xe6)] {
            acc += (self.rd_rv(r[src + ch], 0) as i32 * self.rs(coef) as i32) >> 14
        }
        let fa = self.rd_rv(r[0xfa + ch].wrapping_sub(r[0xe0]), 0) as i32;
        let fb = self.rd_rv(r[0xfc + ch].wrapping_sub(r[0xe1]), 0) as i32;
        let alpha = self.rs(0xe8);
        let x = self.rs(0xe9);
        let ma = clamp((acc + ((fa * neg(alpha)) >> 14)) >> 1) as i16;
        let mb =
            clamp(fa + ((((ma as i32 * alpha as i32) >> 14) + ((fb * neg(x)) >> 14)) >> 1)) as i16;
        let v = clamp(fb + ((mb as i32 * x as i32) >> 15)) as i16;
        self.wr_rv(r[0xfa + ch], ma);
        self.wr_rv(r[0xfc + ch], mb);
        let p = (self.rvb_pos >> 1) as usize;
        self.rusb[ch][p | 32] = v;
        self.rusb[ch][p] = v;
    }
    pub(super) fn reverb(&mut self, input: [i32; 2]) -> [i32; 2] {
        for (ch, &v) in input.iter().enumerate() {
            self.rdsb[ch][self.rvb_pos as usize] = v as i16;
            self.rdsb[ch][self.rvb_pos as usize | 64] = v as i16
        }
        if self.rvb_pos & 1 != 0 {
            let d = down(&self.rdsb[0], ((self.rvb_pos - 38) & 63) as usize);
            self.rv_channel(0, d)
        } else {
            let d = down(&self.rdsb[1], ((self.rvb_pos - 38) & 63) as usize);
            self.rv_channel(1, d);
            self.reverb_cur = (self.reverb_cur + 1) & 0x3ffff;
            if self.reverb_cur == 0 {
                self.reverb_cur = self.reverb_wa
            }
        }
        let src = (((self.rvb_pos >> 1) - 19) & 31) as usize;
        let out = if self.rvb_pos & 1 != 0 {
            [up(&self.rusb[0], src), self.rusb[1][src + 9] as i32]
        } else {
            [self.rusb[0][src + 9] as i32, up(&self.rusb[1], src)]
        };
        self.rvb_pos = (self.rvb_pos + 1) & 63;
        out
    }
}
