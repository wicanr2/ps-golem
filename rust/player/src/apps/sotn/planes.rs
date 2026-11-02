//! ST0 procedural plane geometry, 026 §16.9.14, GPL-3.0-or-later.
//! Original 801BA70C / 801820B0–801821AE, preserved IDA9.4 evidence.
//! Source arrays, phases and camera only. No guest ISA, GTE state, GP0 or old XY.
use psgolem_core::{cpu::State, paint::ProjectedPoint, projection::Camera};
use sha2::{Digest, Sha256};
type Result<T> = std::result::Result<T, String>;
const POOL: u32 = 0x80086fec;
#[derive(Clone, Debug)]
pub struct Quad {
    pub handle: usize,
    pub native: [[i16; 2]; 4],
    pub points: [ProjectedPoint; 4],
    pub vertices: [[i16; 3]; 4],
    pub camera: Option<Camera>,
    pub uv: [[u8; 2]; 4],
    pub clut_index: u16,
    pub order: u16,
    pub page: u16,
    pub flags: u16,
}
pub struct Plan {
    pub quads: Vec<Quad>,
    pub hidden_handles: Vec<usize>,
    pub updated_phases: [u32; 4],
    pub updated_head_phase: [u32; 2],
}
fn h(r: &[u8], p: usize) -> u16 {
    u16::from_le_bytes(r[p..p + 2].try_into().unwrap())
}
fn w(r: &[u8], p: usize) -> u32 {
    u32::from_le_bytes(r[p..p + 4].try_into().unwrap())
}
impl Plan {
    pub fn prepare(r: &[u8], entry: &State) -> Result<Self> {
        Self::prepare_activity(r, entry, false)
    }
    pub(super) fn prepare_activity(r: &[u8], entry: &State, allow_deletion: bool) -> Result<Self> {
        if r.len() != 2 << 20
            || entry.pc != 0x801ba70c
            || !(0x800733d8..0x8007efd8).contains(&entry.gpr[4])
            || (entry.gpr[4] - 0x800733d8) % 188 != 0
        {
            return Err("平面來源事件或物件不符".into());
        }
        for (a, n, hash) in [
            (
                0x1ba70c,
                0x7a4,
                "de1e58a6352e4714bb50592730d2f9a83c534d5d51a512795750853664eb9fda",
            ),
            (
                0x1820b0,
                0xfe,
                "2c9ac32a619228885d5fb22a8ea2e774e94d41912120542afa69dd0a31d4d5e6",
            ),
        ] {
            if format!("{:x}", Sha256::digest(&r[a..a + n])) != hash {
                return Err("平面原程式或表格身份不符".into());
            }
        }
        let e = (entry.gpr[4] - 0x80000000) as usize;
        if h(r, e + 0x2c) == 0
            || !allow_deletion && (h(r, 0x7308e) as i16 as i32 + h(r, 0x733da) as i16 as i32) < 256
        {
            return Err("平面初始化或刪除分支尚未接管".into());
        }
        let ordinal = w(r, e + 0x64);
        if ordinal >= 1280 || w(r, e + 0x7c) != POOL + ordinal * 52 {
            return Err("平面配置head不符".into());
        }
        let mut handles = Vec::new();
        let mut at = w(r, e + 0x7c);
        while at != 0 {
            if !(POOL..POOL + 1280 * 52).contains(&at)
                || (at - POOL) % 52 != 0
                || handles.len() >= 112
                || handles.contains(&at)
            {
                return Err("平面配置鏈越界或循環".into());
            }
            handles.push(at);
            at = w(r, (at - 0x80000000) as usize);
        }
        if handles.len() != 112 {
            return Err("平面配置鏈長度不符".into());
        }
        let updated_phases =
            std::array::from_fn(|i| w(r, e + 0x84 + i * 4).wrapping_add([3, 8, 8, 4][i] << 16));
        let updated_head_phase = std::array::from_fn(|i| {
            w(r, e + 0x9c + i * 4).wrapping_sub(((w(r, 0x97908 + i * 4) << 16) as i32 / 6) as u32)
        });
        let hx = (updated_head_phase[0] >> 16) as u16;
        let hy = (updated_head_phase[1] >> 16) as u16;
        let native = [
            [hx as i16, hy as i16],
            [hx.wrapping_add(101) as i16, hy as i16],
            [hx as i16, hy.wrapping_add(123) as i16],
            [hx.wrapping_add(101) as i16, hy.wrapping_add(123) as i16],
        ];
        let uv = [[0, 128], [101, 128], [0, 251], [101, 251]];
        let points = std::array::from_fn(|i| ProjectedPoint {
            position: native[i].map(|v| v as f64),
            depth: 0.0,
            uv: uv[i].map(|v| v as i32),
            color: [0; 3],
        });
        let parity = (w(r, 0x3c998) & 1) as u16;
        let mut quads = vec![Quad {
            handle: ordinal as usize,
            native,
            points,
            vertices: [[0; 3]; 4],
            camera: None,
            uv,
            clut_index: 70 + u16::from(parity == 0),
            order: 4,
            page: 15,
            flags: 0,
        }];
        // Geometry is fixed source data; rotations at 801821A8 are zero under
        // the table identity above. No architectural GTE state is consulted.
        let vertices =
            std::array::from_fn(|i| std::array::from_fn(|j| h(r, 0x1820f8 + i * 8 + j * 2) as i16));
        let mut candidates = 0usize;
        'layers: for layer in 0..2 {
            let base = (w(r, 0x182198 + layer * 8) - 0x80000000) as usize;
            if base != [0x182118, 0x182158][layer] {
                return Err("平面格子表來源不符".into());
            }
            let origin = (h(r, e + 2) as i16 as i32
                + (updated_phases[layer * 2] >> 16) as u16 as i16 as i32)
                % 2048;
            let phase_y = (updated_phases[layer * 2 + 1] >> 16) as u16 as i32;
            let y =
                h(r, 0x18219c + layer * 8) as i16 as i32 + 64 - (h(r, 0x73092) as i16 as i32 / 4);
            let order = h(r, 0x18219e + layer * 8);
            for row in 0..8 {
                let z = row * 256 + 448 - phase_y % 256;
                let tile_row = (row + phase_y / 256) % 8;
                let d = z - 448;
                let top = [
                    (0xf0000 - 416 * d) >> 12,
                    (0xf0000 - 448 * d) >> 12,
                    (0xf0000 - 352 * d) >> 12,
                ];
                let bottom = [
                    (0xf0000 - 416 * d - 0x1a000) >> 12,
                    (0xf0000 - 448 * d - 0x1c000) >> 12,
                    (0xf0000 - 352 * d - 0x16000) >> 12,
                ];
                let mut x = origin;
                let mut column = 0i32;
                let mut direction = -1i32;
                loop {
                    candidates += 1;
                    if candidates > 4096 {
                        return Err("平面可見性搜尋超過界限".into());
                    }
                    column = (column + direction) & 7;
                    x += direction * 256;
                    let tile = r[base + (tile_row * 8 + column) as usize] as usize;
                    if tile == 0 {
                        continue;
                    }
                    if tile >= 9 {
                        return Err("平面UV格子索引超界".into());
                    }
                    let camera = Camera {
                        rotation: [[4096, 0, 0], [0, 4096, 0], [0, 0, 4096]],
                        translation: [x, y, z],
                        offset: [128 << 16, 160 << 16],
                        distance: 256,
                    };
                    let native = vertices.map(|v| camera.native_xy(v));
                    if native[3][1] < 0 {
                        break;
                    }
                    if native[3][0] < 0 {
                        if direction < 0 {
                            direction = 1;
                            x = origin - 256;
                            column = 7;
                        }
                        continue;
                    }
                    if native[2][0] > 256 {
                        if direction > 0 {
                            break;
                        }
                        continue;
                    }
                    let uv = std::array::from_fn(|i| {
                        let p = 0x1820b0 + tile * 8 + i * 2;
                        [r[p], r[p + 1]]
                    });
                    let points = std::array::from_fn(|i| {
                        let p = camera.project(vertices[i], native[i]);
                        ProjectedPoint {
                            position: p.position,
                            depth: p.depth,
                            uv: uv[i].map(|v| v as i32),
                            color: if i < 2 { top } else { bottom }.map(|v| v as u8),
                        }
                    });
                    let handle = ((handles[quads.len()] - POOL) / 52) as usize;
                    quads.push(Quad {
                        handle,
                        native,
                        points,
                        vertices,
                        camera: Some(camera),
                        uv,
                        clut_index: if tile < 5 { 74 + parity } else { 72 + parity },
                        order,
                        page: 15,
                        flags: 0x15,
                    });
                    if quads.len() == 112 {
                        break 'layers;
                    }
                }
            }
        }
        let hidden_handles = handles[quads.len()..]
            .iter()
            .map(|a| ((a - POOL) / 52) as usize)
            .collect();
        Ok(Self {
            quads,
            hidden_handles,
            updated_phases,
            updated_head_phase,
        })
    }
}
