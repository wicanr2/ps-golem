//! SIO0 / digital pads, 010 and 026 §8. GPL-3.0-or-later.
//! Port of ps-golem internal/sio/sio.go at 343afa4.
//! Reference: beetle-psx-libretro frontio.c, e57860d82faa953dc0fc10f201fd583739bc2f7d,
//! GPL-2.0-or-later. No SIO1 / multitap / analog protocol in this slice.
mod card;
pub use card::Memcard;
#[derive(Clone, Debug, Default)]
pub struct Gamepad {
    pub buttons: u16,
    dtr: bool,
    phase: i32,
    bit: u32,
    rx: u8,
    tx: [u8; 3],
    pos: usize,
    count: usize,
}
impl Gamepad {
    fn select(&mut self, on: bool) {
        if !self.dtr && on {
            self.phase = 0;
            self.bit = 0;
            self.pos = 0;
            self.count = 0;
        }
        self.dtr = on;
    }
    fn clock(&mut self, txd: bool) -> (bool, i64) {
        if !self.dtr {
            return (true, 0);
        }
        let ret = self.count == 0 || (self.tx[self.pos] >> self.bit) & 1 != 0;
        self.rx &= !(1 << self.bit);
        if txd {
            self.rx |= 1 << self.bit;
        }
        self.bit = (self.bit + 1) & 7;
        if self.bit != 0 {
            return (ret, 0);
        }
        if self.count > 0 {
            self.pos += 1;
            self.count -= 1;
        }
        match self.phase {
            0 => {
                if self.rx != 1 {
                    self.phase = -1;
                } else {
                    self.tx[0] = 0x41;
                    self.pos = 0;
                    self.count = 1;
                    self.phase += 1;
                }
            }
            1 => {
                self.phase += 1;
                self.tx[0] = 0x5a;
                if self.rx == 0x42 {
                    self.tx[1] = !(self.buttons as u8);
                    self.tx[2] = !((self.buttons >> 8) as u8);
                    self.pos = 0;
                    self.count = 3;
                } else {
                    self.phase = -1;
                    self.pos = 0;
                    self.count = 0;
                }
            }
            _ => {}
        }
        (ret, if self.count > 0 { 64 } else { 0 })
    }
}
#[derive(Clone, Debug)]
pub struct Sio {
    pub pads: [Option<Gamepad>; 2],
    pub cards: [Option<Memcard>; 2],
    mode: u32,
    control: u32,
    baud: u32,
    tx_buf: u8,
    tx_pending: bool,
    tx_active: bool,
    tx_bit: u32,
    rx_buf: u8,
    rx_pending: bool,
    rx_active: bool,
    rx_avail: bool,
    rx_bit: u32,
    divider: i64,
    ack_delay: [i64; 4],
    ack_until: [i64; 4],
    now: i64,
    irq: bool,
}
impl Default for Sio {
    fn default() -> Self {
        Self::new()
    }
}
impl Sio {
    pub fn new() -> Self {
        Self {
            pads: [None, None],
            cards: [None, None],
            mode: 0,
            control: 0,
            baud: 0,
            tx_buf: 0,
            tx_pending: false,
            tx_active: false,
            tx_bit: 0,
            rx_buf: 0,
            rx_pending: false,
            rx_active: false,
            rx_avail: false,
            rx_bit: 0,
            divider: 0,
            ack_delay: [0; 4],
            ack_until: [-1; 4],
            now: 0,
            irq: false,
        }
    }
    pub fn irq(&self) -> bool {
        self.irq
    }
    fn clock_divider(&self) -> i64 {
        (((self.baud << [0, 0, 4, 6][(self.mode & 3) as usize]) as i64) & !1).max(32)
    }
    fn check_start_stop(&mut self) {
        if (self.rx_pending && self.control & 4 != 0) || (self.tx_pending && self.control & 1 != 0)
        {
            if self.rx_pending {
                self.rx_pending = false;
                self.rx_active = true;
                self.rx_avail = false;
                self.rx_buf = 0;
                self.rx_bit = 0;
            }
            if self.tx_pending {
                self.tx_pending = false;
                self.tx_active = true;
                self.tx_bit = 0;
            }
            self.divider = self.clock_divider();
        }
        if self.control & 5 == 0 {
            self.rx_active = false;
            self.tx_active = false;
        }
        if !self.rx_active && !self.tx_active {
            self.divider = 0;
        }
    }
    pub fn write(&mut self, addr: u32, value: u32) {
        match addr & 15 {
            0 => {
                self.tx_buf = value as u8;
                self.tx_pending = true;
                self.tx_active = false;
            }
            8 => self.mode = value & 0x013f,
            10 => {
                self.control = value & 0x3f2f;
                if value & 16 != 0 {
                    self.irq = false;
                }
                if value & 64 != 0 {
                    self.irq = false;
                    self.divider = 0;
                    self.rx_pending = false;
                    self.tx_pending = false;
                    self.rx_active = false;
                    self.tx_active = false;
                    self.rx_avail = false;
                    self.tx_buf = 0;
                    self.rx_buf = 0;
                    self.rx_bit = 0;
                    self.tx_bit = 0;
                    self.mode = 0;
                    self.control = 0;
                    self.baud = 0;
                }
                for port in 0..2 {
                    let on = self.control & 2 != 0 && ((self.control & 0x2000 != 0) == (port == 1));
                    if let Some(p) = self.pads[port].as_mut() {
                        p.select(on);
                    }
                    if let Some(c) = self.cards[port].as_mut() {
                        c.select(on);
                    }
                    if !on {
                        self.ack_delay[port] = 0;
                        self.ack_delay[port + 2] = 0;
                        self.ack_until[port] = -1;
                        self.ack_until[port + 2] = -1;
                    }
                }
            }
            14 => self.baud = value & 65535,
            _ => {}
        }
        self.check_start_stop();
    }
    pub fn read(&mut self, addr: u32) -> u32 {
        match addr & 15 {
            0 => {
                let value = self.rx_buf as u32 * 0x01010101;
                self.rx_avail = false;
                self.rx_pending = true;
                self.rx_active = false;
                self.check_start_stop();
                value
            }
            4 => {
                (!self.tx_pending && !self.tx_active) as u32
                    | ((self.rx_avail as u32) << 1)
                    | ((self.ack_until.iter().any(|&until| self.now < until) as u32) << 7)
                    | ((self.irq as u32) << 9)
            }
            8 => self.mode,
            10 => self.control,
            14 => self.baud,
            _ => 0,
        }
    }
    pub fn tick(&mut self, clocks: i64) {
        assert!(clocks >= 0);
        self.now += clocks;
        for i in 0..4 {
            if self.ack_delay[i] > 0 {
                self.ack_delay[i] -= clocks;
                if self.ack_delay[i] <= 0 {
                    self.ack_until[i] = self.now + 32 + self.ack_delay[i];
                    if self.control & 0x1000 != 0 {
                        self.irq = true;
                    }
                }
            }
        }
        if self.divider <= 0 {
            return;
        }
        self.divider -= clocks;
        let mut check = false;
        while self.divider <= 0 {
            if !self.rx_active && !self.tx_active {
                break;
            }
            let mut txd = true;
            if self.tx_active {
                txd = (self.tx_buf >> self.tx_bit) & 1 != 0;
                self.tx_bit = (self.tx_bit + 1) & 7;
                if self.tx_bit == 0 {
                    check = true;
                    self.tx_active = false;
                    if self.control & 0x400 != 0 {
                        self.irq = true;
                    }
                }
            }
            let mut rxd = true;
            for i in 0..4 {
                let (bit, delay) = if i < 2 {
                    self.pads[i].as_mut().map_or((true, 0), |p| p.clock(txd))
                } else {
                    self.cards[i - 2]
                        .as_mut()
                        .map_or((true, 0), |c| c.clock(txd))
                };
                rxd &= bit;
                if delay > 0 {
                    self.ack_delay[i] = delay;
                }
            }
            if self.rx_active {
                self.rx_buf &= !(1 << self.rx_bit);
                if rxd {
                    self.rx_buf |= 1 << self.rx_bit;
                }
                self.rx_bit = (self.rx_bit + 1) & 7;
                if self.rx_bit == 0 {
                    check = true;
                    self.rx_active = false;
                    self.rx_avail = true;
                    if self.control & 0x800 != 0 {
                        self.irq = true;
                    }
                }
            }
            self.divider += self.clock_divider();
        }
        if check {
            self.check_start_stop();
        }
    }
    pub fn next_event(&self) -> i64 {
        let mut next = -1;
        for &delay in &self.ack_delay {
            if delay > 0 && (next < 0 || delay < next) {
                next = delay;
            }
        }
        if self.divider > 0
            && (self.rx_active || self.tx_active)
            && (next < 0 || self.divider < next)
        {
            next = self.divider;
        }
        next
    }
    pub fn state_words(&self) -> Vec<u64> {
        let mut words = vec![
            self.mode as u64,
            self.control as u64,
            self.baud as u64,
            self.tx_buf as u64,
            self.tx_pending as u64,
            self.tx_active as u64,
            self.tx_bit as u64,
            self.rx_buf as u64,
            self.rx_pending as u64,
            self.rx_active as u64,
            self.rx_avail as u64,
            self.rx_bit as u64,
            self.divider as u64,
            self.now as u64,
            self.irq as u64,
        ];
        words.extend(self.ack_delay.iter().map(|v| *v as u64));
        words.extend(self.ack_until.iter().map(|v| *v as u64));
        words
    }
}
#[cfg(test)]
mod tests;
