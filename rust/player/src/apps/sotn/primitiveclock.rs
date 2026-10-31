//! SotN primitive resource clock, 026 §16.8.2.2, GPL-3.0-or-later.
//! Own callclock.rs cache contract at 55851c1. Resource slots, not an ISA interpreter.
use super::primitives::DataPlan;
use psgolem_core::{
    cpu::{Bus, State},
    machine::{phase::RamWordPatch, Machine},
};
use sha2::{Digest, Sha256};
type Result<T> = std::result::Result<T, String>;
fn half(r: &[u8], p: usize) -> u16 {
    u16::from_le_bytes(r[p..p + 2].try_into().unwrap())
}
fn word(r: &[u8], p: usize) -> u32 {
    u32::from_le_bytes(r[p..p + 4].try_into().unwrap())
}
struct Clock {
    state: State,
    costs: Vec<u32>,
}
impl Clock {
    fn block(&mut self, first: u32, last: u32) {
        for id in (first..=last).step_by(4) {
            let p = id & 0x1fffffff;
            let slot = (p >> 2) & 3;
            let line = &mut self.state.icache[((p >> 4) & 255) as usize];
            let mut cost = 1;
            if line.tag != p >> 12 || line.valid & (1 << slot) == 0 {
                let fill = ((15 << slot) & 15) as u8;
                line.valid = if line.tag == p >> 12 {
                    line.valid | fill
                } else {
                    fill
                };
                line.tag = p >> 12;
                cost += 7 - slot;
            }
            if RAM_READS.contains(&id) {
                cost += 4;
            }
            self.state.now += cost as u64;
            self.state.cost = cost as u64;
            self.costs.push(cost);
        }
    }
    fn environment(&mut self) {
        // Standard GPU, in-range clip, dither off, display drawing on, no fill.
        for (a, b) in [
            (0x80013648, 0x8001366c),
            (0x80013950, 0x80013968),
            (0x8001397c, 0x800139a0),
            (0x800139b4, 0x800139bc),
            (0x800139e0, 0x80013a00),
            (0x80013a0c, 0x80013a18),
            (0x80013670, 0x800136a4),
            (0x80013a1c, 0x80013a34),
            (0x80013a48, 0x80013a6c),
            (0x80013a80, 0x80013a88),
            (0x80013aac, 0x80013acc),
            (0x80013ad8, 0x80013ae4),
            (0x800136a8, 0x800136b8),
            (0x80013ae8, 0x80013b08),
            (0x80013b14, 0x80013b20),
            (0x800136bc, 0x800136d0),
            (0x80013900, 0x80013910),
            (0x80013930, 0x80013934),
            (0x8001393c, 0x8001394c),
            (0x800136d4, 0x800136dc),
            (0x80013b24, 0x80013b28),
            (0x80013b34, 0x80013ba4),
            (0x800136e0, 0x800136f8),
            (0x800138e0, 0x800138fc),
        ] {
            self.block(a, b);
        }
    }
}
pub struct PrimitiveClockPlan {
    pub data: DataPlan,
    endpoint: State,
    costs: Vec<u32>,
    before: State,
    ram_sha: [u8; 32],
    scratch: Vec<u8>,
    biu: u32,
}
impl PrimitiveClockPlan {
    pub fn prepare(r: &[u8], scratch: &[u8], entry: &State, biu: u32) -> Result<Self> {
        if entry.next_pc != entry.pc.wrapping_add(4)
            || entry.bdbt != 0
            || entry.branched
            || entry.ld_which != 34
            || entry.gpr[0] != 0
            || biu != 125320
            || entry.cop0[12] & (1 << 16) != 0
            || entry.cop0[12] & entry.cop0[13] & 0xff00 != 0 && entry.cop0[12] & 1 != 0
            || entry.icache.iter().any(|c| c.valid & !15 != 0)
            || entry.now > u64::MAX - 2_000_000
        {
            return Err("圖元CPU／cache入口不支援".into());
        }
        let data = DataPlan::prepare(r, scratch, entry)?;
        let b = (word(r, 0x6c37c) - 0x80000000) as usize;
        let env = &r[b + 4..b + 4 + 92];
        let x = half(env, 0) as i16 as i32;
        let y = half(env, 2) as i16 as i32;
        let ex = half(env, 0).wrapping_add(half(env, 4)).wrapping_sub(1) as i16 as i32;
        let ey = half(env, 2).wrapping_add(half(env, 6)).wrapping_sub(1) as i16 as i32;
        if !(0..1024).contains(&x)
            || !(0..512).contains(&y)
            || !(0..1024).contains(&ex)
            || !(0..512).contains(&ey)
            || env[22] != 0
            || env[23] == 0
        {
            return Err("圖元環境時鐘分支未封閉".into());
        }
        let mut c = Clock {
            state: entry.clone(),
            costs: Vec::with_capacity(20000),
        };
        c.block(0x800edc94, 0x800ede48);
        c.block(0x800edf58, 0x800edf70);
        let (mut last_visible, mut last_nonzero) = (None, None);
        for i in 0..1280 {
            let p = 0x86fec + i * 52;
            let kind = r[p + 7];
            c.block(0x800edf74, 0x800edf80);
            if kind != 0 {
                let flags = half(r, p + 50);
                last_nonzero = Some((i, flags));
                c.block(0x800edf84, 0x800edf94);
                if flags & 8 == 0 {
                    if !(-512..=512).contains(&(half(r, p + 8) as i16 as i32))
                        || !(-512..=512).contains(&(half(r, p + 10) as i16 as i32))
                    {
                        return Err("圖元裁掉時鐘分支未封閉".into());
                    }
                    last_visible = Some((i, kind));
                    c.block(0x800edf98, 0x800edfa8);
                    c.block(0x800edfb8, 0x800edff4);
                    c.block(0x800ee090, 0x800ee0b8);
                    match kind {
                        4 => {
                            c.block(0x800ee744, 0x800ee774);
                            c.block(0x800ee77c, 0x800ee78c);
                            c.block(0x800ee79c, 0x800ee8a0);
                            c.block(0x800ee930, 0x800ee9c0);
                            for _ in 0..3 {
                                c.block(0x800ee9c4, 0x800ee9ec);
                            }
                            c.block(0x800ee9f0, 0x800eea70);
                            c.block(0x800ef1c0, 0x800ef1c4);
                        }
                        7 => {
                            c.block(0x800ef018, 0x800ef040);
                            if flags & 0x800 != 0 {
                                c.block(0x800ef064, 0x800ef088);
                                for _ in 0..5 {
                                    c.block(0x800ef08c, 0x800ef0b4);
                                }
                                c.block(0x800ef0b8, 0x800ef0dc);
                                c.environment();
                            } else {
                                c.block(0x800ef064, 0x800ef074);
                            }
                            c.block(0x800ef0e0, 0x800ef0e8);
                            for _ in 0..4 {
                                c.block(0x800ef0ec, 0x800ef114);
                            }
                            c.block(0x800ef118, 0x800ef128);
                            c.block(0x800ef14c, 0x800ef1c4);
                        }
                        _ => return Err("未列圖元資源分支".into()),
                    }
                }
            }
            c.block(0x800ef1c8, 0x800ef1dc);
        }
        c.block(0x800ef1e0, 0x800ef210);
        let (last, kind) = last_visible.ok_or("缺少末圖元")?;
        if kind != 4 {
            return Err("末環境CPU端點未封閉".into());
        }
        let (nonzero, flags) = last_nonzero.unwrap();
        let ci = half(r, 0x86fec + last * 52 + 14) as u32;
        for (reg, value) in [
            (0, 0),
            (1, 0x80040000 + ci * 2),
            (2, 0),
            (3, word(&data.ram, 0x97930)),
            (
                4,
                if nonzero > last {
                    flags as u32
                } else {
                    0x80097930
                },
            ),
            (5, 0),
            (6, word(&data.scratch, 4) - 4),
            (7, 0x1f800054),
            (8, 0x1f800054),
            (9, 1280),
            (34, entry.gpr[16]),
        ] {
            c.state.gpr[reg] = value;
        }
        c.state.pc = entry.gpr[31];
        c.state.next_pc = entry.gpr[31] + 4;
        c.state.bdbt = 0;
        c.state.branched = false;
        c.state.ld_which = 34;
        c.state.ld_value = entry.gpr[16];
        Ok(Self {
            data,
            endpoint: c.state,
            costs: c.costs,
            before: entry.clone(),
            ram_sha: Sha256::digest(r).into(),
            scratch: scratch.to_vec(),
            biu,
        })
    }
    pub fn endpoint(&self) -> &State {
        &self.endpoint
    }
    pub fn costs(&self) -> &[u32] {
        &self.costs
    }
    pub fn commit(&self, m: &mut Machine) -> Result<()> {
        if m.cpu.state != self.before
            || m.bus.biu() != self.biu
            || m.bus.scratch() != self.scratch.as_slice()
            || <[u8; 32]>::from(Sha256::digest(m.bus.ram())) != self.ram_sha
        {
            return Err("圖元時鐘計畫已過期".into());
        }
        let patches: Vec<_> = m
            .bus
            .ram()
            .chunks_exact(4)
            .zip(self.data.ram.chunks_exact(4))
            .enumerate()
            .filter(|(_, (a, b))| a != b)
            .map(|(i, (_, b))| RamWordPatch {
                offset: (i * 4) as u32,
                value: u32::from_le_bytes(b.try_into().unwrap()),
            })
            .collect();
        m.commit_cpu_data_phase(
            &self.endpoint,
            &patches,
            &self.data.scratch,
            &self.costs,
            100000,
        )
    }
}

