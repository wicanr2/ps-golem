//! Own Go adsr/sweep/voice from internal/spu/spu.go a60af2b. Parent attribution applies.
#[derive(Clone, Copy, Default)]
pub(super) struct Sweep {
    pub control: u16,
    pub current: u16,
    pub divider: u32,
}
#[derive(Clone, Default)]
pub(super) struct Adsr {
    pub env: u16,
    pub divider: u32,
    pub phase: u32,
    pub attack_exp: bool,
    pub sustain_exp: bool,
    pub sustain_dec: bool,
    pub release_exp: bool,
    pub attack_rate: i32,
    pub decay_rate: i32,
    pub sustain_rate: i32,
    pub release_rate: i32,
    pub sustain_level: i32,
}
#[derive(Clone, Default)]
pub(super) struct Voice {
    pub decode: [i16; 32],
    pub m2: i16,
    pub m1: i16,
    pub delay: u32,
    pub wp: u32,
    pub rp: u32,
    pub avail: u32,
    pub ignore_loop: bool,
    pub shift: u8,
    pub weight: u8,
    pub flags: u8,
    pub sweep: [Sweep; 2],
    pub pitch: u16,
    pub phase: u32,
    pub start_addr: u32,
    pub cur_addr: u32,
    pub adsr_control: u32,
    pub loop_addr: u32,
    pub pre_lr: i32,
    pub adsr: Adsr,
}
fn delta(zs: u8, speed: u8, log: bool, dec: bool, inv: bool, current: i16) -> (i32, i32) {
    let mut inc = 7 - (speed & 3) as i32;
    if inv {
        inc = !inc
    }
    let mut div = 32768;
    if speed < 0x2c {
        inc = ((inc as u32) << ((0x2f - speed as u32) >> 2)) as i32
    }
    if speed >= 0x30 {
        div >>= (speed as u32 - 0x2c) >> 2
    }
    if log {
        if dec {
            inc = (current as i32 * inc) >> 15
        } else if current as i32 & 0x7fff >= 0x6000 {
            if speed < 0x28 {
                inc >>= 2
            } else if speed >= 0x2c {
                div >>= 2
            } else {
                inc >>= 1;
                div >>= 1
            }
        }
    }
    if div == 0 && speed < zs {
        div = 1
    }
    (inc, div)
}
impl Sweep {
    fn clock(&mut self) {
        let log = self.control & 0x4000 != 0;
        let dec = self.control & 0x2000 != 0;
        let inv = self.control & 0x1000 != 0;
        let mask = if inv && !(dec && log) { 65535 } else { 0 };
        let test = if inv { 65535 } else { 0 };
        let (inc, div) = delta(
            0x7f,
            (self.control & 127) as u8,
            log,
            dec,
            (dec != inv) || (dec && log),
            (self.current ^ mask) as i16,
        );
        let sign = if inv { 0 } else { 0x8000 };
        if (dec && !(inv && log)) && (self.current & 0x8000 == sign || self.current == 0) {
            self.current = 0
        } else {
            self.divider = self.divider.wrapping_add(div as u32);
            if self.divider & 0x8000 != 0 {
                self.divider = 0;
                if dec || self.current ^ test != 0x7fff {
                    let prev = self.current;
                    self.current = (self.current as i32 + inc) as u16;
                    if !dec
                        && (self.current ^ prev) & 0x8000 != 0
                        && (self.current ^ test) & 0x8000 != 0
                    {
                        self.current = 0x7fff ^ test
                    }
                }
            }
        }
    }
    pub fn run(&mut self) {
        if self.control & 0x8000 != 0 {
            self.clock()
        } else {
            self.current = (self.control & 0x7fff) << 1
        }
    }
}
impl Voice {
    pub fn cache_envelope(&mut self) {
        let r = self.adsr_control;
        let a = &mut self.adsr;
        a.attack_exp = r & 0x8000 != 0;
        a.release_exp = r & (1 << 21) != 0;
        a.sustain_exp = r & (1 << 31) != 0;
        a.sustain_dec = r & (1 << 30) != 0;
        a.attack_rate = ((r >> 8) & 127) as i32;
        a.decay_rate = (((r >> 4) & 15) << 2) as i32;
        a.sustain_rate = ((r >> 22) & 127) as i32;
        a.release_rate = (((r >> 16) & 31) << 2) as i32;
        a.sustain_level = (((r & 15) + 1) << 11) as i32;
    }
    pub fn envelope(&mut self) {
        let a = &mut self.adsr;
        if a.phase == 0 && a.env == 0x7fff {
            a.phase += 1
        }
        let (inc, div, reset) = match a.phase {
            0 => {
                let (i, d) = delta(
                    127,
                    a.attack_rate as u8,
                    a.attack_exp,
                    false,
                    false,
                    a.env as i16,
                );
                (i, d, 0x7fff)
            }
            1 => {
                let (i, d) = delta(124, a.decay_rate as u8, true, true, true, a.env as i16);
                (i, d, 0)
            }
            2 => {
                let (i, d) = delta(
                    127,
                    a.sustain_rate as u8,
                    a.sustain_exp,
                    a.sustain_dec,
                    a.sustain_dec,
                    a.env as i16,
                );
                (i, d, if a.sustain_dec { 0 } else { 0x7fff })
            }
            3 => {
                let (i, d) = delta(
                    124,
                    a.release_rate as u8,
                    a.release_exp,
                    true,
                    true,
                    a.env as i16,
                );
                (i, d, 0)
            }
            _ => (0, 0, 0),
        };
        a.divider = a.divider.wrapping_add(div as u32);
        if a.divider & 0x8000 != 0 {
            let prev = a.env;
            a.divider = 0;
            a.env = (a.env as i32 + inc) as u16;
            if a.phase == 0 {
                if ((prev ^ a.env) & a.env) & 0x8000 != 0 {
                    a.env = reset
                }
            } else if a.env & 0x8000 != 0 {
                a.env = reset
            }
            if a.phase == 1 && (a.env as i32) < a.sustain_level {
                a.phase += 1
            }
        }
    }
}
