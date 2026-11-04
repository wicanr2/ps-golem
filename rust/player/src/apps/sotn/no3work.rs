//! NO3 complete native work from typed sources, 026 §16.26, GPL-3.0-or-later.
//! No original OT, generated packet, actor words/address, or CPU execution.
use super::{actor::Quad, call::Tile, no3env};
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
    pub actors: usize,
    pub tiles: usize,
    pub hud: usize,
    pub environments: usize,
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
            || actors.len() > 32
            || tiles.is_empty()
            || tiles.len() > 640
            || w(r, 0x97930) != actors.len() as u32
            || w(r, 0x97938) != 0
            || w(r, 0x97940) != tiles.len() as u32
            || w(r, 0x9792c) != 48
            || w(r, 0x9794c) != 0
            || w(r, 0x3c0f8) != 0
            || w(r, 0x973ec) != 0
            || w(r, 0x2c26c) != 0
            || w(r, 0xbd284) != 0
            || h(r, 0x6c39c) != 0
            || h(r, 0x6c3a0) != 0
        {
            return Err("NO3工作來源或前綴不符".into());
        }
        if format!("{:x}", Sha256::digest(&r[0xedc94..0xef214]))
            != "4bb4ddad39965905700cf855686f57fa1c9740b6da5feaf3cac0a38367a605f3"
        {
            return Err("NO3圖元版本不符".into());
        }
        let mut environments: Vec<_> = no3env::prepare(r)?.into_iter().collect();
        let mut staged = Vec::new();
        let mut serial = 0;
        let mut add = |order: usize, commands: Vec<NativeCommand>| {
            staged.push((order, serial, commands));
            serial += 1;
        };
        for t in tiles {
            if t.page > 7
                || t.order != (if t.foreground { 96 } else { 32 }) + t.page as usize
                || (t.x as i32).abs() > 512
                || (t.y as i32).abs() > 512
            {
                return Err("NO3圖塊排序、頁面或座標不符".into());
            }
            add(
                t.order,
                vec![NativeCommand::Sprite16 {
                    x: t.x,
                    y: t.y,
                    u: (t.gfx & 15) * 16,
                    v: (t.gfx >> 4) * 16,
                    clut: h(r, 0x3c104 + t.palette as usize * 2),
                    color: [0; 3],
                    raw: true,
                    semi: false,
                }],
            );
        }
        for base in [96, 32] {
            for page in 0..24 {
                add(
                    base + page,
                    vec![NativeCommand::Mode {
                        page: page as u16 + 8,
                    }],
                );
            }
        }
        for q in actors {
            if !(104..400).contains(&q.order) || !(0x3c..=0x3f).contains(&q.command) {
                return Err("NO3人物與環境交錯排序未封閉".into());
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
                vec![NativeCommand::Polygon {
                    vertices,
                    page: q.page,
                    clut: q.clut,
                    raw: q.command & 1 != 0,
                    semi: q.command & 2 != 0,
                }],
            );
        }
        let mut hud = 0;
        for id in 0..1280 {
            let p = 0x86fec + id * 52;
            let kind = r[p + 7];
            let flags = h(r, p + 50);
            if kind == 0 || flags & 8 != 0 {
                continue;
            }
            if kind == 7 {
                let i = environments
                    .iter()
                    .position(|g| g.record == id)
                    .ok_or("NO3存在未知環境來源")?;
                let g = environments.remove(i);
                add(g.order, g.commands);
                continue;
            }
            if kind != 4
                || h(r, p + 38) != 496
                || h(r, p + 26) != 31
                || flags & !0x2005 != 0
                || flags & 0x2000 == 0
                || h(r, p + 14) >= 512
            {
                return Err("NO3存在未接管通用圖元".into());
            }
            let mut vertices = Vec::new();
            for i in 0..4 {
                let a = p + i * 12;
                let x = h(r, a + 8) as i16;
                let y = h(r, a + 10) as i16;
                if (x as i32).abs() > 512 || (y as i32).abs() > 512 {
                    return Err("NO3 HUD座標未封閉".into());
                }
                vertices.push(NativeVertex {
                    x,
                    y,
                    u: r[a + 12],
                    v: r[a + 13],
                    color: r[a + 4..a + 7].try_into().unwrap(),
                    color_pad: scratch[0x2b + i * 12],
                    uv_pad: h(scratch, 0x32 + i * 12),
                });
            }
            add(
                496,
                vec![NativeCommand::Polygon {
                    vertices,
                    page: 31,
                    clut: h(r, 0x3c104 + h(r, p + 14) as usize * 2),
                    raw: flags & 4 == 0,
                    semi: flags & 1 != 0,
                }],
            );
            hud += 1;
        }
        if !environments.is_empty() || hud == 0 || hud > 32 {
            return Err("NO3環境或HUD來源不完整".into());
        }
        staged.sort_by(|a, b| a.0.cmp(&b.0).then(b.1.cmp(&a.1)));
        // Pool nodes prepend by order; commands inside one environment keep order.
        Ok(Self {
            commands: staged.into_iter().flat_map(|(_, _, v)| v).collect(),
            actors: actors.len(),
            tiles: tiles.len(),
            hud,
            environments: 2,
        })
    }
}
