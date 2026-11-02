//! Complete detached active-plane data effects, spec026 §16.9.16.
//! GPL-3.0-or-later. Original ST0 801BA70C, preserved IDA9.4 source.
//! No guest CPU execution, instruction translation, GP0 or live clock commit.
use super::planes;
use psgolem_core::{cpu::State, gte::Gte, projection::Camera};
use sha2::{Digest, Sha256};
type Result<T> = std::result::Result<T, String>;
const POOL: usize = 0x86fec;
const IDS: &[(usize, usize, &str)] = &[
    (
        0x1805c,
        12,
        "d8c79a8c2661cba5b0c79fdf925407b5a4945c383bb0c8c293287878a693cac2",
    ),
    (
        0x1803c,
        24,
        "9aa95b09a0c5f31d49c3f954d1bc4cbe5101b46feb9a7a9a92673d0716fdfcf1",
    ),
    (
        0x17e3c,
        48,
        "095799477a23b46afa40d99dc027e7ef467cd14b8e135234d7e5b5c9ebc07883",
    ),
    (
        0x186ac,
        0x28c,
        "bb15388a4382acfb4f166213a27097561e4d921bdb3dc2f06d092e7b9ede3467",
    ),
];
pub struct DataPlan {
    pub ram: Vec<u8>,
    pub scratch: Vec<u8>,
    pub gte: Gte,
    pub geometry: planes::Plan,
    pub compatibility_commands: usize,
    pub projected_candidates: usize,
}
fn h(r: &[u8], p: usize) -> u16 {
    u16::from_le_bytes(r[p..p + 2].try_into().unwrap())
}
fn w(r: &[u8], p: usize) -> u32 {
    u32::from_le_bytes(r[p..p + 4].try_into().unwrap())
}
fn ph(r: &mut [u8], p: usize, v: u16) {
    r[p..p + 2].copy_from_slice(&v.to_le_bytes());
}
fn pw(r: &mut [u8], p: usize, v: u32) {
    r[p..p + 4].copy_from_slice(&v.to_le_bytes());
}
fn xy(v: [i16; 2]) -> u32 {
    v[0] as u16 as u32 | ((v[1] as u16 as u32) << 16)
}
fn vector(g: &mut Gte, i: usize, v: [i16; 3]) {
    g.write(i * 2, v[0] as u16 as u32 | ((v[1] as u16 as u32) << 16));
    g.write(i * 2 + 1, v[2] as i32 as u32);
}
impl DataPlan {
    pub fn prepare(r: &[u8], input_scratch: &[u8], entry: &State, before: &Gte) -> Result<Self> {
        if r.len() != 2 << 20
            || input_scratch.len() != 1024
            || entry.gpr[0] != 0
            || entry.ld_which != 34
            || entry.branched
            || entry.bdbt != 0
            || entry.cop0[12] & (1 << 30) == 0
            || entry.cop0[12] & (1 << 16) != 0
            || !(0x801f0040..=0x80200000).contains(&entry.gpr[29])
            || entry.gpr[29] & 3 != 0
        {
            return Err("平面資料入口、ABI或來源大小不符".into());
        }
        let geometry = planes::Plan::prepare(r, entry)?;
        if geometry.quads.len() >= 112 {
            return Err("平面容量用盡分支尚未封閉".into());
        }
        if IDS
            .iter()
            .any(|&(a, n, id)| format!("{:x}", Sha256::digest(&r[a..a + n])) != id)
            || r[0x2e210..0x2e214] != [0, 0, 0, 16]
        {
            return Err("平面SDK或零角度查表來源不同".into());
        }
        let e = (entry.gpr[4] - 0x80000000) as usize;
        let mut links = Vec::new();
        let mut at = w(r, e + 0x7c);
        while at != 0 {
            let p = (at - 0x80000000) as usize;
            if r[p + 7] != 4 || h(r, p + 26) != 15 {
                return Err("平面鏈種類或頁未封閉".into());
            }
            links.push(p);
            at = w(r, p);
        }
        let head = links[0];
        if h(r, head + 38) != 4
            || h(r, head + 50) != 0
            || (0..4)
                .any(|i| [r[head + 12 + i * 12], r[head + 13 + i * 12]] != geometry.quads[0].uv[i])
        {
            return Err("平面前置圖塊初始化域不同".into());
        }
        let mut ram = r.to_vec();
        let mut scratch = input_scratch.to_vec();
        let frame = (entry.gpr[29] - 0x80000000 - 0x40) as usize;
        for (i, reg) in [16, 17, 18, 19, 20, 21, 22, 23, 30, 31]
            .into_iter()
            .enumerate()
        {
            pw(&mut ram, frame + 0x18 + i * 4, entry.gpr[reg]);
        }
        for i in 0..4 {
            pw(&mut ram, e + 0x84 + i * 4, geometry.updated_phases[i]);
        }
        for i in 0..2 {
            pw(&mut ram, e + 0x9c + i * 4, geometry.updated_head_phase[i]);
        }
        for base in [0x3cb25, 0x54319] {
            ram[base..base + 3].copy_from_slice(&[16, 8, 32]);
        }
        for i in 0..9 {
            ph(&mut scratch, i * 2, if i % 4 == 0 { 4096 } else { 0 });
        }
        scratch[0x60..0x80].copy_from_slice(&r[0x1820f8..0x182118]);
        for i in 0..36 {
            pw(
                &mut scratch,
                0x80 + i * 4,
                h(r, 0x1820b0 + i * 2) as i16 as i32 as u32,
            );
        }
        ph(&mut ram, head + 14, geometry.quads[0].clut_index);
        for i in 0..4 {
            pw(&mut ram, head + 8 + i * 12, xy(geometry.quads[0].native[i]));
        }
        let vertices: [[i16; 3]; 4] =
            std::array::from_fn(|i| std::array::from_fn(|j| h(r, 0x1820f8 + i * 8 + j * 2) as i16));
        let parity = (w(r, 0x3c998) & 1) as u16;
        let mut used = 1;
        let mut candidates = 0;
        let mut searched = 0;
        let mut last = None;
        // Reconstruct the required partial stores of culled candidates. The HD
        // geometry above remains independent of these compatibility writes.
        for layer in 0..2 {
            let table = [0x182118, 0x182158][layer];
            let origin = (h(r, e + 2) as i16 as i32
                + (geometry.updated_phases[layer * 2] >> 16) as u16 as i16 as i32)
                % 2048;
            let phase = (geometry.updated_phases[layer * 2 + 1] >> 16) as u16 as i32;
            let y = h(r, 0x18219c + layer * 8) as i16 as i32 + 64 - h(r, 0x73092) as i16 as i32 / 4;
            let order = h(r, 0x18219e + layer * 8);
            for row in 0..8 {
                let z = row * 256 + 448 - phase % 256;
                let tile_row = (row + phase / 256) % 8;
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
                pw(&mut ram, frame + 0x10, top[0] as u32);
                let top_word = u32::from_le_bytes([top[0] as u8, top[1] as u8, top[2] as u8, 4]);
                let bottom_word =
                    u32::from_le_bytes([bottom[0] as u8, bottom[1] as u8, bottom[2] as u8, 4]);
                pw(&mut scratch, 0x200, top_word);
                pw(&mut scratch, 0x204, bottom_word);
                let mut x = origin;
                let mut column = 0i32;
                let mut direction = -1i32;
                loop {
                    searched += 1;
                    if searched > 4096 {
                        return Err("平面相容搜尋超過界限".into());
                    }
                    column = (column + direction) & 7;
                    x += direction * 256;
                    let tile = r[table + (tile_row * 8 + column) as usize] as usize;
                    if tile == 0 {
                        continue;
                    }
                    if tile >= 9 || used >= links.len() {
                        return Err("平面相容索引或容量超界".into());
                    }
                    let camera = Camera {
                        rotation: [[4096, 0, 0], [0, 4096, 0], [0, 0, 4096]],
                        translation: [x, y, z],
                        offset: [128 << 16, 160 << 16],
                        distance: 256,
                    };
                    let native = vertices.map(|v| camera.native_xy(v));
                    candidates += 1;
                    last = Some(camera);
                    let p = links[used];
                    let uv = 0x1820b0 + tile * 8;
                    ph(&mut ram, p + 50, 0x15);
                    ph(&mut ram, p + 12, h(r, uv));
                    pw(&mut scratch, 0x4c, xy(native[3]));
                    if native[3][1] < 0 {
                        ph(&mut ram, p + 50, 8);
                        break;
                    }
                    if native[3][0] < 0 {
                        ph(&mut ram, p + 50, 8);
                        if direction < 0 {
                            direction = 1;
                            x = origin - 256;
                            column = 7;
                        }
                        continue;
                    }
                    ph(
                        &mut ram,
                        p + 14,
                        if tile < 5 { 74 + parity } else { 72 + parity },
                    );
                    ph(&mut ram, p + 24, h(r, uv + 2));
                    ph(&mut ram, p + 36, h(r, uv + 4));
                    for i in 0..3 {
                        pw(&mut scratch, 0x28 + i * 12, xy(native[i]));
                    }
                    if native[2][0] > 256 {
                        ph(&mut ram, p + 50, 8);
                        if direction > 0 {
                            break;
                        }
                        continue;
                    }
                    ph(&mut ram, p + 48, h(r, uv + 6));
                    for i in 0..4 {
                        pw(&mut ram, p + 8 + i * 12, xy(native[i]));
                    }
                    for i in 0..2 {
                        pw(&mut ram, p + 4 + i * 12, top_word);
                        pw(&mut ram, p + 28 + i * 12, bottom_word);
                    }
                    ph(&mut ram, p + 38, order);
                    used += 1;
                }
            }
        }
        if used != geometry.quads.len() {
            return Err("平面相容選面與直接幾何不一致".into());
        }
        for p in &links[used..] {
            ph(&mut ram, *p + 50, 8);
        }
        let camera = last.ok_or("平面未產生可靠的相容末端")?;
        let mut gte = before.clone();
        for i in 0..5 {
            gte.write(32 + i, w(&scratch, i * 4));
        }
        gte.write(56, 128 << 16);
        gte.write(57, 160 << 16);
        gte.write(58, 256);
        for i in 0..3 {
            gte.write(37 + i, camera.translation[i] as u32);
        }
        vector(&mut gte, 0, vertices[3]);
        gte.execute(0x180001);
        for i in 0..3 {
            vector(&mut gte, i, vertices[i]);
        }
        gte.execute(0x280030);
        debug_assert!(links.iter().all(|p| *p >= POOL));
        Ok(Self {
            ram,
            scratch,
            gte,
            geometry,
            compatibility_commands: 2,
            projected_candidates: candidates,
        })
    }
}
