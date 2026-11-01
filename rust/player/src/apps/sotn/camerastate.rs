//! Camera RAM/GTE data effects, 026 §16.9.4, GPL-3.0-or-later.
//! Original ST0 801BAFB4–801BB04C and SDK helpers listed in IDS.
//! An unregistered data plan, never a CPU/clock jump or old drawing replay.
use super::camera::Sources;
use psgolem_core::{cpu::State, gte::Gte, projection::Camera};
use sha2::{Digest, Sha256};

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
        0x17cdc,
        36,
        "e7c83d5bccd6bc52a2f15133283c464864c5087e31234fa07c436e2946e8eac4",
    ),
    (
        0x17ecc,
        32,
        "2f228789930df0a6d6db76e145ba3301694d7c01b9df2dbf45d2f48ad7aaee1e",
    ),
];

pub struct DataPlan {
    pub ram: Vec<u8>,
    pub gte: Gte,
    pub camera: Camera,
}

fn half(r: &[u8], p: usize) -> u16 {
    u16::from_le_bytes(r[p..p + 2].try_into().unwrap())
}

impl DataPlan {
    /// Produce detached data only. The caller must still close CPU/clock effects
    /// before this plan can replace any original execution on a Machine.
    pub fn prepare(ram: &[u8], entry: &State, gte: &Gte) -> Result<Self, String> {
        let sources = Sources::new(ram)?;
        let sp = entry.gpr[29];
        if !(0x801f0000..=0x801fffb8).contains(&sp)
            || sp & 3 != 0
            || entry.ld_which != 34
            || entry.cop0[12] & (1 << 30) == 0
            || entry.cop0[12] & (1 << 16) != 0
        {
            return Err("鐘塔相機堆疊或CPU前置條件不支援".into());
        }
        if IDS
            .iter()
            .any(|&(a, n, h)| format!("{:x}", Sha256::digest(&ram[a..a + n])) != h)
        {
            return Err("鐘塔相機SDK來源不符".into());
        }
        let camera = sources.camera_at_event(entry)?;
        let entity = (entry.gpr[18] - 0x80000000) as usize;
        let angles = [
            half(ram, entity + 0x9c),
            half(ram, entity + 0x9e).wrapping_sub(0x90),
            half(ram, entity + 0xa0),
        ];
        let sp = (sp - 0x80000000) as usize;
        let mut output = ram.to_vec();
        for (i, value) in angles.iter().enumerate() {
            output[sp + 0x10 + i * 2..sp + 0x12 + i * 2].copy_from_slice(&value.to_le_bytes());
        }
        for (i, value) in camera.rotation.iter().flatten().enumerate() {
            output[sp + 0x28 + i * 2..sp + 0x2a + i * 2].copy_from_slice(&value.to_le_bytes());
        }
        for (i, value) in camera.translation.iter().enumerate() {
            for base in [0x18, 0x3c] {
                output[sp + base + i * 4..sp + base + i * 4 + 4]
                    .copy_from_slice(&value.to_le_bytes());
            }
        }
        let mut after = gte.clone();
        // Read the final packed words, including the untouched matrix padding.
        // Gte::write retains CR4's reserved bits just as the original CTC2 does.
        for i in 0..5 {
            let p = sp + 0x28 + i * 4;
            after.write(
                32 + i,
                u32::from_le_bytes(output[p..p + 4].try_into().unwrap()),
            );
        }
        for (i, value) in camera.translation.iter().enumerate() {
            after.write(37 + i, *value as u32);
        }
        for (i, value) in camera.offset.iter().enumerate() {
            after.write(56 + i, *value as u32);
        }
        after.write(58, camera.distance as u32);
        Ok(Self {
            ram: output,
            gte: after,
            camera,
        })
    }
}
