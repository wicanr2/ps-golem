//! Typed presentation camera, 022 and 026 §16.9. GPL-3.0-or-later.
//! Own Go internal/gte/pgxp.go at 4439672; native fixed-point contract from
//! internal/gte/gte.go. No guest instruction execution or architectural writes.
#[derive(Clone, Copy, Debug)]
pub struct Camera {
    pub rotation: [[i16; 3]; 3],
    pub translation: [i32; 3],
    pub offset: [i32; 2],
    pub distance: u16,
}
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct Projection {
    pub position: [f64; 2],
    /// Zero means unreliable: preserve affine sampling and native coordinates.
    pub depth: f64,
}
impl Camera {
    /// SF=12 / LM=0 projection only. Other command domains stay with their caller.
    pub fn project(&self, vector: [i16; 3], native: [i16; 2]) -> Projection {
        let mut accum = [0i64; 3];
        for (row, acc) in accum.iter_mut().enumerate() {
            *acc = (self.translation[row] as i64) << 12;
            for (&r, &v) in self.rotation[row].iter().zip(&vector) {
                *acc = (*acc + r as i64 * v as i64).wrapping_shl(20) >> 20;
            }
        }
        let h = self.distance as f32;
        let depth = (accum[2] as f64 / 4096.0) as f32;
        let ir = accum.map(|v| ((v >> 12) as i32).clamp(-32768, 32767));
        let denominator = depth.max(h / 2.0);
        let ratio = if denominator > 0.0 {
            h / denominator
        } else {
            131071.0 / 65536.0
        };
        let xy = [
            self.offset[0] as f32 / 65536.0 + ir[0] as f32 * ratio,
            self.offset[1] as f32 / 65536.0 + ir[1] as f32 * ratio,
        ];
        if depth <= h / 2.0
            || depth <= 0.0
            || xy
                .iter()
                .any(|v| !v.is_finite() || *v < -1024.0 || *v > 1023.0)
        {
            Projection {
                position: native.map(|v| v as f64),
                depth: 0.0,
            }
        } else {
            Projection {
                position: xy.map(|v| v as f64),
                depth: depth as f64,
            }
        }
    }
}

// Direct integer fallback. Own gte.rs at 8a1544d, SF12 / LM0 position subset.
// No GTE execute call, instruction dispatch, register state or architectural flags.
const fn native_reciprocals() -> [u8; 257] {
    let mut table = [0; 257];
    let mut divisor = 0x8000u32;
    while divisor < 0x10000 {
        let mut x = 512u32;
        let mut i = 1;
        while i < 5 {
            x = x.wrapping_mul((1024u32 * 512).wrapping_sub((divisor >> 7) * x)) >> 18;
            i += 1;
        }
        table[((divisor >> 7) & 255) as usize] = ((x + 1) >> 1).wrapping_sub(0x101) as u8;
        divisor += 0x80;
    }
    table[256] = table[255];
    table
}
const RECIPROCALS: [u8; 257] = native_reciprocals();
impl Camera {
    /// Project directly from model data, including native-coordinate fallback.
    pub fn project_vertex(&self, vector: [i16; 3]) -> Projection {
        self.project(vector, self.native_xy(vector))
    }
    /// Integer SF12 / LM0 screen coordinates for fallback/compatibility only.
    pub fn native_xy(&self, vector: [i16; 3]) -> [i16; 2] {
        let mut accum = [0i64; 3];
        for (row, acc) in accum.iter_mut().enumerate() {
            *acc = (self.translation[row] as i64) << 12;
            for (&r, &v) in self.rotation[row].iter().zip(&vector) {
                *acc = (*acc + r as i64 * v as i64).wrapping_shl(20) >> 20;
            }
        }
        let ir = accum.map(|v| ((v >> 12) as i32).clamp(-32768, 32767));
        let mut divisor = ((accum[2] >> 12) as i32).clamp(0, 65535) as u32;
        let mut dividend = self.distance as u32;
        let ratio = if divisor * 2 > dividend {
            let shift = (divisor as u16).leading_zeros();
            dividend <<= shift;
            divisor <<= shift;
            let d = (divisor | 0x8000) as u16;
            let x = 0x101 + RECIPROCALS[(((d as u32 & 0x7fff) + 0x40) >> 7) as usize] as i32;
            let tmp = (-(d as i32) * x + 0x80) >> 8;
            let reciprocal = (x * (131072 + tmp) + 0x80) >> 8;
            (((dividend as u64) * (reciprocal as i64 as u64) + 32768) >> 16).min(0x1ffff) as i64
        } else {
            0x1ffff
        };
        std::array::from_fn(|i| {
            (((self.offset[i] as i64 + ir[i] as i64 * ratio) >> 16) as i32).clamp(-1024, 1023)
                as i16
        })
    }
}

#[cfg(test)]
mod model_tests {
    use super::*;
    #[test]
    fn owned_model_projection_and_fallback_match_native_positions() {
        let c = Camera {
            rotation: [[0, 0, 4096], [0, 4096, 0], [-4096, 0, 0]],
            translation: [0, 0, 512],
            offset: [128 << 16; 2],
            distance: 256,
        };
        for v in [[0, 0, 0], [32, -32, 1], [30000, -30000, -1]] {
            let mut g = crate::gte::Gte::new();
            let r = c.rotation;
            let pack = |a: i16, b: i16| a as u16 as u32 | ((b as u16 as u32) << 16);
            for (i, value) in [
                pack(r[0][0], r[0][1]),
                pack(r[0][2], r[1][0]),
                pack(r[1][1], r[1][2]),
                pack(r[2][0], r[2][1]),
                r[2][2] as u16 as u32,
            ]
            .into_iter()
            .enumerate()
            {
                g.write(32 + i, value);
            }
            for i in 0..3 {
                g.write(37 + i, c.translation[i] as u32);
            }
            g.write(56, c.offset[0] as u32);
            g.write(57, c.offset[1] as u32);
            g.write(58, c.distance as u32);
            g.write(0, pack(v[0], v[1]));
            g.write(1, v[2] as u16 as u32);
            g.execute(0x80001);
            let raw = g.read(14);
            let xy = [raw as i16, (raw >> 16) as i16];
            assert_eq!(c.native_xy(v), xy);
            assert_eq!(c.project_vertex(v), c.project(v, xy));
        }
        assert_eq!(c.project_vertex([30000, -30000, -1]).depth, 0.0);
    }
}
