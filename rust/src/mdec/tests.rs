use super::*;
use std::{fs, path::PathBuf};
struct Tape {
    bytes: Vec<u8>,
    i: usize,
}
impl Tape {
    fn new(path: PathBuf, magic: &[u8]) -> Self {
        let bytes = fs::read(path).unwrap();
        assert_eq!(&bytes[..8], magic);
        Self { bytes, i: 8 }
    }
    fn u32(&mut self) -> u32 {
        let n = u32::from_le_bytes(self.bytes[self.i..self.i + 4].try_into().unwrap());
        self.i += 4;
        n
    }
    fn u64(&mut self) -> u64 {
        let n = u64::from_le_bytes(self.bytes[self.i..self.i + 8].try_into().unwrap());
        self.i += 8;
        n
    }
    fn end(&self) {
        assert_eq!(self.i, self.bytes.len());
    }
}
fn fixtures() -> Option<PathBuf> {
    match std::env::var("PSGOLEM_RUST_MDEC_FIXTURES") {
        Ok(s) => Some(s.into()),
        Err(_) => {
            eprintln!("SKIP: select fixed Go MDEC fixtures");
            None
        }
    }
}
#[test]
fn all_formats_tables_clock_fifo_and_dma_offsets() {
    let Some(root) = fixtures() else { return };
    let mut t = Tape::new(root.join("rust-mdec-oracle-v1.bin"), b"PSGMDC01");
    let n = t.u32();
    let mut d = Mdec::new();
    for i in 0..n {
        let (k, a, v) = (t.u32(), t.u32(), t.u32());
        let (value, offset) = match k {
            0 => {
                d.write(a, v);
                (0, 0)
            }
            1 => (d.read(a), 0),
            2 => {
                d.tick(a as i32);
                (0, 0)
            }
            3 => {
                d.dma_write(v);
                (0, 0)
            }
            4 => d.dma_read(),
            5 => {
                d = Mdec::new();
                (0, 0)
            }
            _ => panic!("event {k}"),
        };
        assert_eq!(
            (value, offset),
            (t.u32(), t.u32()),
            "MDEC event {i} kind{k} a{a:08x} v{v:08x}"
        );
        assert_eq!(d.status(), t.u32(), "MDEC status event {i}");
        assert_eq!(d.oracle_hash(), t.u64(), "MDEC full-state event {i}");
        if i % 199 == 0 {
            let clone = d.clone();
            assert_eq!(clone.oracle_hash(), d.oracle_hash());
        }
    }
    t.end();
    eprintln!("RUST_MDEC: {n} Go events, all return values/status/full states exact; clone passed");
}
#[test]
fn bus_dma_and_real_bios_public_programs() {
    use crate::{
        cpu::Bus,
        machine::{Machine, SHELL_ENTRY},
    };
    let Some(root) = fixtures() else { return };
    let mut t = Tape::new(root.join("rust-mdec-machine-oracle-v1.bin"), b"PSGMDM01");
    let count = t.u32();
    let mut m = Machine::new(&vec![0; 0x80000]).unwrap();
    for i in 0..count {
        let (k, a, v) = (t.u32(), t.u32(), t.u32());
        let value = match k {
            0 => {
                m.bus.write32(a, v);
                0
            }
            1 => m.bus.read32(a),
            2 => {
                m.bus.write8(a, v);
                0
            }
            3 => {
                m.bus.write16(a, v);
                0
            }
            4 => {
                m.run(a as u64);
                0
            }
            _ => panic!("bus event {k}"),
        };
        assert_eq!(value, t.u32(), "bus read {i}");
        assert_eq!(m.cycles(), t.u64(), "bus cycles {i}");
        assert_eq!(
            m.bus.mdec.oracle_hash(),
            t.u64(),
            "bus MDEC state {i}, kind{k} a{a:08x} v{v:08x}"
        );
        for a in [
            0x1f801070, 0x1f801080, 0x1f801084, 0x1f801088, 0x1f801090, 0x1f801094, 0x1f801098,
            0x1f8010f0, 0x1f8010f4,
        ] {
            assert_eq!(
                m.bus.read32(a),
                t.u32(),
                "bus/DMA register {a:08x} event{i}"
            );
        }
        let mut h = 14695981039346656037u64;
        for &b in m.bus.ram() {
            h ^= b as u64;
            h = h.wrapping_mul(1099511628211);
        }
        assert_eq!(h, t.u64(), "bus full RAM event{i}");
    }
    t.end();
    eprintln!(
        "RUST_MDEC_MACHINE: {count} Go bus/DMA/timing events, full RAM and device state exact"
    );
    let Ok(bios) = std::env::var("PSGOLEM_RUST_MDEC_BIOS") else {
        eprintln!("SKIP: select real BIOS for nine public MDEC programs");
        return;
    };
    let mut boot = Machine::new(&fs::read(bios).unwrap()).unwrap();
    boot.run_until_pc(SHELL_ENTRY, 400000000).unwrap();
    let dir = root.join("rust-mdec-public-v1");
    for name in [
        "4bit",
        "8bit",
        "frame-15bit",
        "frame-15bit-dma",
        "frame-24bit",
        "frame-24bit-dma",
        "movie-15bit",
        "movie-24bit",
        "step-by-step-log",
    ] {
        let mut m = boot.clone();
        let mark = m.tty().len();
        m.sideload_exe(&fs::read(dir.join(format!("{name}.exe"))).unwrap())
            .unwrap();
        m.run(200000000);
        assert_eq!(
            &m.tty()[mark..],
            fs::read(dir.join(format!("{name}.tty"))).unwrap(),
            "{name} TTY"
        );
        let want = fs::read(dir.join(format!("{name}.vram"))).unwrap();
        let actual: Vec<u8> = m
            .bus
            .gpu
            .vram()
            .iter()
            .flat_map(|v| v.to_le_bytes())
            .collect();
        assert_eq!(actual.len(), want.len());
        assert_eq!(
            actual.iter().zip(&want).position(|(a, b)| a != b),
            None,
            "{name} full VRAM first differing byte"
        );
        let want = fs::read(dir.join(format!("{name}.ram"))).unwrap();
        assert_eq!(m.bus.ram().len(), want.len());
        assert_eq!(
            m.bus.ram().iter().zip(&want).position(|(a, b)| a != b),
            None,
            "{name} full RAM first differing byte"
        );
        let want = fs::read(dir.join(format!("{name}.state"))).unwrap();
        let words: Vec<u64> = want
            .chunks_exact(8)
            .map(|v| u64::from_le_bytes(v.try_into().unwrap()))
            .collect();
        assert_eq!(
            vec![
                m.cycles(),
                m.field(),
                m.cpu.pc() as u64,
                m.bus.mdec.oracle_hash()
            ],
            words,
            "{name} endpoint state"
        );
        eprintln!("RUST_MDEC_PUBLIC: {name}, cycles={}, field={}, PC={:08x}, exact Go TTY/full VRAM/RAM/MDEC state",m.cycles(),m.field(),m.cpu.pc());
    }
}
