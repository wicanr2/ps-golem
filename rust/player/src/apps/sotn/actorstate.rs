//! SotN architectural data/ABI plan, spec026 §16.7.1, GPL-3.0-or-later.
//! Original IDA9.4 locations stay in the spec. No ISA/GP0 or event replay.
use super::actor::ActorPlan;
use psgolem_core::{
    cpu::State,
    gpu::transfer::{Rect, TextureContext, TextureJob},
};
use sha2::{Digest, Sha256};
type Result<T> = std::result::Result<T, String>;
fn hash(b: &[u8]) -> [u8; 32] {
    Sha256::digest(b).into()
}
fn at(r: &[u8], a: u32, n: usize) -> Result<usize> {
    let p = (a & 0x1fffffff) as usize;
    if p >= r.len() || n > r.len() - p {
        Err(format!("架構資料來源越界 {a:08x}/{n}"))
    } else {
        Ok(p)
    }
}
fn h(r: &[u8], a: u32) -> Result<u16> {
    let i = at(r, a, 2)?;
    Ok(u16::from_le_bytes(r[i..i + 2].try_into().unwrap()))
}
fn w(r: &[u8], a: u32) -> Result<u32> {
    let i = at(r, a, 4)?;
    Ok(u32::from_le_bytes(r[i..i + 4].try_into().unwrap()))
}
fn byte(r: &[u8], a: u32) -> Result<u8> {
    Ok(r[at(r, a, 1)?])
}
fn put(r: &mut [u8], a: u32, v: u32) -> Result<()> {
    let i = at(r, a, 4)?;
    r[i..i + 4].copy_from_slice(&v.to_le_bytes());
    Ok(())
}
fn ph(r: &mut [u8], a: u32, v: u16) -> Result<()> {
    let i = at(r, a, 2)?;
    r[i..i + 2].copy_from_slice(&v.to_le_bytes());
    Ok(())
}
fn add(a: u16, b: u16) -> u16 {
    a.wrapping_add(b)
}
fn sub(a: u16, b: u16) -> u16 {
    a.wrapping_sub(b)
}
fn entity(n: usize) -> u32 {
    0x800733d8 + n as u32 * 188
}
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Architecture {
    pub gpr: [u32; 35],
    pub hi: u32,
    pub lo: u32,
    pub pc: u32,
    pub next_pc: u32,
    pub ld_which: usize,
    pub ld_value: u32,
    pub bdbt: u32,
    pub branched: bool,
}
impl Architecture {
    pub fn of(s: &State) -> Self {
        Self {
            gpr: s.gpr,
            hi: s.hi,
            lo: s.lo,
            pc: s.pc,
            next_pc: s.next_pc,
            ld_which: s.ld_which,
            ld_value: s.ld_value,
            bdbt: s.bdbt,
            branched: s.branched,
        }
    }
    pub(super) fn apply(&self, s: &mut State) {
        s.gpr = self.gpr;
        s.hi = self.hi;
        s.lo = self.lo;
        s.pc = self.pc;
        s.next_pc = self.next_pc;
        s.ld_which = self.ld_which;
        s.ld_value = self.ld_value;
        s.bdbt = self.bdbt;
        s.branched = self.branched;
    }
}
#[derive(Clone)]
pub(super) struct Part {
    pub(super) address: u32,
    pub(super) frame_data: u32,
    pub(super) n: u16,
    pub(super) packed: bool,
    pub(super) cache: u32,
    pub(super) dx: u16,
    pub(super) dy: u16,
    pub(super) width: u16,
    pub(super) height: u16,
    pub(super) x: u16,
    pub(super) y: u16,
    pub(super) flags: u8,
    pub(super) flip: u32,
    pub(super) part_flags: u16,
    pub(super) page_low: u16,
    pub(super) u0: i32,
    pub(super) v0: i32,
    pub(super) u1: i32,
    pub(super) v1: i32,
    pub(super) palette: i32,
}
impl Part {
    pub(super) fn read(r: &[u8], slot: usize, index: usize) -> Result<Self> {
        let a = entity(slot);
        let bank = h(r, a + 0x54)?;
        let frame = h(r, a + 0x56)?;
        let table = if bank & 0x8000 == 0 {
            w(r, 0x800a3b80 + bank as u32 * 4)?
        } else {
            let base = w(r, 0x8003c788)?;
            w(r, base + (bank as u32 & 0x7fff) * 4)?
        };
        let fd = w(r, table + (frame as u32 & 0x7fff) * 4)?;
        let n = h(r, fd)?;
        let packed = n & 0x8000 != 0;
        let address = if packed {
            fd + 2
        } else {
            fd + 2 + index as u32 * 22
        };
        at(r, address, if packed { 6 } else { 22 })?;
        let x = add(h(r, a + 2)?, h(r, 0x8006c39c)?);
        let y = add(h(r, a + 6)?, h(r, 0x8006c3a0)?);
        let flags = byte(r, a + 0x19)?;
        let mut flip = (h(r, a + 0x14)? as u32) << 1;
        let (
            mut dx,
            mut dy,
            mut width,
            mut height,
            mut u0,
            mut v0,
            mut u1,
            mut v1,
            palette,
            part_flags,
            page_low,
            cache,
        );
        if packed {
            cache = 0x80097d1c + h(r, a + 0x5a)? as u32 * 40;
            flip ^= byte(r, cache + 0x23)? as u32;
            let sources = match bank {
                13 => w(r, 0x8013c004)?,
                0x8001 => w(r, 0x8003c7a0)?,
                0x8010 => w(r, 0x8003c7a4)?,
                _ => 0x8013c020,
            };
            let src = w(r, sources + (n as u32 & 0x7fff) * 4)?;
            width = byte(r, src)? as u16;
            height = byte(r, src + 1)? as u16;
            dx = add(h(r, fd + 2)?, byte(r, src + 2)? as u16);
            dy = add(h(r, fd + 4)?, byte(r, src + 3)? as u16);
            u0 = (h(r, cache)? & 63) as i32 * 4;
            v0 = byte(r, cache + 2)? as i32;
            u1 = u0 + width as i32;
            v1 = v0 + height as i32;
            palette = h(r, fd + 6)? as i16 as i32;
            part_flags = 0;
            page_low = 0;
        } else {
            cache = 0;
            part_flags = h(r, address)?;
            dx = h(r, address + 2)?;
            dy = h(r, address + 4)?;
            width = h(r, address + 6)?;
            height = h(r, address + 8)?;
            let page = add(h(r, address + 12)?, h(r, a + 0x5a)?);
            page_low = page & 3;
            palette = h(r, address + 10)? as i16 as i32;
            u0 = byte(r, address + 14)? as i32;
            v0 = byte(r, address + 16)? as i32;
            u1 = byte(r, address + 18)? as i32;
            v1 = byte(r, address + 20)? as i32;
            if page & 1 != 0 {
                u0 -= 128;
                u1 -= 128;
            }
            if page & 2 != 0 {
                v0 -= 128;
                v1 -= 128;
            }
            if part_flags & 4 != 0 {
                width = sub(width, 1);
                if part_flags & 2 != 0 {
                    dx = add(dx, 1);
                }
                u1 -= 1;
            }
            if part_flags & 8 != 0 {
                height = sub(height, 1);
                if part_flags & 1 != 0 {
                    dy = add(dy, 1);
                }
                v1 -= 1;
            }
            if part_flags & 16 != 0 {
                width = sub(width, 1);
                if part_flags & 2 == 0 {
                    dx = add(dx, 1);
                }
                u0 += 1;
            }
            if part_flags & 32 != 0 {
                height = sub(height, 1);
                if part_flags & 1 == 0 {
                    dy = add(dy, 1);
                }
                v0 += 1;
            }
        }
        Ok(Self {
            address,
            frame_data: fd,
            n,
            packed,
            cache,
            dx,
            dy,
            width,
            height,
            x,
            y,
            flags,
            flip,
            part_flags,
            page_low,
            u0,
            v0,
            u1,
            v1,
            palette,
        })
    }
    fn product(&self, r: &[u8], slot: usize) -> Result<Option<i64>> {
        let a = entity(slot);
        // Last of the two distinct X/Y corner products, original EB5C0/5F4
        // then EB630/664. Rotation is excluded by the geometry contract.
        let px = if self.flip != 0 {
            sub(self.x, h(r, a + 0x20)?)
        } else {
            add(self.x, h(r, a + 0x20)?)
        };
        let py = add(self.y, h(r, a + 0x22)?);
        let left = if if self.packed {
            self.flip & 2 != 0
        } else {
            h(r, a + 0x14)? != 0
        } {
            sub(self.x, self.dx)
        } else {
            add(self.x, self.dx)
        };
        let right = if self.flip != 0 {
            add(left, 1)
        } else {
            add(left, self.width)
        };
        let bottom = add(add(self.y, self.dy), self.height);
        if self.flags & 2 != 0 {
            Ok(Some(
                sub(bottom, py) as i16 as i64 * h(r, a + 0x1c)? as i16 as i64,
            ))
        } else if self.flags & 1 != 0 {
            Ok(Some(
                sub(right, px) as i16 as i64 * h(r, a + 0x1a)? as i16 as i64,
            ))
        } else {
            Ok(None)
        }
    }
}
const SDK_HASHES: &[(usize, usize, &str)] = &[
    (
        76152,
        280,
        "dee687d91ca8c7a50efecba104337d6fde0120c9ce907bb8bede6b9b4993e5c6",
    ),
    (
        81280,
        588,
        "df65132e04b65777a6be06ceeac1c4fc759506ebc9175db0b7bbf98d908bbce2",
    ),
    (
        81868,
        652,
        "1940ecc83cdbc526a62b43ef895a0396e876a2cda1080a74c06b65cf37ddd51b",
    ),
    (
        83568,
        616,
        "00f03e9a811155d759750556f29c3df12994fa9fc875d56ac7970aa5d6d787e2",
    ),
    (
        85824,
        52,
        "874d40780b4ff771614840a691f4a8824c4366318b89769ab5ad1ae7dd164ff8",
    ),
    (
        86792,
        328,
        "a684dd4c4b7f7ddc2ef4a48249097a6ec2d27e6381f3338aa6b4a47ea3096b9b",
    ),
    (
        963908,
        1048,
        "d0828cff8b09ec9aef1922d0f1b35ffbb4a0e466d92b5eb3715b9180813233b3",
    ),
];
pub struct ActorDataPlan {
    pub geometry: ActorPlan,
    pub scratch: Vec<u8>,
    pub architecture: Architecture,
    before: State,
    ram_hash: [u8; 32],
    scratch_hash: [u8; 32],
    context: TextureContext,
    irq_mask: u32,
}
impl ActorDataPlan {
    pub fn prepare(
        input: &[u8],
        scratch: &[u8],
        state: &State,
        context: TextureContext,
        irq_mask: u32,
    ) -> Result<Self> {
        if state.pc != 0x800eb998
            || state.next_pc != 0x800eb99c
            || state.gpr[29] != 0x801fffc0
            || state.gpr[31] != 0x800e3c34
            || state.ld_which != 34
            || state.bdbt != 0
            || state.branched
            || state.gpr[0] != 0
            || context.information_high != 0
            || context
                .area_start
                .iter()
                .chain(context.area_end.iter())
                .any(|&n| n > 1023)
            || context.offset.iter().any(|&n| !(-1024..=1023).contains(&n))
            || state.cop0[12] & 0x10000 != 0
            || (state.cop0[12] & state.cop0[13] & 0xff00 != 0 && state.cop0[12] & 1 != 0)
        {
            return Err("物件資料架構入口不在支援範圍".into());
        }
        let mut plan = ActorPlan::prepare(input, scratch)?;
        for &(a, v) in &[
            (2147664480u32, 2147664416u32),
            (2147664424u32, 2147567216u32),
            (2147664428u32, 2147564928u32),
            (2147664448u32, 2147565516u32),
            (2147664508u32, 528488464u32),
            (2147664512u32, 528488468u32),
            (2147664516u32, 528486560u32),
            (2147664520u32, 528486564u32),
            (2147664524u32, 528486568u32),
            (2147664552u32, 528488468u32),
            (2147664556u32, 528486672u32),
        ] {
            if w(input, a)? != v {
                return Err("SDK靜態綁定身份不符".into());
            }
        }

        for &(a, n, digest) in SDK_HASHES {
            if format!("{:x}", Sha256::digest(&input[a..a + n])) != digest {
                return Err("SDK helper身份不符".into());
            }
        }

        if plan.count_before != 0
            || plan.hidden != 0
            || plan.count_after >= 768
            || plan.quads.is_empty()
            || w(input, 0x8002c268)? != 0
            || w(input, 0x8002c26c)? != 0
            || w(input, 0x8002c274)? == 0
            || w(input, 0x8002c278)? != 0
            || w(input, 0x8002c2a0)? != w(input, 0x8002c2a4)?
        {
            return Err("SDK分支、queue或物件掃描上下文未證實".into());
        }
        let last_q = plan.quads.last().unwrap().clone();
        let last = Part::read(input, last_q.entity, last_q.part)?;
        let last_active = (0..256)
            .filter(|&i| {
                h(input, entity(i) + 0x54).unwrap() != 0 && h(input, entity(i) + 0x56).unwrap() != 0
            })
            .last();
        if last_active == Some(255) {
            return Err("末槽活躍分支尚未支援".into());
        }
        if last.packed
            || last.flags != 0
            || last.flip != 0
            || last_q.part + 1 != last.n as usize
            || last_active != Some(last_q.entity)
        {
            return Err("最後物件／部件的終值分支尚未支援".into());
        }
        let mut ram = plan.ram.clone();
        let mut sc = scratch.to_vec();
        let buffer = w(input, 0x8006c37c)?;
        let sp = state.gpr[29];
        let frame = sp - 0x58;
        let mut last_upload = None;
        for j in &plan.images {
            let rect = j.job.rect();
            match &j.job {
                TextureJob::Upload { words, .. } => {
                    let cache = 0x80097d1c + h(input, entity(j.entity) + 0x5a)? as u32 * 40;
                    let rects = [
                        Rect {
                            x: add(rect.x, rect.width),
                            y: sub(rect.y, 1),
                            width: 1,
                            height: add(rect.height, 4),
                        },
                        Rect {
                            x: sub(rect.x, 1),
                            y: add(rect.y, rect.height),
                            width: add(rect.width, 2),
                            height: 4,
                        },
                    ];
                    for (i, r) in rects.iter().enumerate() {
                        let a = cache + 8 + i as u32 * 8;
                        put(&mut ram, a, r.x as u32 | (r.y as u32) << 16)?;
                        put(&mut ram, a + 4, r.width as u32 | (r.height as u32) << 16)?;
                    }
                    last_upload = Some((j.entity, cache, j.source, words.len() as i64));
                }
                TextureJob::Clear { .. } if rect.x & 63 == 0 && rect.width & 63 == 0 => {
                    return Err("資料ABI尚未支援對齊快速清邊".into())
                }
                _ => (),
            }
        }
        let (upload_slot, cache, source, _) = last_upload.ok_or("未上傳的物件ABI尚未支援")?;
        let packed = Part::read(input, upload_slot, 0)?;
        let last_clear = Rect {
            x: sub(h(input, cache)?, 1),
            y: add(h(input, cache + 2)?, packed.height),
            width: packed.width / 4 + 2,
            height: 4,
        };
        let xy = last_clear.x as u32 | (last_clear.y as u32) << 16;
        let wh = last_clear.width as u32 | (last_clear.height as u32) << 16;
        let area0 = context.area_start[0] as u32 | (context.area_start[1] as u32) << 10;
        let area1 = context.area_end[0] as u32 | (context.area_end[1] as u32) << 10;
        let offset = (context.offset[0] as u32 & 2047) | ((context.offset[1] as u32 & 2047) << 11);
        let sdk = [
            0x07037e40,
            0xe3000000,
            0xe4ffffff,
            0xe5000000,
            0xe6000000,
            0x60000000,
            xy,
            wh,
            0x03ffffff,
            0xe3000000 | area0,
            0xe4000000 | area1,
            0xe5000000 | offset,
        ];
        for (i, v) in sdk.iter().enumerate() {
            put(&mut ram, 0x80037e20 + i as u32 * 4, *v)?;
        }
        put(
            &mut ram,
            0x80039254,
            w(input, 0x8002d374)?.wrapping_add(240),
        )?;
        put(&mut ram, 0x80039258, 0)?;
        // Completed SDK leaf frames. Values are typed call arguments and
        // the preserved IRQ mask, not values copied from the oracle stack.
        for (delta, v) in [
            (-256, cache),
            (-252, source),
            (-248, 0x80014f50),
            (-224, 0x80013ff4),
            (-200, xy),
            (-196, wh),
            (-192, 0),
            (-188, cache + 16),
            (-184, 0x80014770),
            (-180, irq_mask),
            (-176, source),
            (-172, 0x80014770),
            (-160, 0),
            (-156, 0),
            (-152, 0),
            (-148, cache + 16),
            (-144, packed.v0 as u32),
            (-140, packed.v1 as u32),
            (-136, 0x80012b04),
            (-132, 0x80012ac8),
        ] {
            put(&mut ram, (sp as i64 + delta) as u32, v)?;
        }
        for (i, reg) in [16, 17, 18, 19, 20, 21, 22, 23, 30, 31].iter().enumerate() {
            put(&mut ram, frame + 0x30 + i as u32 * 4, state.gpr[*reg])?;
        }
        put(&mut ram, frame + 0x18, 0x800733d8 + 256 * 188)?;
        let mut working = [0u32; 8];
        working.copy_from_slice(&state.gpr[16..24]);
        working[0] = 0x1f800034;
        working[1] = 0x1f800000;
        let mut product = ((state.hi as u64) << 32 | state.lo as u64) as i64;
        let mut last_transform = None;
        for q in &plan.quads {
            let p = Part::read(input, q.entity, q.part)?;
            if p.packed {
                working[2] = p.cache;
                working[3] = p.u1 as u32;
                working[4] = p.v0 as u32;
                working[5] = p.v1 as u32;
                working[6] = p.u0 as u32;
                if let Some(job) = plan
                    .images
                    .iter()
                    .find(|j| j.entity == q.entity && matches!(j.job, TextureJob::Upload { .. }))
                {
                    if let TextureJob::Upload { words, .. } = &job.job {
                        product = words.len() as i64;
                    }
                }
            } else {
                working[2] = p.part_flags as i16 as i32 as u32;
                working[7] = p.address + 20;
            }
            if p.packed || p.flags & 3 != 0 {
                last_transform = Some((q.entity, p.clone(), working));
                if let Some(v) = p.product(input, q.entity)? {
                    product = v;
                }
            }
            if !p.packed {
                working[3] = p.u1 as u32;
                working[4] = p.v0 as u32;
                working[5] = p.v1 as u32;
                working[6] = p.u0 as u32;
            }
        }
        let (slot, t, regs) = last_transform.ok_or("無transform的ABI未證實")?;
        let transform_frame = frame - 0x38;
        for (off, v) in [
            (0x10, regs[0]),
            (0x14, regs[1]),
            (0x18, regs[2]),
            (0x1c, regs[3]),
            (0x20, regs[4]),
            (0x24, regs[5]),
            (0x28, regs[6]),
            (0x2c, regs[7]),
            (0x30, entity(slot) + 0x24),
            (0x34, if t.packed { 0x800ebefc } else { 0x800ec450 }),
        ] {
            put(&mut ram, transform_frame + off, v)?;
        }
        for (off, v) in [
            (0x10, 0x1f800000),
            (0x14, t.flip),
            (0x20, 128),
            (0x24, t.address),
            (0x28, 0xffffff),
        ] {
            put(&mut ram, frame + off, v)?;
        }
        sc[3] = 12;
        for (i, v) in last_q.words.iter().enumerate() {
            put(&mut sc, 4 + i as u32 * 4, *v)?;
        }
        for (a, v) in [
            (0x34, last.page_low),
            (0x36, h(input, entity(255) + 0x54)?),
            (0x38, h(input, entity(255) + 0x56)?),
            (0x3a, last.width),
            (0x3c, last.height),
            (0x3e, last.x),
            (0x40, last.y),
            (0x42, last.dx),
            (0x44, last.dy),
        ] {
            ph(&mut sc, a, v)?;
        }
        for (a, v) in [
            (0x48, 256),
            (0x4c, last.n as u32),
            (0x50, last.n as u32),
            (0x54, last.flip),
            (0x58, last.flags as u32),
            (0x5c, buffer + 0x474),
            (0x60, buffer + 0x3c74 + plan.count_after * 52),
        ] {
            put(&mut sc, a, v)?;
        }
        let a = entity(last_q.entity);
        let palette = h(input, a + 0x16)?;
        let palette = if palette & 0x8000 != 0 {
            (palette & 0x7fff) as i32
        } else {
            palette as i32 + last.palette
        };
        if !(0..65536).contains(&palette) {
            return Err("CLUT架構索引未支援".into());
        }
        let mut arch = Architecture::of(state);
        for (reg, v) in [
            (0, 0),
            (1, 0x80040000 + palette as u32 * 2),
            (2, 0),
            (3, 1),
            (4, plan.count_after),
            (5, last_q.address),
            (6, 0x1f800030),
            (7, last_q.address + 48),
            (8, 0x1f800030),
            (9, 128),
            (10, last.address + 22),
            (11, 0xffffff),
            (12, 0x800733d8 + 256 * 188),
        ] {
            arch.gpr[reg] = v;
        }
        arch.gpr[34] = state.gpr[16];
        arch.hi = ((product as u64) >> 32) as u32;
        arch.lo = product as u32;
        arch.pc = state.gpr[31];
        arch.next_pc = arch.pc + 4;
        arch.ld_which = 34;
        arch.ld_value = state.gpr[16];
        arch.bdbt = 0;
        arch.branched = false;
        plan.ram = ram;
        Ok(Self {
            geometry: plan,
            scratch: sc,
            architecture: arch,
            before: state.clone(),
            ram_hash: hash(input),
            scratch_hash: hash(scratch),
            context,
            irq_mask,
        })
    }
    /// Validate the complete immutable input before any data or device commit.
    pub(super) fn validate(
        &self,
        ram: &[u8],
        scratch: &[u8],
        state: &State,
        context: TextureContext,
        irq_mask: u32,
    ) -> Result<()> {
        if *state != self.before
            || hash(ram) != self.ram_hash
            || hash(scratch) != self.scratch_hash
            || context != self.context
            || irq_mask != self.irq_mask
        {
            return Err("物件架構資料計畫已過期".into());
        }
        Ok(())
    }
    /// Offline data only: clock, cache and deadlines remain at the input.
    /// Never use this partial operation to advance a live Machine.
    pub fn apply_data(
        &self,
        ram: &mut [u8],
        scratch: &mut [u8],
        state: &mut State,
        context: TextureContext,
        irq_mask: u32,
    ) -> Result<()> {
        self.validate(ram, scratch, state, context, irq_mask)?;
        ram.copy_from_slice(&self.geometry.ram);
        scratch.copy_from_slice(&self.scratch);
        self.architecture.apply(state);
        Ok(())
    }
}
