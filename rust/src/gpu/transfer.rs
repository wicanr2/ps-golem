//! Direct SDK-style texture jobs, spec026 §16.6, GPL-3.0-or-later.
//! Own GPU transfer/primitives contract at 3cab34a. No command parsing.
use super::{Gpu, NONE};

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct Rect {
    pub x: u16,
    pub y: u16,
    pub width: u16,
    pub height: u16,
}
impl Rect {
    fn valid(self) -> bool {
        self.width > 0
            && self.width < 1024
            && self.height > 0
            && self.height < 512
            && self.x as usize + self.width as usize <= 1024
            && self.y as usize + self.height as usize <= 512
    }
    fn xy(self) -> u32 {
        self.x as u32 | (self.y as u32) << 16
    }
    fn wh(self) -> u32 {
        self.width as u32 | (self.height as u32) << 16
    }
}
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum TextureJob {
    Upload { rect: Rect, words: Vec<u16> },
    Clear { rect: Rect },
}
impl TextureJob {
    pub fn rect(&self) -> Rect {
        match self {
            Self::Upload { rect, .. } | Self::Clear { rect } => *rect,
        }
    }
}
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct TextureContext {
    pub area_start: [u16; 2],
    pub area_end: [u16; 2],
    pub offset: [i16; 2],
    pub information_high: u32,
}
impl Gpu {
    pub fn texture_context(&self) -> TextureContext {
        TextureContext {
            area_start: [self.clip_x0 as u16, self.clip_y0 as u16],
            area_end: [self.clip_x1 as u16, self.clip_y1 as u16],
            offset: [self.off_x as i16, self.off_y as i16],
            information_high: self.read_buffer & 0xffc00000,
        }
    }

