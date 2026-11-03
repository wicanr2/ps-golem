//! Read only requested native words, 026 §16.20.1, GPL-3.0-or-later.
//! A word inside a deferred target is an explicit demand for that target.
use super::Gpu;
pub trait VramSource {
    fn word_count(&self) -> usize;
    fn source_word(&self, index: usize) -> u16;
}
impl VramSource for [u16] {
    fn word_count(&self) -> usize {
        self.len()
    }
    fn source_word(&self, index: usize) -> u16 {
        self[index]
    }
}
impl VramSource for Vec<u16> {
    fn word_count(&self) -> usize {
        self.len()
    }
    fn source_word(&self, index: usize) -> u16 {
        self[index]
    }
}
impl<const N: usize> VramSource for [u16; N] {
    fn word_count(&self) -> usize {
        N
    }
    fn source_word(&self, index: usize) -> u16 {
        self[index]
    }
}
impl VramSource for Gpu {
    fn word_count(&self) -> usize {
        self.pixels.len()
    }
    fn source_word(&self, index: usize) -> u16 {
        assert!(index < self.pixels.len(), "VRAM word index out of range");
        let x = index % 1024;
        let y = index / 1024;
        for p in &self.deferred_pixels {
            let r = p.target();
            if x >= r.x as usize
                && x < r.x as usize + r.width as usize
                && y >= r.y as usize
                && y < r.y as usize + r.height as usize
            {
                return p.native().pixels[index];
            }
        }
        self.pixels[index]
    }
}
