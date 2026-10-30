//! SotN ordinary NO3 scene source, spec026 §16.4, GPL-3.0-or-later.
//! Own Go direct scene evidence at 8fbc874 / private probe_sotn_direct_scene.go.
//! Staged: no player registration, CPU execution or GP0 packet reads.
use psgolem_core::scene::{Material, MaterialKey, Scene, Sprite};
use sha2::{Digest, Sha256};
use std::collections::HashMap;
type Result<T> = std::result::Result<T, String>;
fn span(a: u32, n: usize, align: u32) -> Result<usize> {
    if a < 0x8018_0000 || a & (align - 1) != 0 || (a as u64) + (n as u64) > 0x8020_0000 {
        return Err("場景RAM來源超界或未對齊".into());
    }
    Ok((a - 0x8000_0000) as usize)
}
fn word(r: &[u8], a: usize) -> u32 {
    u32::from_le_bytes(r[a..a + 4].try_into().unwrap())
}
fn half(r: &[u8], a: usize) -> u16 {
    u16::from_le_bytes(r[a..a + 2].try_into().unwrap())
}
pub fn build(ram: &[u8], vram: &[u16], graphics: &[u8], left: i32, width: usize) -> Result<Scene> {
    if ram.len() != 2 << 20
        || vram.len() != 1024 * 512
        || width == 0
        || width > 1024
        || !(-1024..=1024).contains(&left)
        || format!("{:x}", Sha256::digest(graphics))
            != "a0bb78a1260eee5e3bda4bae6cfbbd0a61ddd5604905f9fd6ef833fd03319c66"
        || format!("{:x}", Sha256::digest(&ram[0x180000..0x180024]))
            != "0a19676621ba9dfaad1a39a2ab6ead6fd0438fad85a57c91be011b4f9fe0cd82"
        || half(ram, 0x6c39c) != 0
        || half(ram, 0x6c3a0) != 0
        || word(ram, 0x73104) != 1
        || word(ram, 0x7309c) != 96
        || word(ram, 0x730f0) != 32
    {
        return Err("NO3場景或原始來源不在支援範圍".into());
    }
    let mut scene = Scene {
        left,
        width,
        height: 240,
        materials: Vec::new(),
        sprites: Vec::new(),
    };
    let mut materials = HashMap::new();
    for base in [0x730d8, 0x73084] {
        if word(ram, base + 28) != 3 || word(ram, base + 40) != 0 {
            return Err("未知圖層flags或倒數".into());
        }
        let sx = half(ram, base + 10) as i16 as i32;
        let sy = half(ram, base + 14) as i16 as i32;
        let cols = word(ram, base + 32) as u64 * 16;
        let rows = word(ram, base + 36) as u64 * 16;
        if cols == 0 || rows == 0 || cols > 65536 || rows > 65536 || cols * rows > 65536 {
            return Err("圖層格數超界".into());
        }
        let data = span(word(ram, base), (cols * rows * 2) as usize, 2)?;
        let def = span(word(ram, base + 4), 16, 4)?;
        let tables = [
            span(word(ram, def), 4096, 1)?,
            span(word(ram, def + 4), 4096, 1)?,
            span(word(ram, def + 8), 4096, 1)?,
        ];
        for y in sy.div_euclid(16)..=(sy + 239).div_euclid(16) {
            for x in (sx + left).div_euclid(16)..=(sx + left + width as i32 - 1).div_euclid(16) {
                if x < 0 || y < 0 || x as u64 >= cols || y as u64 >= rows {
                    continue;
                }
                let id = half(ram, data + (y as usize * cols as usize + x as usize) * 2) as usize;
                if id == 0 {
                    continue;
                }
                if id >= 4096 {
                    return Err("圖塊ID超界".into());
                }
                let page = ram[tables[0] + id] as usize;
                let gfx = ram[tables[1] + id] as usize;
                let pal = ram[tables[2] + id] as usize;
                if page > 7 {
                    return Err("未知圖塊頁".into());
                }
                let u = (gfx & 15) * 16;
                let v = (gfx >> 4) * 16;
                let q = v / 128 * 2 + u / 128;
                let raw = &graphics[page * 0x8000 + q * 0x2000..page * 0x8000 + (q + 1) * 0x2000];
                let bank = pal % 16;
                let at = bank / 4 * 0x8000
                    + bank % 2 * 32
                    + pal / 16 * 64
                    + if bank % 4 < 2 { 0x5c00 } else { 0x7c00 };
                let palette = &graphics[at..at + 32];
                let colors = std::array::from_fn(|i| half(palette, i * 2));
                let clut = half(ram, 0x3c104 + pal * 2) as usize;
                let cx = (clut & 63) * 16;
                let cy = clut >> 6;
                if cy >= 512 {
                    return Err("色盤超界".into());
                }
                for i in 0..16 {
                    if vram[cy * 1024 + cx + i] != colors[i] {
                        return Err("原顯存色盤不同".into());
                    }
                }
                let px = (page + 8) * 64 + u / 4;
                let py = v;
                for yy in 0..16 {
                    for xx in 0..4 {
                        if vram[(py + yy) * 1024 + px + xx]
                            != half(raw, (v % 128 + yy) * 64 + (u % 128) / 2 + xx * 2)
                        {
                            return Err("原顯存圖塊不同".into());
                        }
                    }
                }
                let mut digest = Sha256::new();
                digest.update(32u32.to_le_bytes());
                digest.update(128u32.to_le_bytes());
                digest.update(raw);
                let key = MaterialKey {
                    upload: digest.finalize().into(),
                    clut: Sha256::digest(palette).into(),
                };
                let index = if let Some(&n) = materials.get(&key) {
                    n
                } else {
                    if scene.materials.len() >= 512 {
                        return Err("場景材質超過512".into());
                    }
                    let n = scene.materials.len();
                    materials.insert(key, n);
                    scene.materials.push(Material {
                        key,
                        width: 128,
                        height: 128,
                        packed: raw.to_vec(),
                        palette: colors,
                    });
                    n
                };
                scene.sprites.push(Sprite {
                    material: index,
                    x: x * 16 - sx,
                    y: y * 16 - sy,
                    u: u % 128,
                    v: v % 128,
                    width: 16,
                    height: 16,
                });
            }
        }
    }
    Ok(scene)
}
