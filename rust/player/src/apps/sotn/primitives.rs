//! Candidate typed compatibility data, 026 §16.8.2. No CPU/GP0 execution.
//! Pure data only; never use this API to skip a live machine call or its clock.
use psgolem_core::cpu::State;
use sha2::{Digest, Sha256};
type Result<T> = std::result::Result<T, String>;
fn word(r: &[u8], p: usize) -> u32 {
    u32::from_le_bytes(r[p..p + 4].try_into().unwrap())
}
fn half(r: &[u8], p: usize) -> u16 {
    u16::from_le_bytes(r[p..p + 2].try_into().unwrap())
}
fn set(r: &mut [u8], p: usize, v: u32) {
    r[p..p + 4].copy_from_slice(&v.to_le_bytes());
}
fn addr(a: u32, n: usize) -> Result<usize> {
    if a < 0x80000000 || (a as u64) + (n as u64) > 0x80200000 {
        return Err("相容資料位址超界".into());
    }
    Ok((a - 0x80000000) as usize)
}
fn link(r: &mut [u8], bucket: usize, dst: usize) {
    let tag = word(r, dst) & 0xff000000 | word(r, bucket) & 0xffffff;
    set(r, dst, tag);
    let tag = word(r, bucket) & 0xff000000 | (dst as u32 & 0xffffff);
    set(r, bucket, tag);
}
pub struct DataPlan {
    pub ram: Vec<u8>,
    pub scratch: Vec<u8>,
    pub hud_records: usize,
    pub environments: usize,
}
impl DataPlan {
    pub fn prepare(input: &[u8], scratch: &[u8], entry: &State) -> Result<Self> {
        if input.len() != 2 << 20
            || scratch.len() != 1024
            || entry.pc != 0x800edc94
            || entry.gpr[29] != 0x801fffc0
            || entry.gpr[31] != 0x800e3c80
            || word(input, 0x3c0f8) != 0
            || word(input, 0x973ec) != 0
            || word(input, 0x2c26c) != 0
        {
            return Err("圖元資料入口不支援".into());
        }
        validate_identity(input)?;
        let buffer = word(input, 0x6c37c);
        let prefix = word(input, 0x97930) as usize;
        if ![0x8003cb08, 0x800542fc].contains(&buffer) || prefix != 15 || word(input, 0x9794c) != 0
        {
            return Err("圖元緩衝或前綴不支援".into());
        }
        let b = addr(buffer, 0x15400)?;
        let ot = b + 0x474;
        let mut r = input.to_vec();
        let mut s = scratch.to_vec();
        let main_sp = addr(entry.gpr[29], 0)? - 0xb0;
        // Original caller-visible ABI save area; semantic records never execute its instructions.
        for (i, g) in [16, 17, 18, 19, 20, 21, 22, 23, 30, 31].iter().enumerate() {
            set(&mut r, main_sp + 0x88 + i * 4, entry.gpr[*g]);
        }
        set(&mut r, main_sp + 0x78, 1280);
        let layout = [
            (0usize, 0usize, ot),
            (4, 0x97930, b + 0x3c74),
            (8, 0x97934, b + 0xd874),
            (12, 0x97938, b + 0xfc74),
            (16, 0x9793c, b + 0x103f4),
            (20, 0x97944, b + 0x13ff4),
            (24, 0x9792c, b + 0xc74),
            (28, 0x97948, b + 0x14ff4),
            (32, 0x9794c, b + 0x74),
        ];
        for &(p, c, base) in &layout {
            let stride = match p {
                0 => 0,
                4 => 52,
                8 => 36,
                12 => 40,
                16 | 28 => 20,
                20 => 16,
                24 => 12,
                32 => 64,
                _ => unreachable!(),
            };
            let value = base as u32
                + if c == 0 {
                    0
                } else {
                    word(input, c).checked_mul(stride).ok_or("圖元計數溢位")?
                };
            if value > 0x200000 {
                return Err("圖元計數超界".into());
            }
            set(&mut s, p, value + 0x80000000);
        }
        s[0x128..0x130].copy_from_slice(&[0, 0, 0, 0, 255, 0, 255, 0]);
        let mut hud = 0;
        let mut envs = 0;
        for i in 0..1280 {
            let p = 0x86fec + i * 52;
            let kind = r[p + 7];
            let flags = half(&r, p + 50);
            if kind == 0 || flags & 8 != 0 {
                continue;
            }
            let order = half(&r, p + 38) as usize;
            if order >= 512 {
                return Err("圖元排序超界".into());
            }
            match kind {
                4 => {
                    if flags != 0x2000
                        || order != 496
                        || half(&r, p + 26) != 31
                        || hud >= 32
                        || !(-512..=512).contains(&(half(&r, p + 8) as i16 as i32))
                        || !(-512..=512).contains(&(half(&r, p + 10) as i16 as i32))
                    {
                        return Err("未支援HUD分支".into());
                    }
                    let ci = half(&r, p + 14) as usize;
                    if ci >= 512 {
                        return Err("色盤表超界".into());
                    }
                    s[0x27] = 12;
                    s[0x2b] = 0x3d;
                    for j in 0..4 {
                        let q = p + j * 12;
                        let t = 0x24 + j * 12;
                        s[t + 4..t + 7].copy_from_slice(&r[q + 4..q + 7]);
                        s[t + 8..t + 12].copy_from_slice(&r[q + 8..q + 12]);
                        s[t + 12..t + 14].copy_from_slice(&r[q + 12..q + 14]);
                    }
                    s[0x32..0x34].copy_from_slice(&r[0x3c104 + ci * 2..0x3c104 + ci * 2 + 2]);
                    s[0x3e..0x40].copy_from_slice(&r[p + 26..p + 28]);
                    let dst = b + 0x3c74 + (prefix + hud) * 52;
                    r[dst..dst + 52].copy_from_slice(&s[0x24..0x58]);
                    link(&mut r, ot + order * 4, dst);
                    hud += 1;
                    set(&mut s, 4, (dst + 52 + 0x80000000) as u32);
                    set(&mut r, 0x97930, (prefix + hud) as u32);
                }
                7 => {
                    if ![0, 0x800].contains(&flags) || envs >= 2 {
                        return Err("未支援環境分支".into());
                    }
                    let source = word(&r, p + 16);
                    if ![0x800974ac, 0x800974ec].contains(&source) {
                        return Err("環境來源不支援".into());
                    }
                    let src = addr(source, 64)?;
                    if flags == 0x800 {
                        let mut env = r[b + 4..b + 4 + 92].to_vec();
                        env[0x18] = 0;
                        r[main_sp + 0x18..main_sp + 0x18 + 92].copy_from_slice(&env);
                        let sdk_sp = main_sp - 0x28;
                        set(&mut r, sdk_sp + 0x18, (p + 38 + 0x80000000) as u32);
                        set(&mut r, sdk_sp + 0x1c, 0x1f800024);
                        set(&mut r, sdk_sp + 0x20, 0x800ef0e0);
                        // Typed DRAWENV encoding and the SDK's small texwindow ABI footprint.
                        let x = half(&env, 0) as i16 as i32;
                        let y = half(&env, 2) as i16 as i32;
                        let ex = (half(&env, 0).wrapping_add(half(&env, 4)).wrapping_sub(1)) as i16
                            as i32;
                        let ey = (half(&env, 2).wrapping_add(half(&env, 6)).wrapping_sub(1)) as i16
                            as i32;
                        let clamp = |v: i32, max: i32| v.clamp(0, max) as u32;
                        let area0 = 0xe3000000 | clamp(x, 1023) | (clamp(y, 511) << 10);
                        let area1 = 0xe4000000 | clamp(ex, 1023) | (clamp(ey, 511) << 10);
                        let offset = 0xe5000000
                            | (half(&env, 8) as u32 & 0x7ff)
                            | ((half(&env, 10) as u32 & 0x7ff) << 11);
                        let mode = 0xe1000000
                            | (half(&env, 20) as u32 & 0x1ff)
                            | if env[22] != 0 { 0x200 } else { 0 }
                            | if env[23] != 0 { 0x400 } else { 0 };
                        let tx = (env[12] as u32) >> 3;
                        let ty = (env[14] as u32) >> 3;
                        let tw = (0u32.wrapping_sub(half(&env, 16) as u32) & 255) >> 3;
                        let th = (0u32.wrapping_sub(half(&env, 18) as u32) & 255) >> 3;
                        let window = 0xe2000000 | (tx << 10) | (ty << 15) | tw | (th << 5);
                        for (j, value) in [tx, ty, tw, th].iter().enumerate() {
                            set(&mut r, sdk_sp - 16 + j * 4, *value);
                        }
                        for (j, value) in [area0, area1, offset, mode, window, 0xe6000000]
                            .iter()
                            .enumerate()
                        {
                            set(&mut r, src + 4 + j * 4, *value);
                        }
                        r[src + 3] = 6;
                    }
                    let dst = b + 0x74 + envs * 64;
                    let bytes = r[src..src + 64].to_vec();
                    r[dst..dst + 64].copy_from_slice(&bytes);
                    link(&mut r, ot + order * 4, dst);
                    envs += 1;
                    set(&mut s, 32, (dst + 64 + 0x80000000) as u32);
                    set(&mut r, 0x9794c, envs as u32);
                }
                _ => return Err("未封閉圖元類型".into()),
            }
        }
        if hud != 7 || envs != 2 {
            return Err("圖元組合不在限定範圍".into());
        }
        Ok(Self {
            ram: r,
            scratch: s,
            hud_records: hud,
            environments: envs,
        })
    }
}

