//! Ordinary NO3 scene-call compatibility data, spec026 §16.5.
//! Own Go hd_call_plan/effects/clock.go at 1f142c7, GPL-3.0-or-later.
//! No guest instructions, GP0 decoding, raster replay or player registration.
//! HD pixels come from scene::build, never the compatibility packets below.
use psgolem_core::cpu::Bus;
use psgolem_core::{
    cpu::State,
    machine::{phase::RamWordPatch, Machine},
};
use sha2::{Digest, Sha256};
type Result<T> = std::result::Result<T, String>;
fn sha(b: &[u8]) -> [u8; 32] {
    Sha256::digest(b).into()
}
fn hash(b: &[u8], expected: &str) -> bool {
    format!("{:x}", Sha256::digest(b)) == expected
}
fn word(r: &[u8], i: usize) -> u32 {
    u32::from_le_bytes(r[i..i + 4].try_into().unwrap())
}
fn half(r: &[u8], i: usize) -> u16 {
    u16::from_le_bytes(r[i..i + 2].try_into().unwrap())
}
fn put(r: &mut [u8], i: usize, v: u32) {
    r[i..i + 4].copy_from_slice(&v.to_le_bytes());
}
fn put16(r: &mut [u8], i: usize, v: u16) {
    r[i..i + 2].copy_from_slice(&v.to_le_bytes());
}
fn source(a: u32, n: usize, align: u32) -> Result<usize> {
    if a < 0x8018_0000 || a & (align - 1) != 0 || a as u64 + n as u64 > 0x8020_0000 {
        return Err("NO3資料來源超界或未對齊".into());
    }
    Ok((a - 0x8000_0000) as usize)
}
pub(super) struct Layer {
    pub(super) ids: Vec<u16>,
    sx: usize,
    sy: usize,
    cols: usize,
    rows: usize,
    order: usize,
    data: usize,
    tables: [usize; 3],
}
impl Layer {
    fn read(r: &[u8], b: usize) -> Result<Self> {
        let sx = half(r, b + 10) as i16;
        let sy = half(r, b + 14) as i16;
        let cols = word(r, b + 32) as u64 * 16;
        let rows = word(r, b + 36) as u64 * 16;
        let order = half(r, b + 24) as usize;
        if word(r, b + 28) != 3
            || word(r, b + 40) != 0
            || sx < 0
            || sy < 0
            || cols == 0
            || rows == 0
            || cols > 65536
            || rows > 65536
            || cols * rows > 65536
            || order + 24 > 512
            || sx as u64 / 16 + 17 > cols
            || sy as u64 / 16 + 16 > rows
            || sy as u64 / 16 + 15 >= 256
        {
            return Err("NO3圖層視窗或分支不在已驗範圍".into());
        }
        let data = source(word(r, b), (cols * rows * 2) as usize, 2)?;
        let def = source(word(r, b + 4), 16, 4)?;
        let tables = [
            source(word(r, def), 4096, 1)?,
            source(word(r, def + 4), 4096, 1)?,
            source(word(r, def + 8), 4096, 1)?,
        ];
        let (sx, sy, cols, rows) = (sx as usize, sy as usize, cols as usize, rows as usize);
        let mut ids = Vec::with_capacity(272);
        for y in sy / 16..sy / 16 + 16 {
            for x in sx / 16..sx / 16 + 17 {
                let id = half(r, data + (y * cols + x) * 2);
                if id >= 4096 || id != 0 && r[tables[0] + id as usize] > 7 {
                    return Err("未知圖塊或材質頁".into());
                }
                ids.push(id);
            }
        }
        Ok(Self {
            ids,
            sx,
            sy,
            cols,
            rows,
            order,
            data,
            tables,
        })
    }
}
pub struct CallPlan {
    before: State,
    biu: u32,
    ram_hash: [u8; 32],
    scratch_hash: [u8; 32],
    endpoint: State,
    patches: Vec<RamWordPatch>,
    scratch: Vec<u8>,
    costs: Vec<u32>,
}
impl CallPlan {
    pub fn prepare(r: &[u8], scratch: &[u8], state: &State, biu: u32) -> Result<Self> {
        if r.len() != 2 << 20
            || scratch.len() != 1024
            || state.pc != 0x800e_cc10
            || state.next_pc != 0x800e_cc14
            || state.gpr[31] != 0x800e_3c2c
            || state.gpr[29] != 0x801f_ffc0
            || state.ld_which != 34
            || state.bdbt != 0
            || state.branched
            || state.gpr[0] != 0
            || biu != 125320
            || state.cop0[12] & 0x10000 != 0
            || state.cop0[12] & state.cop0[13] & 0xff00 != 0 && state.cop0[12] & 1 != 0
            || state.icache.iter().any(|l| l.valid & !15 != 0)
        {
            return Err("NO3呼叫上下文不在已驗範圍".into());
        }
        for (a, n, h) in [
            (
                0xecc10,
                0x91c,
                "40dfd40f6a9922dd5ed1b69f1fd32ada3f7a8139b793984f49d373d3f31b797c",
            ),
            (
                0x11f18,
                60,
                "1c2e33c9e0ddfcd56385916112089fab3d7d343336e2514633a70260ef7d4064",
            ),
            (
                0x135f0,
                1464,
                "bbae27e62769d757261709c0c55190c9d7ea6098ee3efd5cbb8f83c2f1636615",
            ),
            (
                0x180000,
                36,
                "0a19676621ba9dfaad1a39a2ab6ead6fd0438fad85a57c91be011b4f9fe0cd82",
            ),
        ] {
            if !hash(&r[a..a + n], h) {
                return Err("NO3來源版本不符".into());
            }
        }
        let mut layout = Vec::new();
        for a in [
            0x73084, 0x73088, 0x730a4, 0x730a8, 0x730b0, 0x730b4, 0x730d8, 0x730dc, 0x730f8,
            0x730fc, 0x73104,
        ] {
            layout.extend_from_slice(&r[a..a + 4]);
        }
        if !hash(
            &layout,
            "4281dfee0f618f53bf31bfb1a0e30b2b66f57eebb58ce4db0c84b0b4483958fe",
        ) || ![0x8003cb08, 0x800542fc].contains(&word(r, 0x6c37c))
            || [0x97940, 0x9792c, 0x730ac, 0x2c26c]
                .iter()
                .any(|&a| word(r, a) != 0)
            || half(r, 0x6c39c) != 0
            || half(r, 0x6c3a0) != 0
        {
            return Err("NO3固定配置、計數或攝影偏移不符".into());
        }
        for i in 1..16 {
            if word(r, 0x730d8 + i * 48 + 28) != 0 || word(r, 0x730d8 + i * 48 + 40) != 0 {
                return Err("其他背景圖層尚未支援".into());
            }
        }
        let layers = [Layer::read(r, 0x73084)?, Layer::read(r, 0x730d8)?];
        let tiles = layers
            .iter()
            .flat_map(|l| &l.ids)
            .filter(|&&id| id != 0)
            .count();
        if tiles == 0 || tiles > 640 || !layers[1].ids.iter().any(|&id| id != 0) {
            return Err("空背景或圖塊預算分支尚未支援".into());
        }
        let mut out = r.to_vec();
        let mut sc = scratch.to_vec();
        let buffer = (word(r, 0x6c37c) - 0x8000_0000) as usize;
        let ot = buffer + 0x474;
        let mut sprites = 0usize;
        let mut modes = 0usize;
        let mut last = (0u32, 0u32, 0u32, 0u32, 0u32);
        for l in &layers {
            for (i, &id) in l.ids.iter().enumerate() {
                if id == 0 {
                    continue;
                }
                let id = id as usize;
                let (page, gfx, pal) = (
                    r[l.tables[0] + id] as u32,
                    r[l.tables[1] + id] as u32,
                    r[l.tables[2] + id] as u32,
                );
                let x = (i % 17 * 16) as i32 - (l.sx & 15) as i32;
                let y = (i / 17 * 16) as i32 - (l.sy & 15) as i32;
                let xy = x as u16 as u32 | (y as u16 as u32) << 16;
                let uv = (gfx & 15) * 16
                    | (gfx >> 4) * 16 << 8
                    | (half(r, 0x3c104 + pal as usize * 2) as u32) << 16;
                let packet = buffer + 0x117f4 + sprites * 16;
                let bucket = ot + (l.order + page as usize) * 4;
                let old = word(&out, bucket);
                put(&mut out, packet, 0x03000000 | old & 0xffffff);
                put(&mut out, packet + 4, 0x7d000000);
                put(&mut out, packet + 8, xy);
                put(&mut out, packet + 12, uv);
                put(
                    &mut out,
                    bucket,
                    old & 0xff000000 | ((packet as u32 + 0x80000000) & 0xffffff),
                );
                sprites += 1;
                last = (page, gfx, pal, xy, uv);
            }
            for page in 0..24 {
                let packet = buffer + 0xc74 + modes * 12;
                let bucket = ot + (l.order + page) * 4;
                let old = word(&out, bucket);
                put(&mut out, packet, 0x02000000 | old & 0xffffff);
                put(&mut out, packet + 4, 0xe1000000 + page as u32 + 8);
                put(&mut out, packet + 8, 0xe2000000);
                put(
                    &mut out,
                    bucket,
                    old & 0xff000000 | ((packet as u32 + 0x80000000) & 0xffffff),
                );
                modes += 1;
            }
        }
        put(&mut out, 0x97940, sprites as u32);
        put(&mut out, 0x9792c, modes as u32);
        let bg = &layers[1];
        let last_row = bg.sy / 16 + 15;
        let last_col = bg.sx / 16 + 16;
        sc[3] = 3;
        sc[7] = 0x7d;
        put(&mut sc, 8, last.3);
        put(&mut sc, 12, last.4);
        for (a, v) in [
            (0x10, 0),
            (0x12, 0),
            (0x14, 0),
            (0x16, 0),
            (0x18, bg.order),
            (0x1a, 0),
            (0x1c, 0),
            (0x1e, bg.cols),
            (0x20, bg.rows),
            (0x40, 0),
            (0x42, 0),
            (0x44, 255),
            (0x46, 255),
        ] {
            put16(&mut sc, a, v as u16);
        }
        for (a, v) in [
            (
                0x24,
                0x80000000 + bg.data + (last_row * bg.cols + last_col + 1) * 2,
            ),
            (0x28, 0x80000000 + bg.tables[0]),
            (0x2c, 0x80000000 + bg.tables[1]),
            (0x30, 0x80000000 + bg.tables[2]),
            (0x34, 0x80000000 + ot),
            (0x38, 0x80000000 + buffer + 0x117f4 + sprites * 16),
            (0x3c, 0x80000000 + buffer + 0xc74 + modes * 12),
        ] {
            put(&mut sc, a, v as u32);
        }
        let sp = (state.gpr[29] - 0x80000000) as usize;
        let frame = sp - 0x48;
        for (i, reg) in [16, 17, 18, 19, 20, 21, 22, 23, 30, 31].iter().enumerate() {
            put(&mut out, frame + 0x20 + i * 4, state.gpr[*reg]);
        }
        put(&mut out, frame + 24, (bg.sx & 15) as u32);
        put(&mut out, frame + 16, 0x1f800040);
        put(&mut out, sp - 0x58, 0x1f800010);
        put(&mut out, sp - 0x54, 23);
        put(&mut out, sp - 0x50, 0x800ed4a4);
        for n in 0..4 {
            put(&mut out, sp - 0x78 + n * 4, 0);
        }
        let clock = super::callclock::Clock::prepare(state, &layers)?;
        let mut endpoint = clock.state;
        let g = &mut endpoint.gpr;
        for (reg, v) in [
            (0, 0),
            (1, 0x1f800000),
            (2, 0),
            (3, 0),
            (4, (0x80000000 + ot + (bg.order + 23) * 4) as u32),
            (
                5,
                ((0x80000000 + buffer + 0xc74 + (modes - 1) * 12) & 0xffffff) as u32,
            ),
            (6, 0xffffff),
            (7, 0xff000000),
            (8, last.1 << 4),
            (9, 17),
            (10, last.0),
            (11, (272 - (bg.sx & 15)) as u32),
            (12, (last_row * bg.cols) as u32),
            (13, (256 - (bg.sy & 15)) as u32),
            (14, 0xffffff),
            (15, (bg.sx / 16) as u32),
            (24, 0xff000000),
            (25, (bg.sx & 15) as u32),
        ] {
            g[reg] = v;
        }
        g[34] = g[16];
        endpoint.hi = 0;
        endpoint.lo = (last_row * bg.cols) as u32;
        endpoint.pc = g[31];
        endpoint.next_pc = g[31] + 4;
        endpoint.ld_which = 34;
        endpoint.ld_value = g[16];
        endpoint.bdbt = 0;
        endpoint.branched = false;
        let patches = out
            .chunks_exact(4)
            .zip(r.chunks_exact(4))
            .enumerate()
            .filter_map(|(i, (a, b))| {
                (a != b).then(|| RamWordPatch {
                    offset: (i * 4) as u32,
                    value: u32::from_le_bytes(a.try_into().unwrap()),
                })
            })
            .collect();
        Ok(Self {
            before: state.clone(),
            biu,
            ram_hash: sha(r),
            scratch_hash: sha(scratch),
            endpoint,
            patches,
            scratch: sc,
            costs: clock.costs,
        })
    }
    pub fn validate(&self, r: &[u8], s: &[u8], state: &State, biu: u32) -> Result<()> {
        if *state != self.before
            || biu != self.biu
            || sha(r) != self.ram_hash
            || sha(s) != self.scratch_hash
        {
            return Err("NO3資料計畫已過期".into());
        }
        Ok(())
    }
    pub fn endpoint(&self) -> &State {
        &self.endpoint
    }
    pub fn patches(&self) -> &[RamWordPatch] {
        &self.patches
    }
    pub fn scratch(&self) -> &[u8] {
        &self.scratch
    }
    pub fn costs(&self) -> &[u32] {
        &self.costs
    }
    pub fn commit(&self, m: &mut Machine) -> Result<()> {
        self.validate(m.bus.ram(), m.bus.scratch(), &m.cpu.state, m.bus.biu())?;
        m.commit_cpu_data_phase(
            &self.endpoint,
            &self.patches,
            &self.scratch,
            &self.costs,
            200000,
        )
    }
}
