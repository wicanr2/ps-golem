//! Bounded native pixel ownership, 026 §16.19, GPL-3.0-or-later.
//! Typed cache endpoint now; at most two immutable target snapshots on demand.
use super::{scene::NativeCommand, transfer::Rect, Gpu};
use std::sync::{Arc, OnceLock};
fn overlaps(a: Rect, b: Rect) -> bool {
    (a.x as u32) < b.x as u32 + b.width as u32
        && (b.x as u32) < a.x as u32 + a.width as u32
        && (a.y as u32) < b.y as u32 + b.height as u32
        && (b.y as u32) < a.y as u32 + a.height as u32
}
fn covers(a: Rect, b: Rect) -> bool {
    a.x <= b.x
        && a.y <= b.y
        && a.x as u32 + a.width as u32 >= b.x as u32 + b.width as u32
        && a.y as u32 + a.height as u32 >= b.y as u32 + b.height as u32
}
impl Gpu {
    pub fn deferred_target_count(&self) -> usize {
        self.deferred_pixels.len()
    }
    pub fn deferred_native_count(&self) -> usize {
        self.deferred_pixels
            .iter()
            .filter(|p| p.native_ready())
            .count()
    }
    /// No GP0 parsing and no native color writes. Rejection leaves the GPU intact.
    pub fn defer_native_scene(
        &mut self,
        commands: Vec<NativeCommand>,
        target: Rect,
    ) -> Result<(), String> {
        if self.deferred_pixels.len() >= 2
            || self
                .deferred_pixels
                .iter()
                .any(|p| overlaps(target, p.target()))
        {
            return Err("待產生原生區域重疊或超過雙緩衝界限".into());
        }
        let old_targets: Vec<_> = self.deferred_pixels.iter().map(|p| p.target()).collect();
        self.check_scene_reads(&commands, &old_targets)?;
        // Other pending targets cannot be read by this batch. Strip their ownership
        // from the isolated target input to prevent snapshot chains across frames.
        let mut input = self.clone();
        input.deferred_pixels.clear();
        input.observed_pixels = OnceLock::new();
        let prepared = Arc::new(input.prepare_native_scene(commands, target)?);
        let mut next = prepared.cache.clone();
        next.deferred_pixels = self.deferred_pixels.clone();
        next.deferred_pixels.push(prepared);
        next.observed_pixels = OnceLock::new();
        *self = next;
        Ok(())
    }
    pub(super) fn observed_native_pixels(&self) -> &[u16] {
        if self.deferred_pixels.is_empty() {
            return &self.pixels;
        }
        self.observed_pixels.get_or_init(|| {
            let mut pixels = self.pixels.clone();
            for pending in &self.deferred_pixels {
                let r = pending.target();
                let source = &pending.native().pixels;
                for y in r.y as usize..(r.y + r.height) as usize {
                    let at = y * 1024 + r.x as usize;
                    pixels[at..at + r.width as usize]
                        .copy_from_slice(&source[at..at + r.width as usize]);
                }
            }
            pixels
        })
    }
    /// Explicit writable ownership barrier. Metadata/cache state is not replayed.
    pub fn resolve_native_pixels(&mut self) {
        if !self.deferred_pixels.is_empty() {
            self.pixels = self.observed_native_pixels().to_vec();
            self.deferred_pixels.clear();
        }
        self.observed_pixels = OnceLock::new();
    }
    pub(super) fn prepare_native_pixel_write(&mut self, rect: Rect, full_overwrite: bool) {
        self.forget_scene_rect(rect);
        let all_rows = full_overwrite
            && (rect.y as u32..rect.y as u32 + rect.height as u32).all(|y| !self.line_skip(y));
        if self
            .deferred_pixels
            .iter()
            .any(|p| overlaps(rect, p.target()) && !(all_rows && covers(rect, p.target())))
        {
            self.resolve_native_pixels();
        } else if all_rows {
            self.deferred_pixels.retain(|p| !covers(rect, p.target()));
        }
        self.observed_pixels = OnceLock::new();
    }
}

