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
