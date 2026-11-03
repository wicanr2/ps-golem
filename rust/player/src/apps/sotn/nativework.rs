//! ST0 native compatibility jobs from semantic source records, 026 §16.18.
//! GPL-3.0-or-later. No old packet, OT traversal, GP0 parser or CPU execution.
//! HD keeps the direct model/tile/actor producers; this is compatibility data.
use super::{actor::Quad, call::Tile};
use psgolem_core::gpu::scene::{NativeCommand, NativeVertex};
use sha2::{Digest, Sha256};
type Result<T> = std::result::Result<T, String>;
fn h(r: &[u8], p: usize) -> u16 {
    u16::from_le_bytes(r[p..p + 2].try_into().unwrap())
}
fn w(r: &[u8], p: usize) -> u32 {
    u32::from_le_bytes(r[p..p + 4].try_into().unwrap())
}
pub struct Work {
    pub commands: Vec<NativeCommand>,
    pub pool: usize,
    pub actors: usize,
    pub tiles: usize,
}
impl Work {
    pub fn prepare(
        r: &[u8],
        scratch: &[u8],
        tiles: &[Tile],
        actors: &[Quad],
        actor_seed: &[u8],
    ) -> Result<Self> {
        if r.len() != 2 << 20
            || scratch.len() != 1024
            || actor_seed.len() != 1024
            || ![0x8003cb08, 0x800542fc].contains(&w(r, 0x6c37c))
            || w(r, 0x3c0f8) != 0
            || w(r, 0x973ec) != 0
            || w(r, 0x2c26c) != 0
            || w(r, 0xbd284) != 0
            || h(r, 0x6c39c) != 0
            || h(r, 0x6c3a0) != 0
            || actors.len() > 32
            || tiles.is_empty()
            || tiles.len() > 640
            || w(r, 0x97930) != actors.len() as u32
            || w(r, 0x97938) != 0
            || w(r, 0x97940) != tiles.len() as u32
            || w(r, 0x9792c) != 48
            || w(r, 0x9794c) != 0
        {
            return Err("ST0原生工作來源、前綴或分支不符".into());
        }
        if format!("{:x}", Sha256::digest(&r[0xedc94..0xef214]))
            != "4bb4ddad39965905700cf855686f57fa1c9740b6da5feaf3cac0a38367a605f3"
            || format!("{:x}", Sha256::digest(&r[0x180000..0x180024]))
                != "ce14842f7f0101760916814b497603a5db6241409581fff963d5fb5fd2154a1b"
        {
            return Err("ST0或原圖元來源版本不符".into());
        }
        let mut staged = Vec::new();
        let mut serial = 0usize;
        let mut add = |order: usize, command: NativeCommand| {
            staged.push((order, serial, command));
            serial += 1;
        };
        for t in tiles {
            if t.page > 7
                || t.order != (if t.foreground { 160 } else { 96 }) + t.page as usize
                || (t.x as i32).abs() > 512
                || (t.y as i32).abs() > 512
            {
                return Err("ST0圖塊原生工作超界".into());
            }
            add(
                t.order,
                NativeCommand::Sprite16 {
                    x: t.x,
                    y: t.y,
                    u: (t.gfx & 15) * 16,
                    v: (t.gfx >> 4) * 16,
                    clut: h(r, 0x3c104 + t.palette as usize * 2),
                    color: [0; 3],
                    raw: true,
                    semi: false,
                },
            );
        }
        // Layer orders are disjoint. Modes precede sprites in each forward bucket.
        for base in [160, 96] {
            for page in 0..24 {
                add(
                    base + page,
                    NativeCommand::Mode {
                        page: page as u16 + 8,
                    },
                );
            }
        }
        for q in actors {
            if !(104..160).contains(&q.order) || !(0x3c..=0x3f).contains(&q.command) {
                return Err("ST0人物原生排序或類型不符".into());
            }
            let vertices = q
                .vertices
                .iter()
                .enumerate()
                .map(|(i, p)| NativeVertex {
                    x: p.x,
                    y: p.y,
                    u: p.u,
                    v: p.v,
                    color: p.color,
                    color_pad: actor_seed[7 + i * 12],
                    uv_pad: h(actor_seed, 14 + i * 12),
                })
                .collect();
            add(
                q.order,
                NativeCommand::Polygon {
                    vertices,
                    page: q.page,
                    clut: q.clut,
                    raw: q.command & 1 != 0,
                    semi: q.command & 2 != 0,
                },
            );
        }
        let mut pool = 0;
        for id in 0..1280 {
            let p = 0x86fec + id * 52;
            let kind = r[p + 7];
            let flags = h(r, p + 50);
            if kind == 0 || flags & 8 != 0 {
                continue;
            }
            let order = h(r, p + 38) as usize;
            let ci = h(r, p + 14) as usize;
            if ![4, 5].contains(&kind)
                || flags & !0x20f7 != 0
                || order >= 512
                || ci >= 512
                || (h(r, p + 8) as i16 as i32).abs() > 512
                || (h(r, p + 10) as i16 as i32).abs() > 512
            {
                return Err("可見通用池包含未支援圖元".into());
            }
            let n = if kind == 4 { 4 } else { 3 };
            let vertices = (0..n)
                .map(|i| {
                    let a = p + i * 12;
                    NativeVertex {
                        x: h(r, a + 8) as i16,
                        y: h(r, a + 10) as i16,
                        u: r[a + 12],
                        v: r[a + 13],
                        color: r[a + 4..a + 7].try_into().unwrap(),
                        color_pad: scratch[0x2b + i * 12],
                        uv_pad: h(scratch, 0x32 + i * 12),
                    }
                })
                .collect();
            add(
                order,
                NativeCommand::Polygon {
                    vertices,
                    page: h(r, p + 26).wrapping_add(flags & 0x60),
                    clut: h(r, 0x3c104 + ci * 2),
                    raw: flags & 4 == 0,
                    semi: flags & 1 != 0,
                },
            );
            pool += 1;
        }
        staged.sort_by(|a, b| a.0.cmp(&b.0).then(b.1.cmp(&a.1)));
        Ok(Self {
            commands: staged.into_iter().map(|(_, _, c)| c).collect(),
            pool,
            actors: actors.len(),
            tiles: tiles.len(),
        })
    }
}