const RAM_READS: &[u32] = &[
    0x80013660, 0x80013664, 0x80013674, 0x80013678, 0x8001367c, 0x8001368c, 0x800136ac, 0x800136b0,
    0x800136c0, 0x800136c4, 0x800136c8, 0x800136ec, 0x800138e8, 0x800138ec, 0x800138f0, 0x80013904,
    0x80013994, 0x800139e4, 0x80013a60, 0x80013ab0, 0x80013aec, 0x80013b34, 0x80013b44, 0x80013b5c,
    0x80013b70, 0x800edcc8, 0x800edcf8, 0x800edd3c, 0x800edd44, 0x800edd6c, 0x800edd94, 0x800eddcc,
    0x800eddf0, 0x800ede18, 0x800ede2c, 0x800edf74, 0x800edf84, 0x800edf9c, 0x800edfb8, 0x800edfd4,
    0x800ee0ac, 0x800ee750, 0x800ee764, 0x800ee7a0, 0x800ee7ac, 0x800ee7b8, 0x800ee7c4, 0x800ee7d0,
    0x800ee7dc, 0x800ee7e8, 0x800ee7f4, 0x800ee800, 0x800ee80c, 0x800ee818, 0x800ee824, 0x800ee830,
    0x800ee844, 0x800ee850, 0x800ee85c, 0x800ee868, 0x800ee874, 0x800ee880, 0x800ee88c, 0x800ee898,
    0x800ee930, 0x800ee93c, 0x800ee948, 0x800ee954, 0x800ee960, 0x800ee96c, 0x800ee978, 0x800ee984,
    0x800ee990, 0x800ee994, 0x800ee9a4, 0x800ee9b8, 0x800ee9fc, 0x800eea10, 0x800eea14, 0x800eea28,
    0x800eea38, 0x800eea5c, 0x800ef01c, 0x800ef030, 0x800ef034, 0x800ef064, 0x800ef07c, 0x800ef08c,
    0x800ef090, 0x800ef094, 0x800ef098, 0x800ef0b8, 0x800ef0bc, 0x800ef0c0, 0x800ef0ec, 0x800ef0f0,
    0x800ef0f4, 0x800ef0f8, 0x800ef118, 0x800ef14c, 0x800ef160, 0x800ef164, 0x800ef178, 0x800ef188,
    0x800ef1ac, 0x800ef1c8, 0x800ef1e0, 0x800ef1e4, 0x800ef1e8, 0x800ef1ec, 0x800ef1f0, 0x800ef1f4,
    0x800ef1f8, 0x800ef1fc, 0x800ef200, 0x800ef204,
];
