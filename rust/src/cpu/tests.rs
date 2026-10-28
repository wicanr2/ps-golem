use super::*;
const BASIS: u64 = 14695981039346656037;
fn hash(mut h: u64, v: u32) -> u64 {
    for b in v.to_le_bytes() {
        h = (h ^ b as u64).wrapping_mul(1099511628211);
    }
    h
}
struct Memory {
    ram: Vec<u8>,
    biu: u32,
    writes: u64,
    reads: u64,
}
impl Memory {
    fn read(&mut self, a: u32, n: usize) -> u32 {
        let mut v = 0;
        for i in 0..n {
            v |= (self.ram[(a.wrapping_add(i as u32) & 65535) as usize] as u32) << (8 * i);
        }
        self.reads = hash(hash(hash(self.reads, a), n as u32), v);
        v
    }
    fn write(&mut self, a: u32, v: u32, n: usize) {
        self.writes = hash(hash(hash(self.writes, a), n as u32), v);
        for i in 0..n {
            self.ram[(a.wrapping_add(i as u32) & 65535) as usize] = (v >> (8 * i)) as u8;
        }
    }
    fn digest(&self) -> u64 {
        self.ram
            .iter()
            .fold(BASIS, |h, &v| (h ^ v as u64).wrapping_mul(1099511628211))
    }
}
impl Bus for Memory {
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
    fn isolated_write(&mut self, a: u32, v: u32, n: usize) {
        self.writes = hash(hash(hash(self.writes, a), n as u32 | 0x80000000), v);
    }
    fn instruction_bus_error(&self, a: u32) -> bool {
        (0x1f800000..0x1fc00000).contains(&(a & 0x1fffffff))
    }
    fn read_cost(&self, a: u32, n: usize) -> u32 {
        if a & 0x1ff80000 == 0x1fc00000 {
            (n * 6) as u32
        } else {
            4
        }
    }
    fn biu(&self) -> u32 {
        self.biu
    }
}
struct Tape {
    bytes: Vec<u8>,
    at: usize,
}
impl Tape {
    fn u32(&mut self) -> u32 {
        let v = u32::from_le_bytes(self.bytes[self.at..self.at + 4].try_into().unwrap());
        self.at += 4;
        v
    }
    fn u64(&mut self) -> u64 {
        let v = u64::from_le_bytes(self.bytes[self.at..self.at + 8].try_into().unwrap());
        self.at += 8;
        v
    }
}
#[test]
fn fixed_go_single_step_oracle() {
    let Ok(path) = std::env::var("PSGOLEM_CPU_ORACLE") else {
        eprintln!(
            "SKIP: generate the fixed own-engine CPU tape with tools/oracle_rust_cpu.go in Docker"
        );
        return;
    };
    let mut tape = Tape {
        bytes: std::fs::read(path).expect("Explicit CPU oracle path must exist"),
        at: 8,
    };
    assert_eq!(&tape.bytes[..8], b"PSGCPU01");
    let cases = tape.u32();
    let mut steps = 0;
    for expected in 0..cases {
        let case = tape.u32();
        assert_eq!(case, expected);
        let biu = tape.u32();
        let pc = tape.u32();
        let mut cpu = Cpu::new();
        cpu.set_pc(pc);
        for i in 0..32 {
            cpu.set_gpr(i, tape.u32());
        }
        let mut memory = Memory {
            ram: tape.bytes[tape.at..tape.at + 65536].to_vec(),
            biu,
            writes: BASIS,
            reads: BASIS,
        };
        tape.at += 65536;
        let count = tape.u32();
        for step in 0..count {
            cpu.set_irq(tape.u32() != 0);
            cpu.step(&mut memory);
            macro_rules! check {
                ($value:expr) => {
                    assert_eq!(
                        $value as u32,
                        tape.u32(),
                        "case {}, step {}, {}",
                        case,
                        step,
                        stringify!($value)
                    );
                };
            }
            let s = &cpu.state;
            for &v in &s.gpr {
                check!(v);
            }
            for v in [
                s.hi,
                s.lo,
                s.pc,
                s.next_pc,
                s.bdbt,
                s.ld_which as u32,
                s.ld_value,
            ] {
                check!(v);
            }
            for &v in &s.cop0 {
                check!(v);
            }
            check!(s.branched);
            for value in [s.now, s.cost, s.muldiv_done, s.gte_done] {
                assert_eq!(value, tape.u64(), "time case {}, step {}", case, step);
            }
            for line in &s.icache {
                check!(line.tag);
                check!(line.valid);
            }
            for i in 0..64 {
                check!(cpu.gte.read(i));
            }
            for value in [memory.digest(), memory.writes, memory.reads] {
                assert_eq!(value, tape.u64(), "RAM/access case {}, step {}", case, step);
            }
            steps += 1;
        }
    }
    assert_eq!(tape.at, tape.bytes.len());
    assert_eq!(cases, 364);
    assert_eq!(steps, 5824);
    eprintln!("CPU_GO_ORACLE: {} cases, {} exact single-step CPU/GTE/cache/time states and RAM/access hashes",cases,steps);
}
