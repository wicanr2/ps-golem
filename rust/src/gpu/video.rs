//! Native scanout, 026 §7 / 007–008. Port of ps-golem GPU at 343afa4.
//! Original reference: beetle-psx-libretro gpu.c e57860d8, GPL-2.0-or-later.
use super::Gpu;
#[derive(Clone, Debug)]
pub(super) struct VideoTiming {
    pub(super) clock: u32,
    pub(super) line: u32,
    pub(super) lines: u32,
    pub(super) field: bool,
    pub(super) blank: bool,
    pub(super) y: u32,
    pub(super) read_line: u32,
}
impl Default for VideoTiming {
    fn default() -> Self {
        Self {
            clock: 0,
            line: 0,
            lines: 263,
            field: false,
            blank: true,
            y: 0,
            read_line: 0,
        }
    }
}
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Frame {
    pub width: u32,
    pub height: u32,
    pub rgba: Vec<u8>,
}
impl Gpu {
    pub fn irq(&self) -> bool {
        self.irq
    }
    pub fn dot_divider(&self) -> u32 {
        if self.display_mode & 0x40 != 0 {
            7
        } else {
            [10, 8, 5, 4][(self.display_mode & 3) as usize]
        }
    }
    pub fn video_to_next_line(&self) -> u32 {
        3413 - self.video.clock
    }
    pub fn video_state(&self) -> [u32; 8] {
        let v = &self.video;
        [
            v.clock,
            v.line,
            v.lines,
            v.field as u32,
            v.blank as u32,
            v.y,
            v.read_line,
            self.field_readout as u32,
        ]
    }
    pub fn tick_video(&mut self, n: u32) -> (u32, u32) {
        self.video.clock = self.video.clock.wrapping_add(n);
        let (mut blank, mut lines) = (0, 0);
        while self.video.clock >= 3413 {
            self.video.clock -= 3413;
            lines += 1;
            if self.end_line() {
                blank += 1;
            }
        }
        (blank, lines)
    }
    fn end_line(&mut self) -> bool {
        let v = &mut self.video;
        if !v.blank {
            v.y += 1;
        }
        v.line += 1;
        if v.line >= v.lines {
            v.line = 0;
        }
        if v.line == v.lines - 1 {
            v.field = if self.display_mode & 0x20 != 0 {
                !v.field
            } else {
                false
            };
        }
        if v.line == 0 {
            if self.display_mode & 0x20 != 0 {
                v.lines = if self.display_mode & 8 != 0 { 313 } else { 263 } - v.field as u32;
            } else {
                v.field = false;
                v.lines = if self.display_mode & 8 != 0 { 314 } else { 263 };
            }
        }
        let mut started = false;
        if v.line == self.vert_end && !v.blank {
            v.blank = true;
            started = true;
            v.y = 0;
            self.field_readout = if self.display_mode & 0x24 == 0x24 {
                !v.field
            } else {
                false
            };
        }
        if v.line == self.vert_start && v.blank {
            v.blank = false;
        }
        let mut off = v.y;
        if self.display_mode & 0x24 == 0x24 {
            off = v.y << 1;
            if !v.blank && self.field_readout {
                off += 1;
            }
        }
        v.read_line = (self.display_y + off) & 511;
        started
    }
    pub fn screen(&self) -> Frame {
        let hres = if self.display_mode & 0x40 != 0 {
            368
        } else {
            [256, 320, 512, 640][(self.display_mode & 3) as usize]
        };
        let mut width = hres;
        if self.horiz_end > self.horiz_start {
            let cw = ((self.horiz_end - self.horiz_start) / self.dot_divider() + 2) & !3;
            if cw > 0 && cw <= hres {
                width = cw;
            }
        }
        let mut height = if self.vert_end > self.vert_start {
            self.vert_end - self.vert_start
        } else {
            240
        };
        if self.display_mode & 0x24 == 0x24 {
            height *= 2;
        }
        let mut rgba = vec![0; (width * height * 4) as usize];
        for y in 0..height {
            let row = ((self.display_y + y) & 511) * 1024;
            for x in 0..width {
                let o = ((y * width + x) * 4) as usize;
                if self.display_mode & 0x10 != 0 {
                    let b = self.display_x * 2 + x * 3;
                    for i in 0..3 {
                        let p = b + i;
                        rgba[o + i as usize] =
                            (self.pixels[(row + ((p / 2) & 1023)) as usize] >> (8 * (p & 1))) as u8;
                    }
                } else {
                    let pixel = self.pixels[(row + ((self.display_x + x) & 1023)) as usize];
                    for i in 0..3 {
                        let v = ((pixel >> (5 * i)) & 31) as u8;
                        rgba[o + i] = (v << 3) | (v >> 2);
                    }
                }
                rgba[o + 3] = 255;
            }
        }
        Frame {
            width,
            height,
            rgba,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    fn hash(data: &[u8]) -> u64 {
        data.iter().fold(14695981039346656037, |h, &b| {
            (h ^ b as u64).wrapping_mul(1099511628211)
        })
    }
    #[test]
    fn fixed_go_video_and_scanout() {
        let Ok(path) = std::env::var("PSGOLEM_VIDEO_ORACLE") else {
            eprintln!("SKIP: select fixed Go GPU video oracle");
            return;
        };
        let bytes = std::fs::read(path).unwrap();
        assert_eq!(&bytes[..8], b"PSGVID01");
        let mut p = 8;
        fn word(bytes: &[u8], p: &mut usize) -> u32 {
            let v = u32::from_le_bytes(bytes[*p..*p + 4].try_into().unwrap());
            *p += 4;
            v
        }
        let count = word(&bytes, &mut p);
        let mut gpu = Gpu::new();
        gpu.gp0(0xa0000000);
        gpu.gp0(0);
        gpu.gp0(512 << 16 | 1024);
        let mut seed = 0xabc123u32;
        for _ in 0..262144 {
            seed = seed.wrapping_mul(1664525).wrapping_add(1013904223);
            gpu.gp0(seed);
        }
        for event in 0..count {
            let kind = word(&bytes, &mut p);
            let arg = word(&bytes, &mut p);
            let result = match kind {
                0 => {
                    gpu.gp1(arg);
                    (0, 0)
                }
                1 => gpu.tick_video(arg),
                2 => {
                    gpu.gp0(arg);
                    (0, 0)
                }
                _ => panic!("event"),
            };
            let mut outputs = vec![
                result.0,
                result.1,
                gpu.status(),
                gpu.dot_divider(),
                gpu.video_to_next_line(),
            ];
            outputs.extend(gpu.video_state());
            for (field, value) in outputs.iter().enumerate() {
                assert_eq!(
                    *value,
                    word(&bytes, &mut p),
                    "video event {event}, field {field}"
                );
            }
            if event % 16 == 15 {
                let frame = gpu.screen();
                assert_eq!(frame.width, word(&bytes, &mut p));
                assert_eq!(frame.height, word(&bytes, &mut p));
                let expected = word(&bytes, &mut p) as u64 | ((word(&bytes, &mut p) as u64) << 32);
                assert_eq!(hash(&frame.rgba), expected, "screen event {event}");
            }
        }
        assert_eq!(p, bytes.len());
        assert_eq!(count, 1024);
        eprintln!("VIDEO_GO_ORACLE: 1024 events, 64 complete scanout hashes, all timing/status fields exact");
    }
}
