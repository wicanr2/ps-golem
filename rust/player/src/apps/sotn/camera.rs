//! Camera events and mesh scroll presentation, 026 §16.9.3, GPL-3.0-or-later.
//! Evidence: original RotMatrix 800186AC, ST0 801BAFB4, DRA 800F3364–800F352C.
//! Typed math/data only; no CPU execution, GTE control snapshots or old SXY.
use psgolem_core::{cpu::State, projection::Camera};
use sha2::{Digest, Sha256};
type Result<T> = std::result::Result<T, String>;
const IDS: &[(usize, usize, &str)] = &[
    (
        0x186ac,
        0x28c,
        "bb15388a4382acfb4f166213a27097561e4d921bdb3dc2f06d092e7b9ede3467",
    ),
    (
        0x2e210,
        0x4000,
        "b106234165685d79392c926192d1b2f9e30125307abde6040747e401a35d0e87",
    ),
    (
        0xf26e0,
        0x1e38,
        "1366c601cf1cb461c4dc23374c722bede2d96105baee57ee50fb315ecba0908e",
    ),
    (
        0x1baeb0,
        0x4b8,
        "5333d6333987ffe96072d776de76884ca22c889900cda0d2ce3771dc1e6b1e26",
    ),
];
pub struct Sources<'a> {
    ram: &'a [u8],
}
fn half(r: &[u8], p: usize) -> u16 {
    u16::from_le_bytes(r[p..p + 2].try_into().unwrap())
}
impl<'a> Sources<'a> {
    pub fn new(ram: &'a [u8]) -> Result<Self> {
        if ram.len() != 2 << 20
            || IDS
                .iter()
                .any(|&(a, n, h)| format!("{:x}", Sha256::digest(&ram[a..a + n])) != h)
        {
            return Err("鐘塔相機程式或原查表來源不符".into());
        }
        Ok(Self { ram })
    }
    fn trig(&self, angle: i16) -> (i32, i32) {
        let angle = angle as i32;
        let p = (angle.abs() as usize & 4095) * 4 + 0x2e210;
        let s = half(self.ram, p) as i16 as i32;
        let c = half(self.ram, p + 2) as i16 as i32;
        (if angle < 0 { -s } else { s }, c)
    }
    /// Original fixed-point Rx*Ry*Rz order, retaining each intermediate shift.
    pub fn rotation(&self, angles: [i16; 3]) -> [[i16; 3]; 3] {
        let (sx, cx) = self.trig(angles[0]);
        let (sy, cy) = self.trig(angles[1]);
        let (sz, cz) = self.trig(angles[2]);
        let mul = |a: i32, b: i32| a.wrapping_mul(b) >> 12;
        let neg_mul = |a: i32, b: i32| a.wrapping_mul(b).wrapping_neg() >> 12;
        let zc = mul(cz, -sy);
        let zs = mul(sz, -sy);
        [
            [mul(cz, cy) as i16, neg_mul(sz, cy) as i16, sy as i16],
            [
                (mul(sz, cx) - mul(zc, sx)) as i16,
                (mul(cz, cx) + mul(zs, sx)) as i16,
                neg_mul(cy, sx) as i16,
            ],
            [
                (mul(sz, sx) + mul(zc, cx)) as i16,
                (mul(cz, sx) - mul(zs, cx)) as i16,
                mul(cy, cx) as i16,
            ],
        ]
    }
    /// Call at the observed camera-setup boundary, before later entity scrolling.
    pub fn camera_at_event(&self, entry: &State) -> Result<Camera> {
        let e = entry.gpr[18];
        if entry.pc != 0x801bafb4
            || !(0x800733d8..0x8007efd8).contains(&e)
            || (e - 0x800733d8) % 0xbc != 0
        {
            return Err("鐘塔相機事件或物件來源不符".into());
        }
        let p = (e - 0x80000000) as usize;
        let angles = [
            half(self.ram, p + 0x9c) as i16,
            (half(self.ram, p + 0x9e) as i16).wrapping_sub(0x90),
            half(self.ram, p + 0xa0) as i16,
        ];
        Ok(Camera {
            rotation: self.rotation(angles),
            translation: [
                half(self.ram, p + 2) as i16 as i32 - 128,
                half(self.ram, p + 6) as i16 as i32 - 128,
                half(self.ram, p + 0xae) as i16 as i32 + 0x580,
            ],
            offset: [128 << 16; 2],
            distance: 0x300,
        })
    }
    /// Read once after the native scroll phase. The ordinal is the allocator's
    /// persistent handle, never recovered from old projected coordinates.
    /// Only mesh kinds 4/5 are covered; caller owns frame epoch and application.
    pub fn mesh_scroll_after_event(
        &self,
        entry: &State,
        ordinal: usize,
        flags: u16,
    ) -> Result<[i16; 2]> {
        if entry.pc != 0x800f352c || entry.gpr[16] != 1024 || ordinal >= 1280 {
            return Err("鐘塔捲動事件或圖元範圍不符".into());
        }
        if ordinal >= 1024 {
            return Ok([0, 0]);
        }
        Ok(if flags & 2 != 0 {
            [
                half(self.ram, 0x136850) as i16,
                half(self.ram, 0x136854) as i16,
            ]
        } else if flags & 0x200 != 0 {
            [
                0u16.wrapping_sub(half(self.ram, 0x136840)) as i16,
                0u16.wrapping_sub(half(self.ram, 0x136844)) as i16,
            ]
        } else {
            [0, 0]
        })
    }
}