    /// A synchronous data operation only. Machine DMA/IRQ and timing are
    /// caller contracts. Success retains all GPU transfer/cache/ABI effects;
    /// rejection leaves every GPU field unchanged, including warm caches.
    pub fn apply_texture_jobs(&mut self, jobs: &[TextureJob]) -> Result<(), String> {
        if self.in_cmd != NONE || self.len != 0 || self.need != 0 || jobs.len() > 768 {
            return Err("GPU傳輸邊界或作業數不符".into());
        }
        let mut total = 0usize;
        for j in jobs {
            let rect = j.rect();
            if !rect.valid() {
                return Err("顯存矩形空白或越界".into());
            }
            match j {
                TextureJob::Upload { words, .. } => {
                    if words.len() != rect.width as usize * rect.height as usize {
                        return Err("圖像word長度不符".into());
                    }
                    total = total.checked_add(words.len()).ok_or("圖像總量溢位")?;
                }
                TextureJob::Clear { .. } if self.read_buffer & 0xffc00000 != 0 => {
                    return Err("SDK資訊高位未驗證".into())
                }
                _ => (),
            }
        }
        if total > 524288 {
            return Err("圖像作業總量超界".into());
        }
        if jobs.is_empty() {
            return Ok(());
        }
        let mut candidate = self.clone();
        for j in jobs {
            match j {
                TextureJob::Upload { rect, words } => {
                    // SDK cache flush + A0 completed transfer. Derive the
                    // compatibility endpoint directly, without GP0/GP1.
                    candidate.invalidate_cache();
                    for y in 0..rect.height as usize {
                        for x in 0..rect.width as usize {
                            let at = (rect.y as usize + y) * 1024 + rect.x as usize + x;
                            if candidate.pixels[at] & candidate.mask_and == 0 {
                                candidate.pixels[at] =
                                    words[y * rect.width as usize + x] | candidate.mask_or;
                            }
                        }
                    }
                    candidate.fb_x = rect.x as u32;
                    candidate.fb_y = rect.y as u32;
                    candidate.fb_w = rect.width as u32;
                    candidate.fb_h = rect.height as u32;
                    candidate.cur_x = rect.x as u32;
                    candidate.cur_y = rect.y as u32 + rect.height as u32;
                    candidate.buf[0] = 0xa0000000;
                    candidate.buf[1] = rect.xy();
                    candidate.buf[2] = rect.wh();
                    candidate.dma = if words.len().div_ceil(2) >= 16 { 2 } else { 0 };
                }
                TextureJob::Clear { rect } => {
                    // SDK temporarily uses full area/zero offset and clears
                    // mask bits. The saved area/offset is then restored.
                    for y in rect.y as usize..rect.y as usize + rect.height as usize {
                        if candidate.line_skip(y as u32) {
                            continue;
                        }
                        let at = y * 1024 + rect.x as usize;
                        candidate.pixels[at..at + rect.width as usize].fill(0);
                    }
                    candidate.mask_or = 0;
                    candidate.mask_and = 0;
                    let off =
                        (candidate.off_x as u32 & 2047) | ((candidate.off_y as u32 & 2047) << 11);
                    if rect.x & 63 == 0 && rect.width & 63 == 0 {
                        candidate.buf[0] = 0x02000000;
                    } else {
                        candidate.read_buffer = off;
                        candidate.buf[0] = 0xe5000000 | off;
                    }
                    candidate.buf[1] = rect.xy();
                    candidate.buf[2] = rect.wh();
                    candidate.dma = 2;
                }
            }
        }
        *self = candidate;
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    fn rect() -> Rect {
        Rect {
            x: 20,
            y: 12,
            width: 3,
            height: 2,
        }
    }
    fn upload(g: &mut Gpu, r: Rect, w: &[u16]) {
        g.gp1(0x04000000);
        g.gp0(0x01000000);
        for n in [0xa0000000, r.xy(), r.wh()] {
            g.gp0(n);
        }
        for v in w.chunks(2) {
            g.gp0(v[0] as u32 | ((v.get(1).copied().unwrap_or(0) as u32) << 16));
        }
        if w.len().div_ceil(2) >= 16 {
            g.gp1(0x04000002);
        }
    }
    #[test]
    fn direct_upload_preserves_mask_cursor_and_cache_contract() {
        let mut a = Gpu::new();
        let r = rect();
        upload(&mut a, r, &[0x8001, 3, 4, 5, 6, 7]);
        a.gp0(0xe6000003);
        let mut b = a.clone();
        let w = vec![12, 13, 14, 15, 16, 17];
        a.apply_texture_jobs(&[TextureJob::Upload {
            rect: r,
            words: w.clone(),
        }])
        .unwrap();
        upload(&mut b, r, &w);
        assert_eq!(a.pixels, b.pixels);
        assert_eq!(a.buf, b.buf);
        assert_eq!(
            (a.cur_x, a.cur_y, a.fb_w, a.fb_h, a.dma),
            (b.cur_x, b.cur_y, b.fb_w, b.fb_h, b.dma)
        );
        assert_eq!((a.clut_address, a.clut_count), (u32::MAX, 0));
        assert!(a.texture.iter().all(|l| l.tag == u32::MAX));
    }
    #[test]
    fn clear_is_exact_and_restores_area_and_offset() {
        let mut g = Gpu::new();
        g.pixels.fill(0xffff);
        g.gp0(0xe3005100);
        g.gp0(0xe40389ff);
        g.gp0(0xe5000100);
        g.apply_texture_jobs(&[TextureJob::Clear { rect: rect() }])
            .unwrap();
        assert_eq!(g.pixels.iter().filter(|&&v| v == 0).count(), 6);
        assert_eq!(
            (g.clip_x0, g.clip_y0, g.clip_x1, g.clip_y1, g.off_x, g.off_y),
            (256, 20, 511, 226, 256, 0)
        );
        assert_eq!(g.read_buffer, 256);
        assert_eq!(g.buf[..3], [0xe5000100, 12 << 16 | 20, 2 << 16 | 3]);
    }
    #[test]
    fn short_sdk_upload_has_a_cpu_only_dma_boundary() {
        for (count, expected_dma) in [(30, 0), (31, 2), (32, 2)] {
            let mut a = Gpu::new();
            let mut b = a.clone();
            let r = Rect {
                x: 5,
                y: 5,
                width: count,
                height: 1,
            };
            let words = vec![0x1234; count as usize];
            a.apply_texture_jobs(&[TextureJob::Upload {
                rect: r,
                words: words.clone(),
            }])
            .unwrap();
            upload(&mut b, r, &words);
            assert_eq!(a.dma, expected_dma);
            assert_eq!(a.dma, b.dma);
            assert_eq!(a.pixels, b.pixels);
            assert_eq!(a.buf, b.buf);
        }
    }
    #[test]
    fn aligned_sdk_clear_keeps_readback_and_uses_fill_endpoint() {
        let mut a = Gpu::new();
        a.pixels.fill(0xffff);
        a.gp1(0x10000007);
        let ctx = a.texture_context();
        let mut b = a.clone();
        let r = Rect {
            x: 64,
            y: 10,
            width: 64,
            height: 5,
        };
        a.apply_texture_jobs(&[TextureJob::Clear { rect: r }])
            .unwrap();
        b.gp0(0xe6000000);
        b.gp0(0x02000000);
        b.gp0(r.xy());
        b.gp0(r.wh());
        b.gp1(0x04000002);
        assert_eq!(a.pixels, b.pixels);
        assert_eq!(a.buf, b.buf);
        assert_eq!(a.read_buffer, 2);
        assert_eq!(a.read_buffer, b.read_buffer);
        assert_eq!(a.texture_context(), ctx);
    }
    #[test]
    fn bad_later_job_and_active_fifo_do_not_mutate_gpu() {
        let mut g = Gpu::new();
        let before = g.pixels.clone();
        let jobs = [
            TextureJob::Upload {
                rect: rect(),
                words: vec![17; 6],
            },
            TextureJob::Clear {
                rect: Rect { x: 1023, ..rect() },
            },
        ];
        assert!(g.apply_texture_jobs(&jobs).is_err());
        assert_eq!(g.pixels, before);
        assert_eq!(g.dma, 0);
        g.gp0(0xa0000000);
        let buf = g.buf;
        let len = g.len;
        assert!(g.apply_texture_jobs(&[]).is_err());
        assert_eq!(g.buf, buf);
        assert_eq!(g.len, len);
    }
}
