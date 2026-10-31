//! Original-table rotation math, 026 §16.7.16, GPL-3.0-or-later.
//! Original IDA9.4 locations and hashes are evidence, not executable ISA.
use sha2::{Digest, Sha256};
type Result<T> = std::result::Result<T, String>;
pub struct Tables<'a> {
    ram: &'a [u8],
}
#[derive(Clone, Copy, Debug, Default)]
pub struct Point {
    pub dx: i16,
    pub dy: i16,
    pub lzcs: u32,
    pub theta: i32,
    pub cos_q8: i32,
    pub sin_q8: i32,
}
#[derive(Clone, Copy, Debug)]
pub struct Rotated {
    pub position: [u16; 2],
    /// Required GTE LZCS input. It is data only here; no GTE is mutated.
    pub lzcs: u32,
    pub mantissa: Option<u32>,
    pub product: i64,
    pub point: Point,
}
const IDENTITIES: &[(usize, usize, &str)] = &[
    (
        0x1721c,
        148,
        "f504218ef1c81002a5ef5e731d51d666a2528cbd4386a86409e3f3c21efe6233",
    ),
    (
        0x190ac,
        384,
        "ffcce9e7b5ce6a86e3505f104e54ba5ba174b2487abf65d8e194d851ca8bdbf7",
    ),
    (
        0x16d68,
        160,
        "54c123e439ed7e1b94ad11da95034828efeb5b915a52bdd78dd14fe96ecc77f5",
    ),
    (
        0x16c9c,
        60,
        "5d9fdd4b7dd1e8b57a7744c8e828f325f271bf2903c5274093fcf93369515344",
    ),
    (
        0x16cd8,
        144,
        "b6ca5734561df50fbca266db843ce0142566047a2eedfbd74d6cadd3fc90db81",
    ),
    (
        0x2d3cc,
        2050,
        "bab3b71ba7f79dfb2b5cebb71f8382a79606a958d72031bc11b88066c36033bd",
    ),
    (
        0x2dbe0,
        384,
        "45ca1b7b619b33961c03c82243b8baf59fc50c49b4d7eefc94b76edbd417a5c8",
    ),
    (
        0x32210,
        2050,
        "b95a5540f30e1f0f40975157595d078bb07d3242d2c856a1c679e533e2f26233",
    ),
];
impl<'a> Tables<'a> {
    pub fn new(ram: &'a [u8]) -> Result<Self> {
        if ram.len() != 2 << 20
            || IDENTITIES
                .iter()
                .any(|&(a, n, h)| format!("{:x}", Sha256::digest(&ram[a..a + n])) != h)
        {
            return Err("旋轉數學來源或查表身份不符".into());
        }
        Ok(Self { ram })
    }
    fn half(&self, a: usize) -> i32 {
        i16::from_le_bytes(self.ram[a..a + 2].try_into().unwrap()) as i32
    }
    fn quarter(&self, i: usize) -> i32 {
        self.half(0x2d3cc + i * 2)
    }
    pub fn sin(&self, angle: i32) -> i32 {
        let p = (if angle < 0 {
            angle.wrapping_neg()
        } else {
            angle
        } as u32
            & 4095) as usize;
        let v = match p {
            0..=1024 => self.quarter(p),
            1025..=2048 => self.quarter(2048 - p),
            2049..=3072 => -self.quarter(p - 2048),
            _ => -self.quarter(4096 - p),
        };
        if angle < 0 {
            -v
        } else {
            v
        }
    }
    pub fn cos(&self, angle: i32) -> i32 {
        let p = (if angle < 0 {
            angle.wrapping_neg()
        } else {
            angle
        } as u32
            & 4095) as usize;
        match p {
            0..=1024 => self.quarter(1024 - p),
            1025..=2048 => -self.quarter(p - 1024),
            2049..=3072 => -self.quarter(3072 - p),
            _ => self.quarter(p - 3072),
        }
    }
    /// The caller's coordinate deltas are signed 16-bit values. The original
    /// wide-argument overflow/division branches are outside this contract.
    pub fn atan(&self, y: i16, x: i16) -> i32 {
        let ax = (x as i32).abs();
        let ay = (y as i32).abs();
        if ax == 0 && ay == 0 {
            return 0;
        }
        let mut a = if ay < ax {
            self.half(0x32210 + ((ay << 10) / ax) as usize * 2)
        } else {
            1024 - self.half(0x32210 + ((ax << 10) / ay) as usize * 2)
        };
        if x < 0 {
            a = 2048 - a;
        }
        if y < 0 {
            -a
        } else {
            a
        }
    }
    /// Original 192-entry integer approximation to sqrt(input)*64.
    /// The game caller supplies a nonnegative fixed-point squared radius.
    pub fn root_q6(&self, input: u32) -> Result<u32> {
        if input & 0x80000000 != 0 {
            return Err("旋轉負半徑查表分支未封閉".into());
        }
        if input == 0 {
            return Ok(0);
        }
        let even = input.leading_zeros() & !1;
        let normalized = if even >= 24 {
            input << (even - 24)
        } else {
            input >> (24 - even)
        };
        if !(64..256).contains(&normalized) {
            return Err("旋轉半徑索引超界".into());
        }
        let value = self.half(0x2dbe0 + (normalized as usize - 64) * 2) as u32;
        let exponent = (19 - even as i32) >> 1;
        Ok(if exponent >= 0 {
            value << exponent as u32
        } else {
            value >> (-exponent) as u32
        })
    }
    pub fn rotate(
        &self,
        position: [u16; 2],
        pivot: [u16; 2],
        angle: i16,
        mirrored: bool,
    ) -> Result<Rotated> {
        let dx = position[0].wrapping_sub(pivot[0]) as i16;
        let dy = position[1].wrapping_sub(pivot[1]) as i16;
        let squared = (dx as i32)
            .wrapping_mul(dx as i32)
            .wrapping_add((dy as i32).wrapping_mul(dy as i32));
        let lzcs = (squared as u32).wrapping_shl(12);
        let radius = self.root_q6(lzcs)? as i32;
        let mantissa = if lzcs == 0 {
            None
        } else {
            let even = lzcs.leading_zeros() & !1;
            let n = if even >= 24 {
                lzcs << (even - 24)
            } else {
                lzcs >> (24 - even)
            };
            Some(self.half(0x2dbe0 + (n as usize - 64) * 2) as u32)
        };
        let theta = self.atan(dy, dx)
            + if mirrored {
                -(angle as i32)
            } else {
                angle as i32
            };
        let cos_q8 = self.cos(theta) >> 4;
        let sin_q8 = self.sin(theta) >> 4;
        let x = cos_q8.wrapping_mul(radius).wrapping_add(0x80000) >> 20;
        let y = sin_q8.wrapping_mul(radius).wrapping_add(0x80000) >> 20;
        Ok(Rotated {
            position: [
                pivot[0].wrapping_add(x as u16),
                pivot[1].wrapping_add(y as u16),
            ],
            lzcs,
            mantissa,
            product: sin_q8 as i64 * radius as i64,
            point: Point {
                dx,
                dy,
                lzcs,
                theta,
                cos_q8,
                sin_q8,
            },
        })
    }
}
