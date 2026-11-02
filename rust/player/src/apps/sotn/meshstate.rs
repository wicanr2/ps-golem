//! Authored mesh -> HD geometry and compatibility RAM/GTE, 026 §16.9.7.
//! GPL-3.0-or-later. Original ST0 801BB04C–801BB320; own mesh/projection/gte.
//! No guest ISA, GP0, framebuffer or old projected geometry as drawing input.
use super::{
    camera::Sources,
    mesh::{Face, Model},
};
use psgolem_core::{cpu::State, gte::Gte, paint::ProjectedPoint, projection::Camera};
type Result<T> = std::result::Result<T, String>;
const POOL: u32 = 0x80086fec;

#[derive(Clone, Debug)]
pub struct DrawFace {
    pub handle: usize,
    pub face: Face,
    pub points: [ProjectedPoint; 4],
    pub native: [[i16; 2]; 4],
    pub page: u16,
    pub flags: u16,
}
pub struct DataPlan {
    pub ram: Vec<u8>,
    pub gte: Gte,
    pub draws: Vec<DrawFace>,
    pub hidden_handles: Vec<usize>,
    pub compatibility_commands: usize,
}
fn half(r: &[u8], p: usize) -> u16 {
    u16::from_le_bytes(r[p..p + 2].try_into().unwrap())
}
fn word(r: &[u8], p: usize) -> u32 {
    u32::from_le_bytes(r[p..p + 4].try_into().unwrap())
}
fn put_half(r: &mut [u8], p: usize, v: u16) {
    r[p..p + 2].copy_from_slice(&v.to_le_bytes());
}
fn put_word(r: &mut [u8], p: usize, v: u32) {
    r[p..p + 4].copy_from_slice(&v.to_le_bytes());
}

fn controls_match(g: &Gte, c: &Camera) -> bool {
    let flat: Vec<_> = c.rotation.iter().flatten().copied().collect();
    (0..4).all(|i| {
        g.read(32 + i) == flat[i * 2] as u16 as u32 | ((flat[i * 2 + 1] as u16 as u32) << 16)
    }) && g.read(36) == flat[8] as i32 as u32
        && (0..3).all(|i| g.read(37 + i) == c.translation[i] as u32)
        && (0..2).all(|i| g.read(56 + i) == c.offset[i] as u32)
        && g.read(58) == c.distance as i16 as i32 as u32
}
fn vector(g: &mut Gte, slot: usize, v: [i16; 3]) {
    g.write(slot * 2, v[0] as u16 as u32 | ((v[1] as u16 as u32) << 16));
    g.write(slot * 2 + 1, v[2] as i32 as u32);
}
/// Architectural effects only. Four final projected points overwrite all SZ
/// slots; the last three overwrite SXY. No per-face command replay is needed.
fn final_gte(before: &Gte, faces: &[Face]) -> (Gte, usize) {
    let mut g = before.clone();
    let mut commands = 0;
    let last = faces.last().unwrap();
    if last.corners == 3 && faces.len() > 1 {
        let prev = &faces[faces.len() - 2];
        vector(&mut g, 0, prev.vertices[prev.corners - 1]);
        g.execute(0x180001);
        commands += 1;
    }
    for i in 0..3 {
        vector(&mut g, i, last.vertices[i]);
    }
    g.execute(0x280030);
    commands += 1;
    if last.corners == 4 {
        vector(&mut g, 0, last.vertices[3]);
        g.execute(0x180001);
        commands += 1;
    }
    (g, commands)
}

impl DataPlan {
    /// Detached model data only. No CPU endpoint, device timing or PC change.
    pub fn prepare(r: &[u8], event: &State, before_gte: &Gte) -> Result<Self> {
        if event.pc != 0x801bb04c
            || event.ld_which != 34
            || event.cop0[12] & (1 << 30) == 0
            || event.cop0[12] & (1 << 16) != 0
        {
            return Err("鐘塔模型事件或CPU資料前置條件不符".into());
        }
        let sources = Sources::new(r)?;
        // Reuse the source reader at the immediately preceding camera phase.
        // This is a local lookup token; the live CPU/PC is never changed.
        let mut pose_event = event.clone();
        pose_event.pc = 0x801bafb4;
        let camera = sources.camera_at_event(&pose_event)?;
        if !controls_match(before_gte, &camera) {
            return Err("鐘塔模型相機設定尚未一致".into());
        }
        let model = Model::decode(r)?;
        let e = (event.gpr[18] - 0x80000000) as usize;
        let slot = word(r, e + 0x64);
        let head = word(r, e + 0x7c);
        if slot >= 1280 || head != POOL + slot * 52 {
            return Err("鐘塔配置head與handle不符".into());
        }
        let mut links = Vec::with_capacity(40);
        let mut at = head;
        while at != 0 {
            if !(POOL..POOL + 1280 * 52).contains(&at)
                || (at - POOL) % 52 != 0
                || links.len() >= 40
                || links.contains(&at)
            {
                return Err("鐘塔配置鏈越界、循環或容量不符".into());
            }
            links.push(at);
            at = word(r, (at - 0x80000000) as usize);
        }
        if links.len() != 40 || model.faces.len() > links.len() {
            return Err("鐘塔配置鏈未完整".into());
        }
        for (i, &link) in links.iter().enumerate() {
            let p = (link - 0x80000000) as usize;
            if half(r, p + 26) != 15
                || i < model.faces.len()
                    && (0..model.faces[i].corners)
                        .any(|j| r[p + 4 + j * 12..p + 7 + j * 12] != [128; 3])
            {
                return Err("鐘塔模型材質或色彩域未封閉".into());
            }
        }
        let mut ram = r.to_vec();
        let mut draws = Vec::with_capacity(model.faces.len());
        for (&link, face) in links.iter().zip(&model.faces) {
            let p = (link - 0x80000000) as usize;
            let native = face.vertices.map(|v| camera.native_xy(v));
            let points = std::array::from_fn(|i| {
                let v = camera.project(face.vertices[i], native[i]);
                ProjectedPoint {
                    position: v.position,
                    depth: v.depth,
                    uv: face.uv[i].map(|v| v as i32),
                    color: [128; 3],
                }
            });
            ram[p + 7] = if face.corners == 3 { 5 } else { 4 };
            put_half(&mut ram, p + 14, face.clut_index);
            put_half(&mut ram, p + 38, face.order);
            put_half(&mut ram, p + 50, 2);
            for i in 0..face.corners {
                ram[p + 12 + i * 12..p + 14 + i * 12].copy_from_slice(&face.uv[i]);
                put_word(
                    &mut ram,
                    p + 8 + i * 12,
                    native[i][0] as u16 as u32 | ((native[i][1] as u16 as u32) << 16),
                );
            }
            draws.push(DrawFace {
                handle: ((link - POOL) / 52) as usize,
                face: face.clone(),
                points,
                native,
                page: 15,
                flags: 2,
            });
        }
        let hidden_handles = links[model.faces.len()..]
            .iter()
            .map(|&a| {
                put_half(&mut ram, (a - 0x80000000) as usize + 50, 8);
                ((a - POOL) / 52) as usize
            })
            .collect();
        let (gte, compatibility_commands) = final_gte(before_gte, &model.faces);
        Ok(Self {
            ram,
            gte,
            draws,
            hidden_handles,
            compatibility_commands,
        })
    }
}
