//! NO3 environment sources, 026 §16.25. GPL-3.0-or-later.
//! Source identities: retained normal 8382–8385, DRA pool 80086FEC.
//! Cached SDK input is fingerprinted, never decoded as the scene's GP0 stream.
use psgolem_core::gpu::{scene::NativeCommand, transfer::Rect};
use sha2::{Digest, Sha256};
type Result<T> = std::result::Result<T, String>;
fn w(r: &[u8], p: usize) -> u32 {
    u32::from_le_bytes(r[p..p + 4].try_into().unwrap())
}
fn h(r: &[u8], p: usize) -> u16 {
    u16::from_le_bytes(r[p..p + 2].try_into().unwrap())
}
pub struct Group {
    pub record: usize,
    pub order: usize,
    pub commands: Vec<NativeCommand>,
}
pub fn prepare(r: &[u8]) -> Result<[Group; 2]> {
    if r.len() != 2 << 20
        || format!("{:x}", Sha256::digest(&r[0x180000..0x180024]))
            != "0a19676621ba9dfaad1a39a2ab6ead6fd0438fad85a57c91be011b4f9fe0cd82"
    {
        return Err("NO3環境來源版本不符".into());
    }
    let buffer = w(r, 0x6c37c);
    let x = match buffer {
        0x8003cb08 => 0,
        0x800542fc => 256,
        _ => return Err("NO3環境buffer不符".into()),
    };
    for (id, flags, order, source) in [(6, 0, 400, 0x800974ac), (7, 0x800, 432, 0x800974ec)] {
        let p = 0x86fec + id * 52;
        if r[p + 7] != 7 || h(r, p + 50) != flags || h(r, p + 38) != order || w(r, p + 16) != source
        {
            return Err("NO3環境來源槽位、排序或模式不符".into());
        }
    }
    if r[0x974af] != 9
        || format!("{:x}", Sha256::digest(&r[0x974b0..0x974ec]))
            != "69d7f53b021d4e71012192a32987df62d0bf688079d750dc6857712da06f506d"
    {
        return Err("NO3固定暫存環境身份不符".into());
    }
    let p = (buffer - 0x80000000) as usize + 4;
    if [
        h(r, p),
        h(r, p + 2),
        h(r, p + 4),
        h(r, p + 6),
        h(r, p + 8),
        h(r, p + 10),
    ] != [x, 20, 256, 207, x, 0]
        || r[p + 12..p + 20] != [0; 8]
        || h(r, p + 20) != 10
        || r[p + 22..p + 24] != [0, 1]
    {
        return Err("NO3還原DRAWENV未封閉".into());
    }
    Ok([
        Group {
            record: 6,
            order: 400,
            commands: vec![
                NativeCommand::Environment {
                    area: Rect {
                        x: 512,
                        y: 448,
                        width: 63,
                        height: 63,
                    },
                    offset: [512, 448],
                    page: 0x40a,
                },
                NativeCommand::SolidRect {
                    x: 0,
                    y: 0,
                    width: 63,
                    height: 63,
                    color: [0; 3],
                },
            ],
        },
        Group {
            record: 7,
            order: 432,
            commands: vec![NativeCommand::Environment {
                area: Rect {
                    x,
                    y: 20,
                    width: 256,
                    height: 207,
                },
                offset: [x as i16, 0],
                page: 0x40a,
            }],
        },
    ])
}
