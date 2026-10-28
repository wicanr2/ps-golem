//! Native 128 KiB memory card protocol, 010 / 026 §8.
//! Port of own memcard.go at 343afa4; Beetle frontio.c e57860d82faa953dc0fc10f201fd583739bc2f7d,
//! GPL-2.0-or-later reference, project GPL-3.0-or-later.
#[derive(Clone, Debug)]
pub struct Memcard {
    pub data: Box<[u8; 131072]>,
    pub dirty: bool,
    pub writes: u64,
    presence_new: bool,
    buffer: [u8; 128],
    write_xor: u8,
    dtr: bool,
    phase: i32,
    bit: u32,
    rx: u8,
    cmd: u8,
    addr: u32,
    xor: u8,
    tx: u8,
    count: u32,
}
impl Default for Memcard {
    fn default() -> Self {
        Self::new()
    }
}
impl Memcard {
    pub fn new() -> Self {
        let mut c = Self {
            data: vec![0; 1 << 17].into_boxed_slice().try_into().unwrap(),
            dirty: false,
            writes: 0,
            presence_new: true,
            buffer: [0; 128],
            write_xor: 0,
            dtr: false,
            phase: 0,
            bit: 0,
            rx: 0,
            cmd: 0,
            addr: 0,
            xor: 0,
            tx: 0,
            count: 0,
        };
        c.format();
        c
    }
    pub fn load(&mut self, bytes: &[u8]) -> bool {
        if bytes.len() != 1 << 17 {
            return false;
        }
        self.data.copy_from_slice(bytes);
        self.dirty = false;
        true
    }
    pub fn format(&mut self) {
        self.data.fill(0);
        self.data[0] = 0x4d;
        self.data[1] = 0x43;
        self.data[127] = 0x0e;
        for a in (0x80..0x800).step_by(128) {
            self.data[a] = 0xa0;
            self.data[a + 8] = 255;
            self.data[a + 9] = 255;
            self.data[a + 127] = 0xa0;
        }
        for a in (0x800..0x1200).step_by(128) {
            self.data[a..a + 4].fill(255);
            self.data[a + 8] = 255;
            self.data[a + 9] = 255;
        }
    }
    pub(super) fn select(&mut self, on: bool) {
        if !self.dtr && on {
            self.phase = 0;
            self.bit = 0;
            self.count = 0;
        }
        self.dtr = on;
    }
    fn send(&mut self, value: u8) {
        self.tx = value;
        self.count = 1;
    }
    pub(super) fn clock(&mut self, txd: bool) -> (bool, i64) {
        if !self.dtr {
            return (true, 0);
        }
        let ret = self.count == 0 || (self.tx >> self.bit) & 1 != 0;
        self.rx &= !(1 << self.bit);
        if txd {
            self.rx |= 1 << self.bit;
        }
        self.bit = (self.bit + 1) & 7;
        if self.bit != 0 {
            return (ret, 0);
        }
        if self.count > 0 {
            self.count -= 1;
        }
        match self.phase {
            1024..=1151 => {
                let value = if self.addr == 65535 {
                    0
                } else {
                    self.data[((self.addr << 7) + (self.phase - 1024) as u32) as usize]
                };
                self.xor ^= value;
                self.send(value);
                self.phase += 1;
            }
            2048..=2175 => {
                self.xor ^= self.rx;
                self.buffer[(self.phase - 2048) as usize] = self.rx;
                self.send(self.rx);
                self.phase += 1;
            }
            0 => {
                if self.rx != 0x81 {
                    self.phase = -1;
                } else {
                    self.send(if self.presence_new { 8 } else { 0 });
                    self.phase += 1;
                }
            }
            1 => {
                self.cmd = self.rx;
                if self.cmd == b'R' || self.cmd == b'W' {
                    self.phase += 1;
                    self.send(0x5a);
                } else {
                    self.phase = -1;
                    self.tx = 0;
                    self.count = 0;
                }
            }
            2 => {
                self.send(0x5d);
                self.phase += 1;
            }
            3 => {
                self.send(0);
                self.phase = if self.cmd == b'R' { 1000 } else { 2000 };
            }
            1000 => {
                self.addr = (self.rx as u32) << 8;
                self.send(self.rx);
                self.phase += 1;
            }
            1001 => {
                self.addr |= self.rx as u32;
                self.send(b'\\');
                self.phase += 1;
            }
            1002 => {
                if self.addr >= 1024 {
                    self.addr = 65535;
                }
                self.xor = 0;
                self.send(b']');
                self.phase += 1;
            }
            1003 => {
                self.send((self.addr >> 8) as u8);
                self.xor ^= self.tx;
                self.phase += 1;
            }
            1004 => {
                self.send(self.addr as u8);
                self.xor ^= self.tx;
                self.phase = if self.addr == 65535 { -1 } else { 1024 };
            }
            1152 => {
                self.send(self.xor);
                self.phase += 1;
            }
            1153 => {
                self.send(b'G');
                self.phase = -1;
            }
            2000 => {
                self.xor = self.rx;
                self.addr = (self.rx as u32) << 8;
                self.send(self.rx);
                self.phase += 1;
            }
            2001 => {
                self.xor ^= self.rx;
                self.addr |= self.rx as u32;
                self.send(self.rx);
                self.phase = 2048;
            }
            2176 => {
                self.write_xor = self.rx;
                self.send(b'\\');
                self.phase += 1;
            }
            2177 => {
                self.send(b']');
                self.phase += 1;
            }
            2178 => {
                if self.xor != self.write_xor {
                    self.send(b'N');
                } else if self.addr >= 1024 {
                    self.send(255);
                } else {
                    self.send(b'G');
                    self.presence_new = false;
                    self.writes += 1;
                    let offset = (self.addr << 7) as usize;
                    if self.data[offset..offset + 128] != self.buffer {
                        self.data[offset..offset + 128].copy_from_slice(&self.buffer);
                        self.dirty = true;
                    }
                }
                self.phase = -1;
            }
            _ => {}
        }
        (ret, if self.count > 0 { 256 } else { 0 })
    }
}
