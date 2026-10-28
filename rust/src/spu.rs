//! Native sound processor, 013 / 026 section10. Own Go a60af2b port.
//! Reference: beetle-psx-libretro e57860d82faa953dc0fc10f201fd583739bc2f7d,
//! mednafen/psx/spu.c and spu.h, GPL-2.0-or-later. Crate GPL-3.0-or-later.
mod fir;
mod reverb;
#[cfg(test)]
mod tests;
mod voice;
use voice::{Sweep, Voice};
pub trait CdAudio {
    fn cd_audio_sample(&mut self) -> (i32, i32);
}
impl CdAudio for crate::cdrom::Cdrom {
    fn cd_audio_sample(&mut self) -> (i32, i32) {
        crate::cdrom::Cdrom::cd_audio_sample(self)
    }
}
#[derive(Clone)]
pub struct Spu {
    voices: [Voice; 24],
    noise_divider: u32,
    noise_counter: u32,
    lfsr: u16,
    fm_mode: u32,
    noise_mode: u32,
    reverb_mode: u32,
    reverb_wa: u32,
    global_sweep: [Sweep; 2],
    reverb_vol: [i32; 2],
    cd_vol: [i32; 2],
    extern_vol: [i32; 2],
    irq_addr: u32,
    rw_addr: u32,
    control: u16,
    voice_on: u32,
    voice_off: u32,
    block_end: u32,
    cwa: u32,
    regs: [u16; 256],
    aux: [u16; 16],
    rdsb: [[i16; 128]; 2],
    rusb: [[i16; 64]; 2],
    rvb_pos: i32,
    reverb_cur: u32,
    irq: bool,
    clock_divider: i32,
    ram: Vec<u16>,
    buf: Vec<[i16; 2]>,
    buf_pos: u32,
}
fn clamp(v: i32) -> i32 {
    v.clamp(-32768, 32767)
}
impl Default for Spu {
    fn default() -> Self {
        Self::new()
    }
}
impl Spu {
    pub fn new() -> Self {
        Self {
            voices: std::array::from_fn(|_| Voice::default()),
            noise_divider: 0,
            noise_counter: 0,
            lfsr: 0,
            fm_mode: 0,
            noise_mode: 0,
            reverb_mode: 0,
            reverb_wa: 0,
            global_sweep: [Sweep::default(); 2],
            reverb_vol: [0; 2],
            cd_vol: [0; 2],
            extern_vol: [0; 2],
            irq_addr: 0,
            rw_addr: 0,
            control: 0,
            voice_on: 0,
            voice_off: 0,
            block_end: 0,
            cwa: 0,
            regs: [0; 256],
            aux: [0; 16],
            rdsb: [[0; 128]; 2],
            rusb: [[0; 64]; 2],
            rvb_pos: 0,
            reverb_cur: 0,
            irq: false,
            clock_divider: 768,
            ram: vec![0; 262144],
            buf: vec![[0; 2]; 4096],
            buf_pos: 0,
        }
    }
    pub fn irq(&self) -> bool {
        self.irq
    }
    pub fn irq_enabled(&self) -> bool {
        self.control & 64 != 0
    }
    pub fn ram(&self) -> &[u16] {
        &self.ram
    }
    pub fn take_samples(&mut self) -> Vec<i16> {
        let mut out = Vec::with_capacity(self.buf_pos as usize * 2);
        for v in &self.buf[..self.buf_pos as usize] {
            out.extend_from_slice(v)
        }
        self.buf_pos = 0;
        out
    }
    fn check_irq(&mut self, addr: u32) {
        if self.control & 64 != 0 && self.irq_addr == addr {
            self.irq = true
        }
    }
    fn write_ram(&mut self, a: u32, v: u16) {
        self.check_irq(a);
        self.ram[a as usize] = v
    }
    fn read_ram(&mut self, a: u32) -> u16 {
        self.check_irq(a);
        self.ram[a as usize]
    }
    fn decode(&mut self, n: usize) {
        let v = &mut self.voices[n];
        if v.avail >= 11 {
            if self.control & 64 != 0 {
                let a = v.cur_addr.wrapping_sub(1) & 0x3ffff;
                if self.irq_addr == a || self.irq_addr == a & 0x3fff8 {
                    self.irq = true
                }
            }
            return;
        }
        if v.cur_addr & 7 == 0 && v.flags & 1 != 0 {
            v.cur_addr = v.loop_addr & !7;
            self.block_end |= 1 << n;
            if v.flags & 2 == 0 && self.noise_mode & (1 << n) == 0 {
                v.adsr.phase = 3;
                v.adsr.env = 0
            }
        }
        if self.control & 64 != 0 {
            let a = v.cur_addr & 0x3ffff;
            if self.irq_addr == a || self.irq_addr == a & 0x3fff8 {
                self.irq = true
            }
        }
        if v.cur_addr & 7 == 0 {
            let cv = self.ram[v.cur_addr as usize];
            v.shift = (cv & 15) as u8;
            v.weight = ((cv >> 4) & 15) as u8;
            v.flags = (cv >> 8) as u8;
            if v.flags & 4 != 0 && !v.ignore_loop {
                v.loop_addr = v.cur_addr
            }
            v.cur_addr = (v.cur_addr + 1) & 0x3ffff
        }
        let weights = [[0, 0], [60, 0], [115, -52], [98, -55], [122, -60]];
        let w = if v.weight < 5 {
            weights[v.weight as usize]
        } else {
            [0, 0]
        };
        let mut cv = self.ram[v.cur_addr as usize];
        let mut shift = v.shift;
        if shift > 12 {
            shift = 8;
            cv &= 0x8888
        }
        let mut coded = (cv as u32) << 12;
        for i in 0..4 {
            let mut sample = ((coded & 0xf000) as u16 as i16 as i32) >> shift;
            sample += (v.m2 as i32 * w[1]) >> 6;
            sample += (v.m1 as i32 * w[0]) >> 6;
            let sample = clamp(sample) as i16;
            v.decode[(v.wp + i) as usize] = sample;
            v.m2 = v.m1;
            v.m1 = sample;
            coded >>= 4
        }
        v.wp = (v.wp + 4) & 31;
        v.avail = v.avail.wrapping_add(4);
        v.cur_addr = (v.cur_addr + 1) & 0x3ffff;
    }
    fn noise(&mut self) {
        let rf = ((self.control >> 8) & 63) as u32;
        let (mut div, mut inc) = (2 << (rf >> 2), 4 + (rf & 3));
        if rf >= 60 {
            div = 0x8000;
            inc = 8
        }
        self.noise_divider = self.noise_divider.wrapping_add(div);
        if self.noise_divider & 0x8000 != 0 {
            self.noise_divider = 0;
            self.noise_counter += inc;
            if self.noise_counter & 8 != 0 {
                self.noise_counter &= 7;
                let l = self.lfsr;
                self.lfsr = (l << 1) | ((l >> 15 ^ l >> 12 ^ l >> 11 ^ l >> 10 ^ 1) & 1)
            }
        }
    }
    pub fn advance(&mut self, clocks: i64, mut cd: Option<&mut dyn CdAudio>) -> i64 {
        let mut div = self.clock_divider as i64 - clocks;
        let mut count = 0;
        while div <= 0 {
            div += 768;
            count += 1
        }
        self.clock_divider = div as i32;
        for _ in 0..count {
            self.sample(&mut cd)
        }
        self.clock_divider as i64
    }
    fn sample(&mut self, cd: &mut Option<&mut dyn CdAudio>) {
        let (mut accum, mut rv) = ([0i32; 2], [0i32; 2]);
        let fm = self.fm_mode & !1;
        let mut st = self.control & 63;
        if self.irq {
            st |= 64
        }
        if self.regs[0xd6] == 4 && self.cwa & 0x100 != 0 {
            st |= 0x800
        }
        self.regs[0xd7] = st;
        for n in 0..24 {
            let bit = 1 << n;
            let prev = if n == 0 { 0 } else { self.voices[n - 1].pre_lr };
            self.voices[n].pre_lr = 0;
            if self.voices[n].delay != 0 {
                self.voices[n].ignore_loop = false
            }
            self.decode(n);
            let v = &mut self.voices[n];
            let mut pvs = if self.noise_mode & bit != 0 {
                self.lfsr as i16 as i32
            } else {
                let si = v.rp;
                let f = fir::FIR[((v.phase & 0xfff) >> 4) as usize];
                let mut sum = 0i32;
                for i in 0..4 {
                    sum = sum.wrapping_add(
                        v.decode[((si + i) & 31) as usize] as i32 * f[i as usize] as i32,
                    )
                }
                sum >> 15
            };
            pvs = (pvs * v.adsr.env as i16 as i32) >> 15;
            v.pre_lr = pvs;
            // RAM capture for voices 1/3 occurs here, before envelope/key changes.
            if n == 1 || n == 3 {
                let a = 0x400 | ((n as u32 >> 1) * 0x200) | self.cwa;
                if self.control & 64 != 0 && self.irq_addr == a {
                    self.irq = true
                }
                self.ram[a as usize] = pvs as u16
            }
            for ch in 0..2 {
                let x = (pvs * v.sweep[ch].current as i16 as i32) >> 15;
                accum[ch] += x;
                if self.reverb_mode & bit != 0 {
                    rv[ch] += x
                }
                v.sweep[ch].run()
            }
            if v.delay == 0 {
                v.envelope();
                let mut inc = if fm & bit != 0 {
                    (v.pitch as i32 + ((v.pitch as i16 as i32 * prev) >> 15)) as u32
                } else {
                    v.pitch as u32
                };
                inc = inc.min(0x3fff);
                let tmp = v.phase + inc;
                let used = tmp >> 12;
                v.phase = tmp & 0xfff;
                v.avail = v.avail.wrapping_sub(used);
                v.rp = (v.rp + used) & 31
            } else {
                v.delay -= 1
            }
            if self.voice_off & bit != 0 && v.adsr.phase != 3 && v.delay < 3 {
                v.adsr.divider = 0;
                v.adsr.phase = 3
            }
            if self.voice_on & bit != 0 {
                v.adsr.env = 0;
                v.adsr.divider = 0;
                v.adsr.phase = 0;
                v.flags = 0;
                v.wp = 0;
                v.rp = 0;
                v.avail = 0;
                v.delay = 4;
                self.block_end &= !bit;
                v.m2 = 0;
                v.m1 = 0;
                v.phase = 0;
                v.cur_addr = v.start_addr & !7;
                v.ignore_loop = false
            }
            if self.control & 0x8000 == 0 {
                v.adsr.phase = 3;
                v.adsr.env = 0
            }
        }
        self.voice_off = 0;
        self.voice_on = 0;
        if self.control & 0x4000 == 0 {
            accum = [0; 2];
            rv = [0; 2]
        }
        let raw = if let Some(c) = cd.as_mut() {
            let (l, r) = c.cd_audio_sample();
            [l, r]
        } else {
            [0, 0]
        };
        self.write_ram(self.cwa, raw[0] as u16);
        self.write_ram(self.cwa | 0x200, raw[1] as u16);
        for ch in 0..2 {
            let x = (raw[ch] * self.cd_vol[ch]) >> 15;
            if self.control & 1 != 0 {
                accum[ch] += x;
                if self.control & 4 != 0 {
                    rv[ch] += x
                }
            }
        }
        self.cwa = (self.cwa + 1) & 511;
        self.noise();
        for v in &mut rv {
            *v = clamp(*v)
        }
        let reverb = self.reverb(rv);
        for ch in 0..2 {
            accum[ch] += reverb[ch] * self.reverb_vol[ch] >> 15;
            accum[ch] = clamp(accum[ch])
        }
        if self.buf_pos < 4096 {
            for (ch, &v) in accum.iter().enumerate() {
                let out = clamp(v * self.global_sweep[ch].current as i16 as i32 >> 15);
                self.buf[self.buf_pos as usize][ch] = ((out * 3 + 2) >> 2) as i16
            }
            self.buf_pos += 1
        }
        for s in &mut self.global_sweep {
            s.run()
        }
    }
    pub fn dma_write(&mut self, v: u32) {
        self.write_ram(self.rw_addr, v as u16);
        self.rw_addr = (self.rw_addr + 1) & 0x3ffff;
        self.write_ram(self.rw_addr, (v >> 16) as u16);
        self.rw_addr = (self.rw_addr + 1) & 0x3ffff;
        self.check_irq(self.rw_addr)
    }
    pub fn dma_read(&mut self) -> u32 {
        let mut r = self.read_ram(self.rw_addr) as u32;
        self.rw_addr = (self.rw_addr + 1) & 0x3ffff;
        r |= (self.read_ram(self.rw_addr) as u32) << 16;
        self.rw_addr = (self.rw_addr + 1) & 0x3ffff;
        self.check_irq(self.rw_addr);
        r
    }
    pub fn write(&mut self, addr: u32, value: u16) {
        let a = (addr & 0x3ff) as usize;
        let v = value;
        let v32 = v as u32;
        if a >= 0x200 {
            if a < 0x260 {
                self.voices[(a - 0x200) >> 2].sweep[(a & 2) >> 1].current = v
            } else if a < 0x280 {
                self.aux[(a & 31) >> 1] = v
            }
            return;
        }
        if a < 0x180 {
            let vo = &mut self.voices[a >> 4];
            match a & 15 {
                0 | 2 => vo.sweep[(a & 2) >> 1].control = v,
                4 => vo.pitch = v,
                6 => vo.start_addr = (v32 << 2) & 0x3ffff,
                8 => {
                    vo.adsr_control = vo.adsr_control & 0xffff0000 | v32;
                    vo.cache_envelope()
                }
                10 => {
                    vo.adsr_control = vo.adsr_control & 65535 | v32 << 16;
                    vo.cache_envelope()
                }
                12 => vo.adsr.env = v,
                14 => {
                    vo.loop_addr = (v32 << 2) & 0x3ffff;
                    vo.ignore_loop = true
                }
                _ => {}
            }
        } else {
            match a & 127 {
                0 | 2 => self.global_sweep[(a & 2) >> 1].control = v,
                4 => self.reverb_vol[0] = v as i16 as i32,
                6 => self.reverb_vol[1] = v as i16 as i32,
                8 => self.voice_on = self.voice_on & 0xffff0000 | v32,
                10 => self.voice_on = self.voice_on & 65535 | (v32 & 255) << 16,
                12 => self.voice_off = self.voice_off & 0xffff0000 | v32,
                14 => self.voice_off = self.voice_off & 65535 | (v32 & 255) << 16,
                16 => self.fm_mode = self.fm_mode & 0xffff0000 | v32,
                18 => self.fm_mode = self.fm_mode & 65535 | (v32 & 255) << 16,
                20 => self.noise_mode = self.noise_mode & 0xffff0000 | v32,
                22 => self.noise_mode = self.noise_mode & 65535 | (v32 & 255) << 16,
                24 => self.reverb_mode = self.reverb_mode & 0xffff0000 | v32,
                26 => self.reverb_mode = self.reverb_mode & 65535 | (v32 & 255) << 16,
                28 => self.block_end = self.block_end & 0xffff0000 | v32,
                30 => self.block_end = self.block_end & 65535 | v32 << 16,
                34 => {
                    self.reverb_wa = (v32 << 2) & 0x3ffff;
                    self.reverb_cur = self.reverb_wa
                }
                36 => {
                    self.irq_addr = (v32 << 2) & 0x3ffff;
                    self.check_irq(self.rw_addr)
                }
                38 => {
                    self.rw_addr = (v32 << 2) & 0x3ffff;
                    self.check_irq(self.rw_addr)
                }
                40 => {
                    self.write_ram(self.rw_addr, v);
                    self.rw_addr = (self.rw_addr + 1) & 0x3ffff;
                    self.check_irq(self.rw_addr)
                }
                42 => {
                    self.control = v;
                    if v & 64 == 0 {
                        self.irq = false
                    }
                    self.check_irq(self.rw_addr)
                }
                48 => self.cd_vol[0] = v as i16 as i32,
                50 => self.cd_vol[1] = v as i16 as i32,
                52 => self.extern_vol[0] = v as i16 as i32,
                54 => self.extern_vol[1] = v as i16 as i32,
                56 | 58 => self.global_sweep[(a & 2) >> 1].current = v,
                _ => {}
            }
        }
        self.regs[(a & 511) >> 1] = v;
    }
    pub fn read(&mut self, addr: u32) -> u16 {
        let a = (addr & 0x3ff) as usize;
        if a >= 0x200 {
            return if a < 0x260 {
                self.voices[(a - 0x200) >> 2].sweep[(a & 2) >> 1].current
            } else if a < 0x280 {
                self.aux[(a & 31) >> 1]
            } else {
                65535
            };
        }
        if a < 0x180 {
            let v = &self.voices[a >> 4];
            match a & 15 {
                12 => return v.adsr.env,
                14 => return (v.loop_addr >> 2) as u16,
                _ => {}
            }
        } else {
            match a & 127 {
                28 => return self.block_end as u16,
                30 => return (self.block_end >> 16) as u16,
                40 => {
                    let v = self.read_ram(self.rw_addr);
                    self.rw_addr = (self.rw_addr + 1) & 0x3ffff;
                    self.check_irq(self.rw_addr);
                    return v;
                }
                42 => return self.control,
                60 => return 0,
                56 | 58 => return self.global_sweep[(a & 2) >> 1].current,
                _ => {}
            }
        }
        self.regs[(a & 511) >> 1]
    }
}
