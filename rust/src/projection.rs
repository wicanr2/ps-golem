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