/// Presentation identity survives native materialization; actual writes invalidate it.
#[derive(Clone)]
pub struct SceneToken(Arc<()>);
#[derive(Clone)]
pub(super) struct Owner {
    watch: Rect,
    token: SceneToken,
}
impl Gpu {
    pub fn scene_token(&mut self, target: Rect, watch: Rect) -> Option<SceneToken> {
        if watch.width == 0
            || watch.height == 0
            || watch.x as u32 + watch.width as u32 > 1024
            || watch.y as u32 + watch.height as u32 > 512
            || !covers(watch, target)
            || !self.deferred_pixels.iter().any(|p| p.target() == target)
        {
            return None;
        }
        let mut next: Vec<_> = self
            .scene_owners
            .iter()
            .filter(|p| !overlaps(p.watch, watch))
            .cloned()
            .collect();
        if next.len() >= 2 {
            return None;
        }
        let token = SceneToken(Arc::new(()));
        next.push(Owner {
            watch,
            token: token.clone(),
        });
        self.scene_owners = next;
        Some(token)
    }
    pub fn scene_token_alive(&self, token: &SceneToken) -> bool {
        self.scene_owners
            .iter()
            .any(|p| Arc::ptr_eq(&p.token.0, &token.0))
    }
    pub(super) fn forget_scene_rect(&mut self, r: Rect) {
        self.scene_owners.retain(|p| !overlaps(p.watch, r));
    }
    pub(super) fn forget_scene_pixel(&mut self, x: u32, y: u32) {
        if self.scene_owners.is_empty() {
            return;
        }
        let (x, y) = (x & 1023, y & 511);
        self.scene_owners.retain(|p| {
            !(x >= p.watch.x as u32
                && x < p.watch.x as u32 + p.watch.width as u32
                && y >= p.watch.y as u32
                && y < p.watch.y as u32 + p.watch.height as u32)
        });
    }
}
#[cfg(test)]
mod token_tests {
    use super::*;
    fn pending() -> (Gpu, Rect, SceneToken) {
        let mut g = Gpu::new();
        let r = Rect {
            x: 0,
            y: 20,
            width: 256,
            height: 207,
        };
        g.clip_x0 = 0;
        g.clip_y0 = 20;
        g.clip_x1 = 255;
        g.clip_y1 = 226;
        g.defer_native_scene(vec![NativeCommand::Mode { page: 0 }], r)
            .unwrap();
        let t = g
            .scene_token(
                r,
                Rect {
                    x: 0,
                    y: 0,
                    width: 256,
                    height: 240,
                },
            )
            .unwrap();
        assert_eq!(g.deferred_native_count(), 0);
        (g, r, t)
    }
    #[test]
    fn ownership_demand_and_write_barriers() {
        let (mut g, r, t) = pending();
        let _ = g.vram();
        assert!(g.scene_token_alive(&t));
        g.gp1(0x03000000);
        assert!(g.scene_token_alive(&t));
        g.prepare_native_pixel_write(
            Rect {
                x: 700,
                y: 300,
                width: 1,
                height: 1,
            },
            true,
        );
        assert!(g.scene_token_alive(&t));
        g.prepare_native_pixel_write(r, true);
        assert!(!g.scene_token_alive(&t));
        let (mut g, _, t) = pending();
        g.prepare_native_pixel_write(
            Rect {
                x: 1,
                y: 21,
                width: 1,
                height: 1,
            },
            false,
        );
        assert!(!g.scene_token_alive(&t));
        let (mut g, _, t) = pending();
        g.gp0(0xa0000000);
        g.gp0(700 | (300 << 16));
        g.gp0(1 | (1 << 16));
        g.gp0(0x1234);
        assert_eq!(g.deferred_target_count(), 0);
        assert!(g.scene_token_alive(&t));
        g.gp0(0xc0000000);
        g.gp0(0 | (20 << 16));
        g.gp0(1 | (1 << 16));
        g.read_data();
        assert!(g.scene_token_alive(&t));
        g.gp0(0x020000ff);
        g.gp0(0);
        g.gp0(0x00100010);
        assert!(!g.scene_token_alive(&t));
        let (mut g, _, t) = pending();
        g.resolve_native_pixels();
        assert!(g.scene_token_alive(&t));
        g.put(1024, 532, 1);
        assert!(!g.scene_token_alive(&t));
        let (mut g, _, t) = pending();
        g.resolve_native_pixels();
        g.plot(1, 21, 1, -1, false);
        assert!(!g.scene_token_alive(&t));
    }
    #[test]
    fn bounded_observers_survive_two_targets() {
        let (mut g, _, a) = pending();
        g.resolve_native_pixels();
        let r = Rect {
            x: 256,
            y: 20,
            width: 256,
            height: 207,
        };
        g.clip_x0 = 256;
        g.clip_x1 = 511;
        g.defer_native_scene(vec![NativeCommand::Mode { page: 0 }], r)
            .unwrap();
        let b = g
            .scene_token(
                r,
                Rect {
                    x: 256,
                    y: 0,
                    width: 256,
                    height: 240,
                },
            )
            .unwrap();
        g.resolve_native_pixels();
        assert!(g.scene_token_alive(&a) && g.scene_token_alive(&b));
        let r = Rect {
            x: 512,
            y: 20,
            width: 256,
            height: 207,
        };
        g.clip_x0 = 512;
        g.clip_x1 = 767;
        g.defer_native_scene(vec![NativeCommand::Mode { page: 0 }], r)
            .unwrap();
        assert!(g
            .scene_token(
                r,
                Rect {
                    x: 512,
                    y: 0,
                    width: 256,
                    height: 240
                }
            )
            .is_none());
        assert!(g.scene_token_alive(&a) && g.scene_token_alive(&b));
        g.put(255, 239, 1);
        assert!(!g.scene_token_alive(&a) && g.scene_token_alive(&b));
        g.put(256, 240, 1);
        assert!(g.scene_token_alive(&b));
        g.put(256, 239, 1);
        assert!(!g.scene_token_alive(&b));
    }
}