fn validate_identity(r: &[u8]) -> Result<()> {
    if format!("{:x}", Sha256::digest(&r[0xedc94..0xef214]))
        != "4bb4ddad39965905700cf855686f57fa1c9740b6da5feaf3cac0a38367a605f3"
    {
        return Err("原圖元／SDK身份不符".into());
    }
    if format!("{:x}", Sha256::digest(&r[0x13648..0x13900]))
        != "659e9238e252c354f0d94d41a72a26fa92f3a45a1c7f3cc38ac4378d0ca6ac1c"
    {
        return Err("原圖元／SDK身份不符".into());
    }
    if format!("{:x}", Sha256::digest(&r[0x13950..0x13a1c]))
        != "8fff28d87c2f0bac5a15a4c00b3658d616efbdbb522ac928768bd97a302dd7b0"
    {
        return Err("原圖元／SDK身份不符".into());
    }
    if format!("{:x}", Sha256::digest(&r[0x13a1c..0x13ae8]))
        != "6aac1ea4f5a3cb3ec5d6215f2f105db75b359d60b38e92a7a0b0d7601300da2d"
    {
        return Err("原圖元／SDK身份不符".into());
    }
    if format!("{:x}", Sha256::digest(&r[0x13ae8..0x13b24]))
        != "137d00d737b54eeb961ebc789a407c6000209606631715ee8cb901e992258fc5"
    {
        return Err("原圖元／SDK身份不符".into());
    }
    if format!("{:x}", Sha256::digest(&r[0x13900..0x13950]))
        != "132cfe1c1ab0ff1efe16514ce78eaecd394ebbf9c958f177b0a67c0949b44641"
    {
        return Err("原圖元／SDK身份不符".into());
    }
    if format!("{:x}", Sha256::digest(&r[0x13b24..0x13ba8]))
        != "9470f6c7730820992858889bf0e29b129889201b751105ff1500b1a7604517ac"
    {
        return Err("原圖元／SDK身份不符".into());
    }
    Ok(())
}
