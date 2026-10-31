//! Rational UV planes, 022 / 026 §16.9. GPL-3.0-or-later.
//! Port of own render/hd/perspective.go at 4439672dc9d609c4d6a0e7b7038c874da83fe053.
use super::Vertex;
#[derive(Clone, Copy)]
struct Plane {
    base: f64,
    dx: f64,
    dy: f64,
}
impl Plane {
    fn new(v: [Vertex; 3], a: [f64; 3]) -> Self {
        let (x0, y0) = (v[0].x as f64, v[0].y as f64);
        let (x1, y1) = ((v[1].x - v[0].x) as f64, (v[1].y - v[0].y) as f64);
        let (x2, y2) = ((v[2].x - v[0].x) as f64, (v[2].y - v[0].y) as f64);
        let den = x1 * y2 - x2 * y1;
        let dx = ((a[1] - a[0]) * y2 - (a[2] - a[0]) * y1) / den;
        let dy = (x1 * (a[2] - a[0]) - x2 * (a[1] - a[0])) / den;
        Self {
            base: a[0] - dx * x0 - dy * y0,
            dx,
            dy,
        }
    }
    fn at(self, x: f64, y: f64) -> f64 {
        self.base + self.dx * x + self.dy * y
    }
}
#[derive(Clone, Copy)]
pub(super) struct Perspective {
    u: Plane,
    v: Plane,
    q: Plane,
}
impl Perspective {
    pub(super) fn new(v: [Vertex; 3]) -> Option<Self> {
        if v.iter().any(|p| !p.z.is_finite() || p.z <= 0.0)
            || (v[0].z == v[1].z && v[1].z == v[2].z)
        {
            return None;
        }
        let q = v.map(|p| 1.0 / p.z);
        Some(Self {
            u: Plane::new(v, std::array::from_fn(|i| v[i].u as f64 * q[i])),
            v: Plane::new(v, std::array::from_fn(|i| v[i].v as f64 * q[i])),
            q: Plane::new(v, q),
        })
    }
    pub(super) fn sample(self, x: i32, y: i32, affine: [u32; 2]) -> [u32; 2] {
        let (x, y) = (x as f64 + 0.5, y as f64 + 0.5);
        let q = self.q.at(x, y);
        if !q.is_finite() || q <= 0.0 {
            return affine;
        }
        let uv = [self.u.at(x, y) / q, self.v.at(x, y) / q];
        if uv.iter().any(|v| !v.is_finite() || v.abs() > 65536.0) {
            return affine;
        }
        uv.map(|v| (v * 16777216.0).floor() as i64 as u32)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn rational_planes_and_unreliable_depth() {
        let v = [
            Vertex {
                x: 0,
                y: 0,
                u: 0,
                v: 0,
                z: 1.0,
                ..Vertex::default()
            },
            Vertex {
                x: 4,
                y: 0,
                u: 4,
                v: 0,
                z: 2.0,
                ..Vertex::default()
            },
            Vertex {
                x: 0,
                y: 4,
                u: 0,
                v: 4,
                z: 4.0,
                ..Vertex::default()
            },
        ];
        let p = Perspective::new(v).unwrap();
        // Barycentric weights (3/4,1/8,1/8) give UV=(8/27,4/27).
        assert_eq!(
            p.sample(0, 0, [999, 999]),
            [
                (8u64 * (1 << 24) / 27) as u32,
                (4u64 * (1 << 24) / 27) as u32
            ]
        );
        // At (1.5,.5), weights (1/2,3/8,1/8) give UV=(24/23,4/23).
        assert_eq!(
            p.sample(1, 0, [999, 999]),
            [
                (24u64 * (1 << 24) / 23) as u32,
                (4u64 * (1 << 24) / 23) as u32
            ]
        );
        let mut invalid = v;
        invalid[0].z = 0.0;
        assert!(Perspective::new(invalid).is_none());
        invalid[0].z = f64::NAN;
        assert!(Perspective::new(invalid).is_none());
        assert!(Perspective::new(v.map(|p| Vertex { z: 1.0, ..p })).is_none());
        assert_eq!(p.sample(20, 20, [7, 9]), [7, 9]);
    }
}
