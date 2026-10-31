//! ST0 authored model data, 026 §16.9.2. GPL-3.0-or-later.
//! Evidence: original sub_801BAEB0, preserved IDA9.4 bytes/xrefs and normal
//! source-flow-v2. No game instructions, GTE state, SXY, GP0 or primitive pool.
use psgolem_core::{paint::ProjectedPoint, projection::Camera};
use sha2::{Digest, Sha256};
type Result<T> = std::result::Result<T, String>;
const CODE_ID: &str = "5333d6333987ffe96072d776de76884ca22c889900cda0d2ce3771dc1e6b1e26";

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Face {
    pub vertices: [[i16; 3]; 4],
    pub uv: [[u8; 2]; 4],
    pub corners: usize,
    pub clut_index: u16,
    pub order: u16,
}
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Model {
    pub faces: Vec<Face>,
}
fn half(r: &[u8], p: usize) -> u16 {
    u16::from_le_bytes(r[p..p + 2].try_into().unwrap())
}
fn word(r: &[u8], p: usize) -> u32 {
    u32::from_le_bytes(r[p..p + 4].try_into().unwrap())
}
impl Model {
    pub fn decode(ram: &[u8]) -> Result<Self> {
        if ram.len() != 2 << 20
            || format!("{:x}", Sha256::digest(&ram[0x1baeb0..0x1bb368])) != CODE_ID
        {
            return Err("鐘塔模型的ST0來源或長度不符".into());
        }
        let pointers: Vec<_> = (0..9).map(|i| word(ram, 0x182440 + i * 4)).collect();
        let mut banks = Vec::new();
        for (i, &p) in pointers.iter().enumerate() {
            let end = if i + 1 < 9 {
                pointers[i + 1]
            } else {
                0x801823c4
            };
            if !(0x801821f4..0x801823c4).contains(&p)
                || end <= p
                || end > 0x801823c4
                || (p - 0x801821f4) % 8 != 0
                || (end - p) % 8 != 0
            {
                return Err("鐘塔頂點bank範圍不符".into());
            }
            banks.push(((p - 0x80000000) as usize, ((end - p) / 8) as usize));
        }
        let mut faces = Vec::new();
        for (start, end, corners) in [(0x1823c4, 0x1823fc, 3), (0x1823fc, 0x182440, 4)] {
            let mut at = start;
            let mut bank = None;
            loop {
                if at >= end {
                    return Err("鐘塔面索引缺少結束標記".into());
                }
                let tag = ram[at];
                if tag == 255 {
                    break;
                }
                if tag & 128 != 0 {
                    if at + 2 >= end {
                        return Err("鐘塔bank切換截斷".into());
                    }
                    let id = ram[at + 1] as usize;
                    if id >= banks.len() {
                        return Err("鐘塔bank索引超界".into());
                    }
                    bank = Some(id);
                    at += 2;
                }
                if at + corners + 2 > end || faces.len() >= 40 {
                    return Err("鐘塔面資料或容量超界".into());
                }
                let bank = bank.ok_or("鐘塔首面缺少bank來源")?;
                let (base, count) = banks[bank];
                let ui = ram[at] as usize;
                let mut vertices = [[0; 3]; 4];
                for (i, v) in vertices.iter_mut().enumerate().take(corners) {
                    let vi = ram[at + 1 + i] as usize;
                    if vi >= count {
                        return Err("鐘塔頂點索引超界".into());
                    }
                    *v = std::array::from_fn(|axis| half(ram, base + vi * 8 + axis * 2) as i16);
                }
                let (uv, clut) = if corners == 3 {
                    if ui >= 3 {
                        return Err("鐘塔三角形UV索引超界".into());
                    }
                    let p = 0x1821b0 + ui * 8;
                    (
                        [
                            [ram[p], ram[p + 1]],
                            [ram[p + 2], ram[p + 3]],
                            [ram[p + 4], ram[p + 5]],
                            [ram[p + 4], ram[p + 5]],
                        ],
                        half(ram, p + 6),
                    )
                } else {
                    if ui >= 7 {
                        return Err("鐘塔四邊形UV索引超界".into());
                    }
                    let p = 0x1821c8 + ui * 6;
                    let (u, v, w, h) = (ram[p], ram[p + 1], ram[p + 2], ram[p + 3]);
                    (
                        [
                            [u, v],
                            [u.wrapping_add(w), v],
                            [u, v.wrapping_add(h)],
                            [u.wrapping_add(w), v.wrapping_add(h)],
                        ],
                        half(ram, p + 4),
                    )
                };
                if clut >= 512 {
                    return Err("鐘塔CLUT索引超界".into());
                }
                if corners == 3 {
                    vertices[3] = vertices[2];
                }
                faces.push(Face {
                    vertices,
                    uv,
                    corners,
                    clut_index: clut,
                    order: 64 + ram[at + corners + 1] as u16,
                });
                at += corners + 2;
            }
        }
        if faces.is_empty() {
            return Err("鐘塔模型沒有面".into());
        }
        Ok(Self { faces })
    }
}
impl Face {
    /// Pose and later screen translation are explicit typed inputs. Their live
    /// game producers/clock effects are separate and not bypassed by this API.
    pub fn project(&self, camera: &Camera, screen_offset: [i16; 2]) -> [ProjectedPoint; 4] {
        std::array::from_fn(|i| {
            let p = camera.project_vertex(self.vertices[i]);
            ProjectedPoint {
                position: std::array::from_fn(|axis| p.position[axis] + screen_offset[axis] as f64),
                depth: p.depth,
                uv: self.uv[i].map(|v| v as i32),
                color: [128; 3],
            }
        })
    }
}
