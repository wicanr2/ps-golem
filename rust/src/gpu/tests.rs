use super::*;
fn send(g: &mut Gpu, words: &[u32]) {
    for &w in words {
        g.gp0(w);
    }
}
fn xy(x: u32, y: u32) -> u32 {
    x | y << 16
}
fn open(g: &mut Gpu) {
    send(g, &[0xe3000000, 0xe4000000 | 1023 | 511 << 10]);
}

#[test]
fn reset_and_partial_packet() {
    let mut g = Gpu::new();
    assert_eq!(g.status(), 0x14802000);
    g.gp0(0x600000ff);
    g.gp0(xy(12, 12));
    assert_eq!(g.vram()[12 * 1024 + 12], 0);
    g.gp1(0x01000000);
    assert_eq!(g.status(), 0x14802000);
    open(&mut g);
    send(&mut g, &[0x600000ff, xy(12, 12), xy(2, 2)]);
    assert_eq!(g.vram()[12 * 1024 + 12], 31);
}

#[test]
fn odd_upload_readback_wrap_and_mask() {
    let mut g = Gpu::new();
    open(&mut g);
    send(
        &mut g,
        &[0xa0000000, xy(1023, 511), xy(3, 1), 0x1234001f, 0xdead03e0],
    );
    assert_eq!(g.texel(1023, 511), 31);
    assert_eq!(g.texel(0, 511), 0x1234);
    assert_eq!(g.texel(1, 511), 0x03e0);
    send(&mut g, &[0xc0000000, xy(1023, 511), xy(3, 1)]);
    assert_eq!(g.read_data(), 0x1234001f);
    assert_eq!(g.read_data() & 65535, 0x03e0);
    send(&mut g, &[0xe6000001, 0xa0000000, xy(10, 10), xy(1, 1), 31]);
    send(
        &mut g,
        &[0xe6000002, 0xa0000000, xy(10, 10), xy(1, 1), 0x03e0],
    );
    assert_eq!(g.texel(10, 10), 0x801f);
}

#[test]
fn overlapping_copy_retains_128_word_chunks() {
    let mut g = Gpu::new();
    send(&mut g, &[0xa0000000, 0, xy(130, 1)]);
    for n in (1u32..=130).step_by(2) {
        g.gp0(n | (n + 1) << 16);
    }
    send(&mut g, &[0x80000000, 0, 1, xy(130, 1)]);
    assert_eq!(g.texel(1, 0), 1);
    assert_eq!(g.texel(128, 0), 128);
    assert_eq!(g.texel(129, 0), 128);
    assert_eq!(g.texel(130, 0), 130);
}

fn reference_blend(mode: i32, b: u16, f: u16) -> u16 {
    let mut v = 0x8000;
    for c in 0..3 {
        let x = (b >> (5 * c)) & 31;
        let y = (f >> (5 * c)) & 31;
        let z = match mode {
            0 => (x + y) / 2,
            1 => (x + y).min(31),
            2 => x.saturating_sub(y),
            _ => (x + y / 4).min(31),
        };
        v |= z << (5 * c);
    }
    v
}

#[test]
fn blend_components_and_saturation() {
    for mode in 0..4 {
        for b in 0u16..32 {
            for f in 0u16..32 {
                let bg = b | (31 - b) << 5 | ((b * 7) & 31) << 10;
                let fg = 0x8000 | f | ((f * 11) & 31) << 5 | (31 - f) << 10;
                assert_eq!(
                    raster::blend(mode, bg, fg),
                    reference_blend(mode, bg, fg),
                    "{mode}/{b}/{f}"
                );
            }
        }
    }
}

#[test]
fn clut_cache_invalidation() {
    let mut g = Gpu::new();
    open(&mut g);
    send(&mut g, &[0xa0000000, xy(0, 10), xy(16, 1)]);
    for _ in 0..8 {
        g.gp0(0x001f001f);
    }
    send(&mut g, &[0xa0000000, xy(64, 256), xy(4, 16)]);
    for _ in 0..32 {
        g.gp0(0x11111111);
    }
    let uv = (10 * 64) << 16;
    send(&mut g, &[0xe1000011, 0x7d000000, xy(0, 0), uv]);
    assert_eq!(g.texel(0, 0), 31);
    send(&mut g, &[0xa0000000, xy(0, 10), xy(16, 1)]);
    for _ in 0..8 {
        g.gp0(0x03e003e0);
    }
    send(&mut g, &[0x7d000000, xy(16, 0), uv]);
    assert_eq!(g.texel(16, 0), 31);
    send(&mut g, &[0x01000000, 0x7d000000, xy(32, 0), uv]);
    assert_eq!(g.texel(32, 0), 0x03e0);
}

#[test]
fn polygon_uses_own_texture_page_blend() {
    let mut cases = 0;
    for previous in 0..4 {
        for current in 0..4 {
            for tex in [0xa108u16, 0x2108, 0x8000, 0] {
                for cmd in [0x25u32, 0x27, 0x2d, 0x2f, 0x35, 0x37, 0x3d, 0x3f] {
                    let mut g = Gpu::new();
                    open(&mut g);
                    send(
                        &mut g,
                        &[
                            0x60606060,
                            0,
                            xy(32, 32),
                            0xa0000000,
                            xy(640, 256),
                            xy(8, 8),
                        ],
                    );
                    for _ in 0..32 {
                        g.gp0(tex as u32 | (tex as u32) << 16);
                    }
                    g.gp0(0xe1000000 | 0x11a | previous << 5);
                    let nv = if cmd & 8 != 0 { 4 } else { 3 };
                    let mut words = Vec::new();
                    for i in 0..nv {
                        if i == 0 || cmd & 16 != 0 {
                            words.push(0x808080 | if i == 0 { cmd << 24 } else { 0 });
                        }
                        let x = 8 + (i & 1) * 8;
                        let y = 8 + ((i & 2) >> 1) * 8;
                        words.push(xy(x, y));
                        words.push(
                            (x - 8)
                                | (y - 8) << 8
                                | if i == 1 {
                                    (0x11a | current << 5) << 16
                                } else {
                                    0
                                },
                        );
                    }
                    send(&mut g, &words);
                    let expected = if tex == 0 {
                        0x318c
                    } else if cmd & 2 != 0 && tex & 0x8000 != 0 {
                        reference_blend(current as i32, 0x318c, tex)
                    } else {
                        tex
                    };
                    assert_eq!(
                        g.texel(10, 10),
                        expected,
                        "{previous}/{current}/{cmd:x}/{tex:x}"
                    );
                    cases += 1;
                }
            }
        }
    }
    assert_eq!(cases, 512);
}
