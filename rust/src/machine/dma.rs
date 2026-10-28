//! DMA2/OTC/DICR from machine/io.go at 343afa4, 006/007/026 §7.
use super::SystemBus;
#[derive(Clone, Debug)]
pub(super) struct Dma {
    madr: [u32; 7],
    bcr: [u32; 7],
    chcr: [u32; 7],
    dpcr: u32,
    dicr: u32,
}
impl Default for Dma {
    fn default() -> Self {
        let mut d = Self {
            madr: [0; 7],
            bcr: [0; 7],
            chcr: [0; 7],
            dpcr: 0x07654321,
            dicr: 0,
        };
        d.chcr[6] = 2;
        d
    }
}
impl Dma {
    pub(super) fn read(&self, p: u32) -> u32 {
        let off = p - 0x1f801080;
        if off >= 0x70 {
            return match off {
                0x70 => self.dpcr,
                0x74 => self.dicr,
                _ => 0,
            };
        }
        let ch = (off >> 4) as usize;
        match off & 15 {
            0 => self.madr[ch],
            4 => self.bcr[ch],
            _ => self.chcr[ch],
        }
    }
    fn update_master(&mut self, before: u32) -> bool {
        let flags = (self.dicr >> 24) & 127;
        let on = self.dicr & (1 << 15) != 0 || (self.dicr & (1 << 23) != 0 && flags != 0);
        self.dicr &= !(1 << 31);
        if on {
            self.dicr |= 1 << 31;
        }
        before & (1 << 31) == 0 && on
    }
}
impl SystemBus {
    pub(super) fn dma_write(&mut self, p: u32, v: u32) {
        let off = p - 0x1f801080;
        if off >= 0x70 {
            match off {
                0x70 => self.dma.dpcr = v,
                0x74 => {
                    let before = self.dma.dicr;
                    self.dma.dicr =
                        v & 0x00ff803f | (self.dma.dicr & 0x7f000000) & !(v & 0x7f000000);
                    if self.dma.update_master(before) {
                        self.i_stat |= 1 << 3;
                    }
                }
                _ => {}
            }
            return;
        }
        let ch = (off >> 4) as usize;
        match off & 15 {
            0 => self.dma.madr[ch] = v & 0xffffff,
            4 => self.dma.bcr[ch] = v,
            _ => {
                self.dma.chcr[ch] = if ch == 6 {
                    v & ((1 << 24) | (1 << 28) | (1 << 30)) | 2
                } else {
                    v & 0x71770703
                };
                let c = self.dma.chcr[ch];
                if c & (1 << 24) != 0
                    && self.dma.dpcr & (1 << (3 + 4 * ch)) != 0
                    && (ch != 6 || c & (1 << 28) != 0)
                {
                    self.dma_run(ch);
                }
            }
        }
    }
    fn ram_word(&self, a: u32) -> u32 {
        let a = (a & 0x1ffffc) as usize;
        u32::from_le_bytes(self.ram[a..a + 4].try_into().unwrap())
    }
    fn set_ram_word(&mut self, a: u32, v: u32) {
        let a = (a & 0x1ffffc) as usize;
        self.ram[a..a + 4].copy_from_slice(&v.to_le_bytes());
    }
    fn dma_run(&mut self, ch: usize) {
        match ch {
            2 => self.dma_gpu(),
            3 => self.dma_cd(),
            6 => {
                let mut addr = self.dma.madr[6] & 0x1ffffc;
                let n = self.dma.bcr[6] & 65535;
                let n = if n == 0 { 65536 } else { n };
                for i in 0..n {
                    let word = if i == n - 1 {
                        0xffffff
                    } else {
                        addr.wrapping_sub(4) & 0x1fffff
                    };
                    self.set_ram_word(addr, word);
                    addr = addr.wrapping_sub(4) & 0x1ffffc;
                }
            }
            _ => {}
        } // 006 M2: unimplemented device channels complete without payload.
        self.dma.chcr[ch] &= !(1 << 24 | 1 << 28);
        let before = self.dma.dicr;
        if self.dma.dicr & (1 << (16 + ch)) != 0 && self.dma.dicr & (1 << 23) != 0 {
            self.dma.dicr |= 1 << (24 + ch);
        }
        if self.dma.update_master(before) {
            self.i_stat |= 1 << 3;
        }
    }
    fn dma_cd(&mut self) {
        let mut words = self.dma.bcr[3] & 65535;
        if (self.dma.chcr[3] >> 9) & 3 == 1 {
            words = words.wrapping_mul(self.dma.bcr[3] >> 16);
        }
        if words == 0 {
            words = 65536;
        }
        let mut addr = self.dma.madr[3] & 0x1ffffc;
        for _ in 0..words {
            let v = self.cd.dma_read();
            self.set_ram_word(addr, v);
            addr = addr.wrapping_add(4) & 0x1ffffc;
        }
        if (self.dma.chcr[3] >> 9) & 3 == 1 {
            self.dma.madr[3] = addr;
            self.dma.bcr[3] &= 65535;
        }
    }
    fn dma_gpu(&mut self) {
        let control = self.dma.chcr[2];
        let to_device = control & 1 != 0;
        let step = if control & 2 != 0 { !3 } else { 4 };
        let mut addr = self.dma.madr[2] & 0x1ffffc;
        match (control >> 9) & 3 {
            0 | 1 => {
                let block = self.dma.bcr[2] & 65535;
                let block = if block == 0 { 65536 } else { block };
                let words = if (control >> 9) & 3 == 0 {
                    block
                } else {
                    block.wrapping_mul(self.dma.bcr[2] >> 16)
                };
                for _ in 0..words {
                    if to_device {
                        self.gpu.gp0(self.ram_word(addr));
                    } else {
                        let word = self.gpu.read_data();
                        self.set_ram_word(addr, word);
                    }
                    addr = addr.wrapping_add(step) & 0x1ffffc;
                }
                if (control >> 9) & 3 == 1 {
                    self.dma.madr[2] = addr;
                    self.dma.bcr[2] &= 65535;
                }
            }
            2 => {
                for _ in 0..(1 << 20) {
                    let header = self.ram_word(addr);
                    let count = header >> 24;
                    for i in 0..count {
                        self.gpu.gp0(self.ram_word(addr.wrapping_add(4 + 4 * i)));
                    }
                    if header & 0x800000 != 0 {
                        break;
                    }
                    addr = header & 0x1ffffc;
                }
                self.dma.madr[2] = 0xffffff;
            }
            _ => {}
        }
    }
}
