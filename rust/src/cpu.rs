//! Own R3000A port: specs 004, 008 and 026 §6, GPL-3.0-or-later.
//! Source: ps-golem internal/cpu/cpu.go, 4951d1e26d2fdde57d92c37ee91129616a5ec471.
//! Original reference: beetle-psx-libretro mednafen/psx/cpu.c,
//! e57860d82faa953dc0fc10f201fd583739bc2f7d, GPL-2.0-or-later.
use crate::gte::Gte;
pub trait Bus {
    fn read8(&mut self, addr: u32) -> u8;
    fn read16(&mut self, addr: u32) -> u16;
    fn read32(&mut self, addr: u32) -> u32;
    fn write8(&mut self, addr: u32, value: u32);
    fn write16(&mut self, addr: u32, value: u32);
    fn write32(&mut self, addr: u32, value: u32);
    fn isolated_write(&mut self, addr: u32, value: u32, size: usize);
    fn instruction_bus_error(&self, addr: u32) -> bool;
    fn read_cost(&self, addr: u32, size: usize) -> u32;
    fn biu(&self) -> u32;
}
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
pub struct CacheLine {
    pub tag: u32,
    pub valid: u8,
}
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct State {
    pub gpr: [u32; 35],
    pub hi: u32,
    pub lo: u32,
    pub pc: u32,
    pub next_pc: u32,
    pub bdbt: u32,
    pub ld_which: usize,
    pub ld_value: u32,
    pub cop0: [u32; 32],
    pub branched: bool,
    pub now: u64,
    pub cost: u64,
    pub muldiv_done: u64,
    pub gte_done: u64,
    pub icache: [CacheLine; 256],
}
impl Default for State {
    fn default() -> Self {
        Self {
            gpr: [0; 35],
            hi: 0,
            lo: 0,
            pc: 0xbfc00000,
            next_pc: 0xbfc00004,
            bdbt: 0,
            ld_which: 34,
            ld_value: 0,
            cop0: {
                let mut v = [0; 32];
                v[12] = 1 << 22 | 1 << 21;
                v[15] = 2;
                v
            },
            branched: false,
            now: 0,
            cost: 0,
            muldiv_done: 0,
            gte_done: 0,
            icache: [CacheLine::default(); 256],
        }
    }
}
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct Cpu {
    pub state: State,
    pub gte: Gte,
}
const MULT: [u64; 24] = [
    14, 14, 14, 14, 14, 14, 14, 14, 14, 14, 14, 14, 10, 10, 10, 10, 10, 10, 10, 10, 10, 7, 7, 7,
];
impl Cpu {
    pub fn new() -> Self {
        Self::default()
    }
    pub fn reset(&mut self) {
        *self = Self::default();
    }
    pub fn pc(&self) -> u32 {
        self.state.pc
    }
    pub fn gpr(&self, i: usize) -> u32 {
        if i == 0 {
            0
        } else {
            self.state.gpr[i]
        }
    }
    pub fn set_gpr(&mut self, i: usize, v: u32) {
        if i != 0 {
            self.state.gpr[i] = v;
        }
    }
    pub fn set_pc(&mut self, pc: u32) {
        self.state.pc = pc;
        self.state.next_pc = pc.wrapping_add(4);
        self.state.bdbt = 0;
        self.state.ld_which = 34;
    }
    pub fn set_irq(&mut self, active: bool) {
        if active {
            self.state.cop0[13] |= 1 << 10;
        } else {
            self.state.cop0[13] &= !(1 << 10);
        }
    }
    fn pending_irq(&self) -> bool {
        self.state.cop0[12] & self.state.cop0[13] & 0xff00 != 0 && self.state.cop0[12] & 1 != 0
    }
    fn lds(&mut self) {
        self.state.gpr[self.state.ld_which] = self.state.ld_value;
        self.state.ld_which = 34;
    }
    fn start_load(&mut self, rt: usize, v: u32) {
        self.state.ld_which = rt;
        self.state.ld_value = v;
    }
    fn cancel_load(&mut self, rt: usize) {
        if self.state.ld_which == rt {
            self.state.ld_which = 0;
        }
    }
    fn raise(&mut self, code: u32, instr: u32) {
        let s = &mut self.state;
        s.cop0[14] = s.pc;
        if s.bdbt & 2 != 0 {
            s.cop0[14] = s.cop0[14].wrapping_sub(4);
            s.cop0[6] = s.next_pc;
        }
        let sr = s.cop0[12];
        s.next_pc = if sr & (1 << 22) != 0 {
            0xbfc00180
        } else {
            0x80000080
        };
        s.cop0[12] = sr & !0x3f | ((sr << 2) & 0x3f);
        s.cop0[13] =
            s.cop0[13] & 0xff00 | (code << 2) | (s.bdbt << 30) | ((instr << 2) & (3 << 28));
        s.bdbt = 0;
    }
    fn address_error(&mut self, code: u32, addr: u32, instr: u32) {
        self.state.cop0[8] = addr;
        self.raise(code, instr);
    }
    fn branch(&mut self, cond: bool, offset: u32, mask: u32, link: usize, do_link: bool) {
        let s = &mut self.state;
        s.pc = s.next_pc;
        s.next_pc = s.next_pc.wrapping_add(4);
        s.bdbt = 2;
        if do_link {
            s.gpr[link] = s.next_pc;
        }
        if cond {
            s.next_pc = (s.next_pc.wrapping_sub(4) & mask).wrapping_add(offset);
            s.bdbt = 3;
        }
        s.branched = true;
    }
    fn advance(&mut self) {
        if !self.state.branched {
            self.state.pc = self.state.next_pc;
            self.state.next_pc = self.state.next_pc.wrapping_add(4);
            self.state.bdbt = 0;
        }
    }
    fn cache_cost(&mut self, pc: u32) -> u64 {
        let p = pc & 0x1fffffff;
        let line = &mut self.state.icache[((p >> 4) & 255) as usize];
        let word = (p >> 2) & 3;
        if line.tag == p >> 12 && line.valid & (1 << word) != 0 {
            return 0;
        }
        let fill = ((15 << word) & 15) as u8;
        if line.tag == p >> 12 {
            line.valid |= fill;
        } else {
            line.tag = p >> 12;
            line.valid = fill;
        }
        3 + (4 - word) as u64
    }
    fn wait(&mut self, t: u64) {
        if self.state.now + self.state.cost < t {
            self.state.cost = t - self.state.now;
        }
    }
    pub fn step<B: Bus>(&mut self, bus: &mut B) -> u32 {
        self.state.cost = 1;
        self.state.gpr[0] = 0;
        self.state.branched = false;
        let pc = self.state.pc;
        if pc & 3 != 0 {
            self.address_error(4, pc, 0);
            self.advance();
        } else if bus.instruction_bus_error(pc) {
            self.raise(6, 0);
            self.advance();
        } else {
            let fetch = if pc >> 29 == 5 || bus.biu() & 0x800 == 0 {
                bus.read_cost(pc, 4) as u64
            } else {
                self.cache_cost(pc)
            };
            self.state.cost += fetch;
            let instr = bus.read32(pc);
            if self.pending_irq() && instr >> 26 != 0x12 {
                self.lds();
                self.raise(0, instr);
            } else if instr == 0 {
                self.lds();
                self.state.gpr[0] = 0;
            } else {
                self.execute(bus, instr);
            }
            self.advance();
        }
        self.state.now += self.state.cost;
        self.state.cost as u32
    }
    fn read<B: Bus>(&mut self, bus: &mut B, addr: u32, size: usize) -> u32 {
        self.state.cost += bus.read_cost(addr, size) as u64;
        match size {
            1 => bus.read8(addr) as u32,
            2 => bus.read16(addr) as u32,
            _ => bus.read32(addr),
        }
    }
    fn write<B: Bus>(&mut self, bus: &mut B, addr: u32, v: u32, size: usize) {
        if self.state.cop0[12] & (1 << 16) != 0 {
            let biu = bus.biu();
            if biu & 0x800 != 0 && biu & 7 != 0 {
                self.state.icache[((addr >> 4) & 255) as usize].valid = 0;
            }
            bus.isolated_write(addr, v, size);
            return;
        }
        match size {
            1 => bus.write8(addr, v),
            2 => bus.write16(addr, v),
            _ => bus.write32(addr, v),
        }
    }
    fn merge_base(&mut self, rt: usize) -> u32 {
        if self.state.ld_which == rt {
            return self.state.ld_value;
        }
        let v = self.state.gpr[rt];
        self.lds();
        v
    }
    fn usable(&self, n: u32) -> bool {
        self.state.cop0[12] & (1 << (28 + n)) != 0
    }
    fn execute<B: Bus>(&mut self, bus: &mut B, instr: u32) {
        let op = instr >> 26;
        let rs = ((instr >> 21) & 31) as usize;
        let rt = ((instr >> 16) & 31) as usize;
        let rd = ((instr >> 11) & 31) as usize;
        let sh = (instr >> 6) & 31;
        let imm = instr as i16 as i32 as u32;
        let ze = instr & 0xffff;
        // Read all source values before committing the preceding delayed load.
        let a = self.state.gpr[rs];
        let b = self.state.gpr[rt];
        if op == 0 {
            match instr & 63 {
                0 | 2 | 3 | 4 | 6 | 7 => {
                    let n = if instr & 4 != 0 { a & 31 } else { sh };
                    let v = match instr & 3 {
                        0 => b << n,
                        2 => b >> n,
                        _ => ((b as i32) >> n) as u32,
                    };
                    self.lds();
                    self.state.gpr[rd] = v;
                }
                8 | 9 => {
                    self.lds();
                    self.branch(true, a, 0, rd, instr & 63 == 9);
                }
                0x0c | 0x0d => {
                    self.lds();
                    self.raise(if instr & 63 == 0x0c { 8 } else { 9 }, instr);
                }
                0x10 | 0x12 => {
                    self.lds();
                    self.wait(self.state.muldiv_done);
                    self.state.gpr[rd] = if instr & 63 == 0x10 {
                        self.state.hi
                    } else {
                        self.state.lo
                    };
                }
                0x11 | 0x13 => {
                    if instr & 63 == 0x11 {
                        self.state.hi = a;
                    } else {
                        self.state.lo = a;
                    }
                    self.lds();
                }
                0x18 | 0x19 => {
                    let signed = instr & 63 == 0x18;
                    let key = if signed {
                        a ^ ((a as i32 >> 31) as u32)
                    } else {
                        a
                    };
                    self.state.muldiv_done = self.state.now
                        + self.state.cost
                        + MULT[(key | 0x400).leading_zeros() as usize];
                    let v = if signed {
                        ((a as i32 as i64) * (b as i32 as i64)) as u64
                    } else {
                        a as u64 * b as u64
                    };
                    self.lds();
                    self.state.lo = v as u32;
                    self.state.hi = (v >> 32) as u32;
                }
                0x1a | 0x1b => {
                    self.state.muldiv_done = self.state.now + self.state.cost + 37;
                    if b == 0 {
                        self.state.lo = if instr & 63 == 0x1a && a & 0x80000000 != 0 {
                            1
                        } else {
                            !0
                        };
                        self.state.hi = a;
                    } else if instr & 63 == 0x1a {
                        if a == 0x80000000 && b == !0 {
                            self.state.lo = a;
                            self.state.hi = 0;
                        } else {
                            self.state.lo = ((a as i32) / (b as i32)) as u32;
                            self.state.hi = ((a as i32) % (b as i32)) as u32;
                        }
                    } else {
                        self.state.lo = a / b;
                        self.state.hi = a % b;
                    }
                    self.lds();
                }
                0x20..=0x27 | 0x2a | 0x2b => {
                    let fun = instr & 63;
                    let v = match fun {
                        0x20 | 0x21 => a.wrapping_add(b),
                        0x22 | 0x23 => a.wrapping_sub(b),
                        0x24 => a & b,
                        0x25 => a | b,
                        0x26 => a ^ b,
                        0x27 => !(a | b),
                        0x2a => ((a as i32) < (b as i32)) as u32,
                        _ => (a < b) as u32,
                    };
                    let overflow = match fun {
                        0x20 => (!(a ^ b)) & (a ^ v) & 0x80000000 != 0,
                        0x22 => (a ^ b) & (a ^ v) & 0x80000000 != 0,
                        _ => false,
                    };
                    self.lds();
                    if overflow {
                        self.raise(12, instr);
                    } else {
                        self.state.gpr[rd] = v;
                    }
                }
                _ => {
                    self.lds();
                    self.raise(10, instr);
                }
            }
            return;
        }
        match op {
            1 => {
                let cond = ((a ^ ((rt as u32) << 31)) as i32) < 0;
                let link = if rt & 0x1e == 0x10 { 31 } else { 0 };
                self.lds();
                self.branch(cond, imm << 2, !0, link, true);
            }
            2 | 3 => {
                self.lds();
                self.branch(true, (instr & 0x3ffffff) << 2, 0xf0000000, 31, op == 3);
            }
            4..=7 => {
                let cond = match op {
                    4 => a == b,
                    5 => a != b,
                    6 => (a as i32) <= 0,
                    _ => (a as i32) > 0,
                };
                self.lds();
                self.branch(cond, imm << 2, !0, 0, false);
            }
            8..=15 => {
                let v = match op {
                    8 | 9 => a.wrapping_add(imm),
                    10 => ((a as i32) < (imm as i32)) as u32,
                    11 => (a < imm) as u32,
                    12 => a & ze,
                    13 => a | ze,
                    14 => a ^ ze,
                    _ => ze << 16,
                };
                let overflow = op == 8 && (!(a ^ imm)) & (a ^ v) & 0x80000000 != 0;
                self.lds();
                if overflow {
                    self.raise(12, instr);
                } else {
                    self.state.gpr[rt] = v;
                }
            }
            0x10 => self.cop0(instr),
            0x12 => self.cop2(instr),
            0x11 | 0x13 => {
                self.lds();
                if !self.usable(op & 3) {
                    self.raise(11, instr);
                } else if [8, 12].contains(&((instr >> 21) & 31)) {
                    self.branch(instr & (1 << 16) == 0, imm << 2, !0, 0, false);
                }
            }
            0x20 | 0x21 | 0x23 | 0x24 | 0x25 => {
                let addr = a.wrapping_add(imm);
                let size = match op {
                    0x20 | 0x24 => 1,
                    0x21 | 0x25 => 2,
                    _ => 4,
                };
                if addr & (size as u32 - 1) != 0 {
                    self.lds();
                    self.address_error(4, addr, instr);
                    return;
                }
                self.cancel_load(rt);
                self.lds();
                let mut v = self.read(bus, addr, size);
                if op == 0x20 {
                    v = v as i8 as i32 as u32;
                } else if op == 0x21 {
                    v = v as i16 as i32 as u32;
                }
                self.start_load(rt, v);
            }
            0x22 | 0x26 => {
                let addr = a.wrapping_add(imm);
                let base = self.merge_base(rt);
                let n = (addr & 3) as usize;
                let size = if op == 0x22 {
                    [1, 2, 4, 4][n]
                } else {
                    [4, 4, 2, 1][n]
                };
                self.state.cost +=
                    bus.read_cost(if op == 0x22 { addr & !3 } else { addr }, size) as u64;
                let w = bus.read32(addr & !3);
                let v = if op == 0x22 {
                    match n {
                        0 => base & 0x00ffffff | w << 24,
                        1 => base & 0x0000ffff | w << 16,
                        2 => base & 0xff | w << 8,
                        _ => w,
                    }
                } else {
                    match n {
                        0 => w,
                        1 => base & 0xff000000 | w >> 8,
                        2 => base & 0xffff0000 | w >> 16,
                        _ => base & 0xffffff00 | w >> 24,
                    }
                };
                self.start_load(rt, v);
            }
            0x28 | 0x29 | 0x2b => {
                let addr = a.wrapping_add(imm);
                let size = match op {
                    0x28 => 1,
                    0x29 => 2,
                    _ => 4,
                };
                if addr & (size as u32 - 1) != 0 {
                    self.address_error(5, addr, instr);
                } else {
                    self.write(bus, addr, b, size);
                }
                self.lds();
            }
            0x2a | 0x2e => {
                let addr = a.wrapping_add(imm);
                let aligned = addr & !3;
                let w = self.read(bus, aligned, 4);
                let n = addr & 3;
                let value = if op == 0x2a {
                    match n {
                        0 => w & 0xffffff00 | b >> 24,
                        1 => w & 0xffff0000 | b >> 16,
                        2 => w & 0xff000000 | b >> 8,
                        _ => b,
                    }
                } else {
                    match n {
                        0 => b,
                        1 => w & 0xff | b << 8,
                        2 => w & 0xffff | b << 16,
                        _ => w & 0x00ffffff | b << 24,
                    }
                };
                self.write(bus, aligned, value, 4);
                self.lds();
            }
            0x30 | 0x31 | 0x33 => {
                let addr = a.wrapping_add(imm);
                self.lds();
                if !self.usable(op & 3) {
                    self.raise(11, instr);
                } else if addr & 3 != 0 {
                    self.address_error(4, addr, instr);
                } else {
                    self.read(bus, addr, 4);
                }
            }
            0x32 => {
                let addr = a.wrapping_add(imm);
                self.lds();
                if !self.usable(2) {
                    self.raise(11, instr);
                } else if addr & 3 != 0 {
                    self.address_error(4, addr, instr);
                } else {
                    self.wait(self.state.gte_done);
                    let v = self.read(bus, addr, 4);
                    self.gte.write(rt, v);
                }
            }
            0x38 | 0x39 | 0x3b => {
                let addr = a.wrapping_add(imm);
                self.lds();
                if !self.usable(op & 3) {
                    self.raise(11, instr);
                } else if addr & 3 != 0 {
                    self.address_error(5, addr, instr);
                }
            }
            0x3a => {
                let addr = a.wrapping_add(imm);
                if !self.usable(2) {
                    self.raise(11, instr);
                } else if addr & 3 != 0 {
                    self.address_error(5, addr, instr);
                } else {
                    self.wait(self.state.gte_done);
                    let v = self.gte.read(rt);
                    self.write(bus, addr, v, 4);
                }
                self.lds();
            }
            _ => {
                self.lds();
                self.raise(10, instr);
            }
        }
    }
    fn cop0(&mut self, instr: u32) {
        let sub = (instr >> 21) & 31;
        let rt = ((instr >> 16) & 31) as usize;
        let rd = ((instr >> 11) & 31) as usize;
        let value = self.state.gpr[rt];
        match sub {
            0 => {
                if [0, 1, 2, 4, 10].contains(&rd) {
                    self.lds();
                    self.raise(10, instr);
                } else if [3, 5, 6, 7, 8, 9, 11, 12, 13, 14, 15].contains(&rd) {
                    self.cancel_load(rt);
                    self.lds();
                    self.start_load(rt, self.state.cop0[rd]);
                } else {
                    self.lds();
                }
            }
            4 => {
                self.lds();
                match rd {
                    0 | 1 | 2 | 4 | 10 => self.raise(10, instr),
                    3 | 5 | 9 | 11 => self.state.cop0[rd] = value,
                    7 => self.state.cop0[rd] = value & 0xff80003f,
                    13 => self.state.cop0[13] = self.state.cop0[13] & !(3 << 8) | value & (3 << 8),
                    12 => self.state.cop0[12] = value & !(3 << 26 | 3 << 23 | 3 << 6),
                    _ => {}
                }
            }
            2 | 6 => {
                self.lds();
                self.raise(10, instr);
            }
            8 | 12 => {
                self.lds();
                self.branch(
                    instr & (1 << 16) == 0,
                    (instr as i16 as i32 as u32) << 2,
                    !0,
                    0,
                    false,
                );
            }
            16..=31 => {
                self.lds();
                match instr & 31 {
                    16 => {
                        let sr = self.state.cop0[12];
                        self.state.cop0[12] = sr & !15 | (sr >> 2) & 15;
                    }
                    1 | 2 | 6 | 8 => self.raise(10, instr),
                    _ => {}
                }
            }
            _ => self.lds(),
        }
    }
    fn cop2(&mut self, instr: u32) {
        let sub = (instr >> 21) & 31;
        let rt = ((instr >> 16) & 31) as usize;
        let rd = ((instr >> 11) & 31) as usize;
        let value = self.state.gpr[rt];
        if !self.usable(2) {
            self.lds();
            self.raise(11, instr);
            return;
        }
        if sub != 8 && sub != 12 {
            self.wait(self.state.gte_done);
        }
        match sub {
            0 | 2 => {
                self.cancel_load(rt);
                self.lds();
                self.start_load(rt, self.gte.read(rd + if sub == 2 { 32 } else { 0 }));
            }
            4 | 6 => {
                self.lds();
                self.gte.write(rd + if sub == 6 { 32 } else { 0 }, value);
            }
            8 | 12 => {
                self.lds();
                self.branch(
                    instr & (1 << 16) == 0,
                    (instr as i16 as i32 as u32) << 2,
                    !0,
                    0,
                    false,
                );
            }
            16..=31 => {
                self.lds();
                let n = self.gte.execute(instr);
                self.state.gte_done = self.state.now + self.state.cost + n as u64 - 1;
            }
            _ => self.lds(),
        }
    }
}
#[cfg(test)]
mod tests;
