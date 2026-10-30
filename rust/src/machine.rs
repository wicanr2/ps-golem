//! Own native minimal machine, 006 / 007 / 008 and 026 §7.
//! GPL-3.0-or-later. Port of machine/{machine,io,exe}.go at 343afa4.
//! CD/SIO integrated by 026 sections 8-9; SPU integrated by 026 section10; MDEC integrated by 026 section11.
use crate::{
    cpu::{Bus, Cpu},
    gpu::Gpu,
};
mod dma;
pub mod phase;
mod timers;
use dma::Dma;
use timers::Timers;
pub const SHELL_ENTRY: u32 = 0x80030000;
pub fn physical(addr: u32) -> u32 {
    match addr >> 29 {
        4 => addr & 0x7fffffff,
        5 => addr & 0x1fffffff,
        _ => addr,
    }
}
fn scratch(virt: u32, p: u32) -> bool {
    (0x1f800000..0x1f800400).contains(&p) && virt >> 29 != 5
}
fn width_mask(size: usize) -> u32 {
    match size {
        1 => 255,
        2 => 65535,
        _ => !0,
    }
}
#[derive(Clone)]
pub struct SystemBus {
    ram: Vec<u8>,
    bios: Vec<u8>,
    scratch: [u8; 1024],
    biu: u32,
    regs: [u8; 4096],
    pub gpu: Gpu,
    i_stat: u32,
    i_mask: u32,
    dma: Dma,
    timers: Timers,
    video_acc: u64,
    dot_acc: u64,
    field: u64,
    gpu_irq: bool,
    sio_irq: bool,
    pub sio: crate::sio::Sio,
    pub cd: crate::cdrom::Cdrom,
    cd_irq: bool,
    pub spu: crate::spu::Spu,
    spu_irq: bool,
    pub mdec: crate::mdec::Mdec,
}
#[derive(Clone)]
pub struct Machine {
    pub cpu: Cpu,
    pub bus: SystemBus,
    cycles: u64,
    tty: Vec<u8>,
}
impl Machine {
    pub fn new(bios: &[u8]) -> Result<Self, String> {
        if bios.len() != 0x80000 {
            return Err(format!("BIOS size {}, expected 524288", bios.len()));
        }
        let mut sio = crate::sio::Sio::new();
        sio.pads[0] = Some(crate::sio::Gamepad::default());
        sio.cards[0] = Some(crate::sio::Memcard::new());
        Ok(Self {
            cpu: Cpu::new(),
            bus: SystemBus {
                ram: vec![0; 2 << 20],
                bios: bios.to_vec(),
                scratch: [0; 1024],
                biu: 0,
                regs: [0; 4096],
                gpu: Gpu::new(),
                i_stat: 0,
                i_mask: 0,
                dma: Dma::default(),
                timers: Timers::default(),
                video_acc: 0,
                dot_acc: 0,
                field: 0,
                gpu_irq: false,
                sio_irq: false,
                sio,
                cd: crate::cdrom::Cdrom::new(),
                cd_irq: false,
                spu: crate::spu::Spu::new(),
                spu_irq: false,
                mdec: crate::mdec::Mdec::new(),
            },
            cycles: 0,
            tty: Vec::new(),
        })
    }
    pub fn insert_disc(&mut self, disc: std::sync::Arc<crate::disc::Disc>) {
        self.bus.cd.insert(Some(disc));
    }
    pub fn take_audio(&mut self) -> Vec<i16> {
        self.bus.spu.take_samples()
    }
    pub fn cycles(&self) -> u64 {
        self.cycles
    }
    pub fn field(&self) -> u64 {
        self.bus.field
    }
    pub fn tty(&self) -> &[u8] {
        &self.tty
    }
    pub fn step(&mut self) {
        let p = physical(self.cpu.pc());
        let function = self.cpu.gpr(9);
        if (p == 0xa0 && function == 0x3c) || (p == 0xb0 && function == 0x3d) {
            self.tty.push(self.cpu.gpr(4) as u8);
        }
        self.cpu.set_irq(self.bus.irq_pending());
        let n = self.cpu.step(&mut self.bus) as u64;
        self.cycles += n;
        self.bus.advance(n);
        self.cpu.set_irq(self.bus.irq_pending());
    }
    pub fn run(&mut self, cycles: u64) {
        let end = self.cycles.saturating_add(cycles);
        while self.cycles < end {
            self.step();
        }
    }
    pub fn run_until_pc(&mut self, pc: u32, max_cycles: u64) -> Result<(), String> {
        let end = self.cycles.saturating_add(max_cycles);
        while self.cycles < end {
            if physical(self.cpu.pc()) == physical(pc) {
                return Ok(());
            }
            self.step();
        }
        Err(format!(
            "PC {:08x} not reached after {} cycles, current {:08x}",
            pc,
            max_cycles,
            self.cpu.pc()
        ))
    }
    pub fn sideload_exe(&mut self, bytes: &[u8]) -> Result<(), String> {
        if bytes.len() < 0x800 || &bytes[..8] != b"PS-X EXE" {
            return Err("Invalid PS-EXE header".into());
        }
        let word =
            |offset: usize| u32::from_le_bytes(bytes[offset..offset + 4].try_into().unwrap());
        let (pc, gp, dest, size, bss, bss_size, sp, sp_off) = (
            word(0x10),
            word(0x14),
            word(0x18),
            word(0x1c),
            word(0x28),
            word(0x2c),
            word(0x30),
            word(0x34),
        );
        if size as usize > bytes.len() - 0x800 {
            return Err("Truncated PS-EXE body".into());
        }
        for (i, &v) in bytes[0x800..0x800 + size as usize].iter().enumerate() {
            self.bus.ram[(physical(dest).wrapping_add(i as u32) & 0x1fffff) as usize] = v;
        }
        for i in 0..bss_size {
            self.bus.ram[(physical(bss).wrapping_add(i) & 0x1fffff) as usize] = 0;
        }
        self.cpu.set_gpr(28, gp);
        if sp != 0 {
            self.cpu.set_gpr(29, sp.wrapping_add(sp_off));
            self.cpu.set_gpr(30, sp.wrapping_add(sp_off));
        }
        self.cpu.set_gpr(4, 1);
        self.cpu.set_gpr(5, 0);
        self.cpu.set_pc(pc);
        Ok(())
    }
}
impl SystemBus {
    pub fn ram(&self) -> &[u8] {
        &self.ram
    }
    pub fn irq_registers(&self) -> (u32, u32) {
        (self.i_stat, self.i_mask)
    }
    fn irq_pending(&self) -> bool {
        self.i_stat & self.i_mask != 0
    }
    fn advance(&mut self, n: u64) {
        let early_mdec = self.mdec.waiting_clock() || self.dma.mdec_active();
        if early_mdec {
            self.mdec.tick(n as i32);
            self.mdec_pump();
        }
        self.video_acc += n * 11;
        let video = self.video_acc / 7;
        self.video_acc %= 7;
        let (blank, lines) = self.gpu.tick_video(video as u32);
        if blank > 0 {
            self.field += blank as u64;
            self.i_stat |= 1;
        }
        self.dot_acc += video;
        let div = self.gpu.dot_divider() as u64;
        let dots = self.dot_acc / div;
        self.dot_acc %= div;
        self.i_stat |= self.timers.tick(n, dots, lines as u64);
        self.cd.tick_with_spu(n as i64, &mut self.spu);
        let spu_irq = self.spu.irq();
        if spu_irq && !self.spu_irq {
            self.i_stat |= 1 << 9;
        }
        self.spu_irq = spu_irq;
        let cd_irq = self.cd.irq();
        if cd_irq && !self.cd_irq {
            self.i_stat |= 1 << 2;
        }
        self.cd_irq = cd_irq;
        if !early_mdec {
            self.mdec.tick(n as i32);
        }
        self.mdec_pump();
        self.sio.tick(n as i64);
        let serial_irq = self.sio.irq();
        if serial_irq && !self.sio_irq {
            self.i_stat |= 1 << 7;
        }
        self.sio_irq = serial_irq;
        let irq = self.gpu.irq();
        if irq && !self.gpu_irq {
            self.i_stat |= 2;
        }
        self.gpu_irq = irq;
    }
    fn reg32(&self, p: u32) -> u32 {
        u32::from_le_bytes(
            self.regs[(p - 0x1f801000) as usize..][..4]
                .try_into()
                .unwrap(),
        )
    }
    fn set_reg32(&mut self, p: u32, v: u32) {
        self.regs[(p - 0x1f801000) as usize..][..4].copy_from_slice(&v.to_le_bytes());
    }
    fn reg16(&self, p: u32) -> u16 {
        u16::from_le_bytes(
            self.regs[(p - 0x1f801000) as usize..][..2]
                .try_into()
                .unwrap(),
        )
    }
    fn set_reg16(&mut self, p: u32, v: u16) {
        self.regs[(p - 0x1f801000) as usize..][..2].copy_from_slice(&v.to_le_bytes());
    }
    fn spu_read(&mut self, p: u32) -> u16 {
        self.spu.read(p)
    }
    fn io_peek(&self, p: u32) -> u32 {
        match p {
            0x1f801070 => self.i_stat,
            0x1f801074 => self.i_mask,
            0x1f801080..=0x1f8010fc => self.dma.read(p),
            0x1f801100..=0x1f80112c => self.timers.peek(p),
            0x1f801820 => 0,
            0x1f801824 => self.mdec.status(),
            _ => self.reg32(p),
        }
    }
    fn io_read32(&mut self, p: u32) -> u32 {
        match p {
            0x1f801054 => 5,
            0x1f801070 => self.i_stat,
            0x1f801074 => self.i_mask,
            0x1f801080..=0x1f8010fc => self.dma.read(p),
            0x1f801100..=0x1f80112c => self.timers.read(p),
            0x1f801810 => self.gpu.read_data(),
            0x1f801814 => self.gpu.status(),
            0x1f801820 | 0x1f801824 => {
                let v = self.mdec.read(p);
                self.mdec_pump();
                v
            }
            _ => self.reg32(p),
        }
    }
    fn io_read(&mut self, p: u32, size: usize) -> u32 {
        if p >= 0x1f802000 {
            return width_mask(size);
        }
        if (0x1f801040..0x1f801050).contains(&p) {
            return self.sio.read(p & 0xe) & width_mask(size);
        }
        if (0x1f801800..0x1f801804).contains(&p) {
            return self.cd.read(p) as u32;
        }
        if p >= 0x1f801c00 {
            let h = p & !1;
            return match size {
                4 => self.spu_read(h) as u32 | ((self.spu_read(h + 2) as u32) << 16),
                2 => self.spu_read(h) as u32,
                _ => ((self.spu_read(h) >> (8 * (p & 1))) & 255) as u32,
            };
        }
        (self.io_read32(p & !3) >> ((p & 3) * 8)) & width_mask(size)
    }
    fn io_full_width(p: u32) -> bool {
        if (0x1f801000..0x1f801024).contains(&p)
            || (0x1f801060..0x1f801064).contains(&p)
            || (0x1f801070..0x1f801078).contains(&p)
            || (0x1f8010f0..0x1f8010f8).contains(&p)
            || (0x1f801100..0x1f80112c).contains(&p)
        {
            return true;
        }
        (0x1f801080..0x1f8010f0).contains(&p) && p & 12 != 4
    }
    fn io_write(&mut self, p: u32, mut v: u32, size: usize) {
        if p >= 0x1f802000 {
            return;
        }
        if p >= 0x1f801c00 {
            if size == 4 {
                self.spu.write(p, v as u16);
                self.spu.write(p + 2, (v >> 16) as u16);
            } else if p & 1 == 0 {
                self.spu.write(p, v as u16);
            }
            return;
        }
        if size < 4 && Self::io_full_width(p) {
            self.io_write32(p & !3, v << ((p & 3) * 8));
            return;
        }
        v &= width_mask(size);
        if (0x1f801800..0x1f801804).contains(&p) {
            self.cd.write(p, v as u8);
            return;
        }
        if (0x1f801040..0x1f801050).contains(&p) {
            self.sio.write(p & 0xe, v);
            return;
        }
        let al = p & !3;
        if size < 4 {
            let shift = (p & 3) * 8;
            let mask = width_mask(size) << shift;
            v = self.io_peek(al) & !mask | (v << shift) & mask;
        }
        self.io_write32(al, v);
    }
    fn io_write32(&mut self, p: u32, v: u32) {
        match p {
            0x1f801070 => self.i_stat &= v,
            0x1f801074 => self.i_mask = v & 0x7ff,
            0x1f801080..=0x1f8010fc => self.dma_write(p, v),
            0x1f801100..=0x1f80112c => self.timers.write(p, v),
            0x1f801810 => self.gpu.gp0(v),
            0x1f801814 => self.gpu.gp1(v),
            0x1f801820 | 0x1f801824 => {
                self.mdec.write(p, v);
                self.mdec_pump();
            }
            _ => self.set_reg32(p, v),
        }
    }
    fn read(&mut self, addr: u32, size: usize) -> u32 {
        let p = physical(addr);
        let bytes = if p < 0x800000 {
            Some((&self.ram[..], (p & 0x1fffff) as usize))
        } else if scratch(addr, p) {
            Some((&self.scratch[..], (p & 1023) as usize))
        } else if (0x1fc00000..0x1fc80000).contains(&p) {
            Some((&self.bios[..], (p - 0x1fc00000) as usize))
        } else {
            None
        };
        if let Some((bytes, offset)) = bytes {
            let mut value = 0;
            for i in 0..size {
                value |= (bytes[offset + i] as u32) << (8 * i);
            }
            return value;
        }
        if (0x1f801000..0x1f803000).contains(&p) {
            return self.io_read(p, size);
        }
        if (0x1f000000..0x1f800000).contains(&p) {
            return width_mask(size);
        }
        if size == 4 && p == 0xfffe0130 {
            return self.biu;
        }
        0
    }
    fn write(&mut self, addr: u32, v: u32, size: usize) {
        let p = physical(addr);
        let bytes = if p < 0x800000 {
            Some((&mut self.ram[..], (p & 0x1fffff) as usize))
        } else if scratch(addr, p) {
            Some((&mut self.scratch[..], (p & 1023) as usize))
        } else {
            None
        };
        if let Some((bytes, offset)) = bytes {
            for i in 0..size {
                bytes[offset + i] = (v >> (8 * i)) as u8;
            }
            return;
        }
        if (0x1f801000..0x1f803000).contains(&p) {
            self.io_write(p, v, size);
        } else if size == 4 && p == 0xfffe0130 {
            self.biu = v & !0x440;
        }
    }
}
impl Bus for SystemBus {
    fn read8(&mut self, a: u32) -> u8 {
        self.read(a, 1) as u8
    }
    fn read16(&mut self, a: u32) -> u16 {
        self.read(a, 2) as u16
    }
    fn read32(&mut self, a: u32) -> u32 {
        self.read(a, 4)
    }
    fn write8(&mut self, a: u32, v: u32) {
        self.write(a, v, 1);
    }
    fn write16(&mut self, a: u32, v: u32) {
        self.write(a, v, 2);
    }
    fn write32(&mut self, a: u32, v: u32) {
        self.write(a, v, 4);
    }
    fn isolated_write(&mut self, a: u32, v: u32, size: usize) {
        if self.biu & 0x81 == 0x80 {
            let offset = (a & 1023) & !(size as u32 - 1);
            for i in 0..size {
                self.scratch[offset as usize + i] = (v >> (8 * i)) as u8;
            }
        }
    }
    fn instruction_bus_error(&self, a: u32) -> bool {
        let p = physical(a);
        scratch(a, p)
            || (0x1f801070..0x1f801078).contains(&p)
            || (0x1f801820..0x1f801828).contains(&p)
    }
    fn read_cost(&self, a: u32, size: usize) -> u32 {
        let p = physical(a);
        let w = match size {
            2 => 1,
            4 => 2,
            _ => 0,
        };
        if p < 0x800000 {
            4
        } else if scratch(a, p) {
            0
        } else if (0x1fc00000..0x1fc80000).contains(&p) {
            [6, 12, 24][w]
        } else if (0x1f801800..0x1f801804).contains(&p) {
            [7, 13, 25][w]
        } else if (0x1f801c00..0x1f802000).contains(&p) {
            [17, 17, 38][w]
        } else if (0x1f801000..0x1f802000).contains(&p) {
            2
        } else if (0x1f802000..0x1f804000).contains(&p) {
            [10, 25, 55][w]
        } else if (0x1fa00000..0x1fc00000).contains(&p) {
            [5, 5, 9][w]
        } else if (0x1f000000..0x1f800000).contains(&p) {
            [6, 12, 24][w]
        } else {
            0
        }
    }
    fn biu(&self) -> u32 {
        self.biu
    }
}

#[cfg(test)]
mod tests;

#[cfg(test)]
mod cd_tests;

#[cfg(test)]
mod spu_tests;
