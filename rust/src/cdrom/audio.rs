//! Own Go audio.go f00311b; inherited Beetle cdc.c attribution in parent.
use super::*;
#[derive(Clone)]
pub(super) struct Audio {
    pub(super) samples: [[i16; 4096]; 2],
    pub size: u32,
    pub(super) freq: u32,
    pub read: u32,
    pub(super) prev: [[i16; 2]; 2],
    pub(super) resamp: [[i16; 64]; 2],
    pub(super) pos: usize,
    pub(super) phase: usize,
    pub pending_vol: [[u8; 2]; 2],
    pub vol: [[u8; 2]; 2],
}
impl Default for Audio {
    fn default() -> Self {
        Self {
            samples: [[0; 4096]; 2],
            size: 0,
            freq: 0,
            read: 0,
            prev: [[0; 2]; 2],
            resamp: [[0; 64]; 2],
            pos: 0,
            phase: 0,
            pending_vol: [[0; 2]; 2],
            vol: [[0; 2]; 2],
        }
    }
}
impl Audio {
    pub fn clear(&mut self) {
        let p = self.pending_vol;
        let v = self.vol;
        *self = Self::default();
        self.pending_vol = p;
        self.vol = v
    }
    pub fn reset_volume(&mut self) {
        self.pending_vol = [[128, 0], [0, 128]];
        self.vol = self.pending_vol
    }
    fn read_buffer(&mut self) -> [i32; 2] {
        let i = (self.read & 4095) as usize;
        let r = [self.samples[0][i] as i32, self.samples[1][i] as i32];
        self.read = self.read.wrapping_add(1);
        r
    }
}
const IMP: [[i16; 25]; 7] = [
    [
        0, -5, 17, -35, 70, -23, -68, 347, -839, 2062, -4681, 15367, 21472, -5882, 2810, -1352,
        635, -235, 26, 43, -35, 16, -8, 2, 0,
    ],
    [
        0, -2, 10, -34, 65, -84, 52, 9, -266, 1024, -2680, 9036, 26516, -6016, 3021, -1571, 848,
        -365, 107, 10, -16, 17, -8, 3, -1,
    ],
    [
        -2, 0, 3, -19, 60, -75, 162, -227, 306, -67, -615, 3229, 29883, -4532, 2488, -1471, 882,
        -424, 166, -27, 5, 6, -8, 3, -1,
    ],
    [
        -1, 3, -2, -5, 31, -74, 179, -402, 689, -926, 1272, -1446, 31033, -1446, 1272, -926, 689,
        -402, 179, -74, 31, -5, -2, 3, -1,
    ],
    [
        -1, 3, -8, 6, 5, -27, 166, -424, 882, -1471, 2488, -4532, 29883, 3229, -615, -67, 306,
        -227, 162, -75, 60, -19, 3, 0, -2,
    ],
    [
        -1, 3, -8, 17, -16, 10, 107, -365, 848, -1571, 3021, -6016, 26516, 9036, -2680, 1024, -266,
        9, 52, -84, 65, -34, 10, -2, 0,
    ],
    [
        0, 2, -8, 16, -35, 43, 26, -235, 635, -1352, 2810, -5882, 21472, 15367, -4681, 2062, -839,
        347, -68, -23, 70, -35, 17, -5, 0,
    ],
];
fn sat(v: i32) -> i32 {
    v.clamp(-32768, 32767)
}
fn decode(input: &[u8; 28], out: &mut [i16; 30], shift: u8, weight: u8) {
    let weights = [[0, 0], [60, 0], [115, -52], [98, -55], [122, -60]];
    let w = if weight < 5 {
        weights[weight as usize]
    } else {
        [0, 0]
    };
    for i in 0..28 {
        let mut s = ((input[i] as u16) << 8) as i16 as i32;
        s >>= shift;
        s += ((out[i + 1] as i32 * w[0]) >> 6) + ((out[i] as i32 * w[1]) >> 6);
        out[i + 2] = sat(s) as i16
    }
}
impl Cdrom {
    pub fn cd_audio_sample(&mut self) -> (i32, i32) {
        let a = &mut self.audio;
        if a.read >= a.size || a.freq == 0 {
            return (0, 0);
        }
        let mut out = [0; 2];
        if a.freq == 7 || a.freq == 14 {
            out = a.read_buffer();
            if a.freq == 14 {
                out = a.read_buffer()
            }
        } else {
            for (ch, v) in out.iter_mut().enumerate() {
                let base = (a.pos + 32 - 25) & 31;
                let mut acc = 0i32;
                for i in 0..25 {
                    acc = acc.wrapping_add(IMP[a.phase][i] as i32 * a.resamp[ch][base + i] as i32)
                }
                *v = sat(acc >> 15)
            }
            a.phase += a.freq as usize;
            if a.phase >= 7 {
                a.phase -= 7;
                let raw = a.read_buffer();
                for (ch, v) in raw.iter().enumerate() {
                    a.resamp[ch][a.pos] = *v as i16;
                    a.resamp[ch][a.pos + 32] = *v as i16
                }
                a.pos = (a.pos + 1) & 31
            }
        }
        if self.muted {
            return (0, 0);
        }
        let v = a.vol;
        (
            sat(((out[0] * v[0][0] as i32) >> 7) + ((out[1] * v[1][0] as i32) >> 7)),
            sat(((out[0] * v[0][1] as i32) >> 7) + ((out[1] * v[1][1] as i32) >> 7)),
        )
    }
    pub(super) fn xa_process(&mut self, buf: &[u8; SECTOR_SIZE]) {
        let a = &mut self.audio;
        let coding = buf[19];
        let sh = if coding & 16 != 0 { 0 } else { 1 };
        let stereo = coding & 1 != 0;
        a.read = 0;
        a.size = 18 * (4 << sh) * 28;
        if stereo {
            a.size >>= 1
        }
        a.freq = if coding & 4 != 0 { 3 } else { 6 };
        for group in 0..18usize {
            let sg = &buf[24 + group * 128..];
            let params = &sg[..16];
            let samples = &sg[16..128];
            for unit in 0..(4usize << sh) {
                let param = params[(unit & 3) | ((unit & 4) << 1)];
                let copy = params[4 | (unit & 3) | ((unit & 4) << 1)];
                let mut input = [0; 28];
                for i in 0..28 {
                    let mut t = samples[i * 4 + (unit >> sh)];
                    if sh != 0 {
                        if unit & 1 == 0 {
                            t = t.wrapping_shl(4)
                        }
                        t &= 0xf0
                    }
                    input[i] = t
                }
                let ch = if unit & 1 != 0 && stereo { 1 } else { 0 };
                let mut out = [0; 30];
                out[..2].copy_from_slice(&a.prev[ch]);
                decode(&input, &mut out, param & 15, param >> 4);
                a.prev[ch] = [out[28], out[29]];
                if param != copy {
                    out.fill(0)
                }
                if stereo {
                    let base = group * (2 << sh) * 28 + (unit >> 1) * 28;
                    a.samples[ch][base..base + 28].copy_from_slice(&out[2..]);
                } else {
                    let base = group * (4 << sh) * 28 + unit * 28;
                    for ch in 0..2 {
                        a.samples[ch][base..base + 28].copy_from_slice(&out[2..])
                    }
                }
            }
        }
    }
    pub(super) fn buffer_cdda(&mut self, buf: &[u8; SECTOR_SIZE]) {
        let a = &mut self.audio;
        a.freq = if self.mode & 128 != 0 { 14 } else { 7 };
        a.size = 588;
        for i in 0..588 {
            for ch in 0..2 {
                a.samples[ch][i] = if self.subq[0] & 64 != 0 {
                    0
                } else {
                    i16::from_le_bytes([buf[i * 4 + ch * 2], buf[i * 4 + ch * 2 + 1]])
                }
            }
        }
        a.read = 0;
    }
}
