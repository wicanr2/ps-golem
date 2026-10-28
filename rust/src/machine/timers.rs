//! Timer architecture from machine/io.go at 343afa4, specs 006/008/026 §7.
#[derive(Clone, Copy, Debug, Default)]
struct Timer {
    count: u32,
    mode: u32,
    target: u32,
}
#[derive(Clone, Debug, Default)]
pub(super) struct Timers {
    t: [Timer; 3],
    divider: u64,
}
impl Timers {
    pub(super) fn peek(&self, addr: u32) -> u32 {
        let t = &self.t[((addr - 0x1f801100) >> 4) as usize];
        match addr & 15 {
            0 => t.count,
            4 => t.mode,
            8 => t.target,
            _ => 0,
        }
    }
    pub(super) fn read(&mut self, addr: u32) -> u32 {
        let value = self.peek(addr);
        if addr & 15 == 4 {
            self.t[((addr - 0x1f801100) >> 4) as usize].mode &= !(3 << 11);
        }
        value
    }
    pub(super) fn write(&mut self, addr: u32, v: u32) {
        let t = &mut self.t[((addr - 0x1f801100) >> 4) as usize];
        match addr & 15 {
            0 => t.count = v & 65535,
            4 => {
                t.mode = v & 0x3ff | 1 << 10;
                t.count = 0;
            }
            8 => t.target = v & 65535,
            _ => {}
        }
    }
    pub(super) fn tick(&mut self, sys: u64, dots: u64, lines: u64) -> u32 {
        let mut irq = 0;
        for i in 0..3 {
            let mut steps = sys;
            match i {
                0 if self.t[0].mode & (1 << 8) != 0 => steps = dots,
                1 if self.t[1].mode & (1 << 8) != 0 => steps = lines,
                2 if self.t[2].mode & (1 << 9) != 0 => {
                    self.divider += sys;
                    steps = self.divider / 8;
                    self.divider %= 8;
                }
                _ => {}
            }
            while steps > 0 {
                let t = &mut self.t[i];
                let distance = |v: u32| {
                    let n = v.wrapping_sub(t.count) & 65535;
                    (if n == 0 { 65536 } else { n }) as u64
                };
                let e = distance(t.target).min(distance(65535));
                if steps < e {
                    t.count = (t.count + steps as u32) & 65535;
                    break;
                }
                t.count = (t.count + e as u32) & 65535;
                let mut hit = false;
                if t.count == t.target {
                    t.mode |= 1 << 11;
                    hit = t.mode & (1 << 4) != 0;
                    if t.mode & (1 << 3) != 0 {
                        t.count = 0;
                    }
                }
                if t.count == 65535 {
                    t.mode |= 1 << 12;
                    hit |= t.mode & (1 << 5) != 0;
                }
                if hit {
                    irq |= 1 << (4 + i);
                }
                steps -= e;
            }
        }
        irq
    }
}
