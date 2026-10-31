//! Synthetic painter tests, never substitutes for original-game evidence.
use super::*;
use crate::gpu::{
    transfer::{Rect, TextureJob},
    Gpu,
};
fn material() -> Material {
    let mut palette = [0; 16];
    palette[1] = 31;
    palette[2] = 0x83e0;
    palette[3] = 0xfc00;
    palette[4] = 0x8000;
    Material {
        key: crate::scene::MaterialKey {
            upload: [0; 32],
            clut: [0; 32],
        },
        width: 8,
        height: 8,
        packed: (0..32)
            .map(|i| if i % 3 == 0 { 0x21 } else { 0x43 })
            .collect(),
        palette,
    }
}
#[test]
fn direct_native_matches_independent_command_entry() {
    let mat = material();
    for mirror in [false, true] {
        for raw in [false, true] {
            for mode in -1..4 {
                for skew in [false, true] {
                    let points = std::array::from_fn(|i| Point {
                        x: if skew {
                            [3, 15, 6, 18][i]
                        } else {
                            [3, 11, 3, 11][i]
                        },
                        y: if skew {
                            [3, 5, 15, 17][i]
                        } else {
                            [3, 3, 11, 11][i]
                        },
                        u: if mirror {
                            [7, -1, 7, -1][i]
                        } else {
                            [0, 8, 0, 8][i]
                        },
                        v: [0, 0, 8, 8][i],
                        color: [
                            [128, 64, 240],
                            [192, 128, 32],
                            [64, 240, 128],
                            [255, 96, 192],
                        ][i],
                    });
                    let mut canvas = Canvas {
                        width: 32,
                        height: 32,
                        words: vec![0x1234; 1024],
                    };
                    draw(
                        &mut canvas,
                        1,
                        std::slice::from_ref(&mat),
                        &[None],
                        &[Draw {
                            material: 0,
                            points,
                            raw,
                            blend: mode,
                        }],
                    )
                    .unwrap();
                    let mut g = Gpu::new();
                    let tex: Vec<_> = mat
                        .packed
                        .chunks_exact(2)
                        .map(|b| u16::from_le_bytes([b[0], b[1]]))
                        .collect();
                    g.apply_texture_jobs(&[
                        TextureJob::Upload {
                            rect: Rect {
                                x: 513,
                                y: 257,
                                width: 2,
                                height: 8,
                            },
                            words: tex,
                        },
                        TextureJob::Upload {
                            rect: Rect {
                                x: 0,
                                y: 240,
                                width: 16,
                                height: 1,
                            },
                            words: mat.palette.to_vec(),
                        },
                        TextureJob::Upload {
                            rect: Rect {
                                x: 0,
                                y: 0,
                                width: 32,
                                height: 32,
                            },
                            words: vec![0x1234; 1024],
                        },
                    ])
                    .unwrap();
                    for w in [0xe3000000, 0xe4007c1f, 0xe5000000, 0xe6000000, 0xe2000000] {
                        g.gp0(w);
                    }
                    for (i, p) in points.iter().enumerate() {
                        let c = p.color;
                        let high = if i == 0 {
                            ((0x3c | raw as u32 | if mode >= 0 { 2 } else { 0 }) as u32) << 24
                        } else {
                            0
                        };
                        g.gp0(high | c[0] as u32 | (c[1] as u32) << 8 | (c[2] as u32) << 16);
                        g.gp0(p.x as u16 as u32 | (p.y as u16 as u32) << 16);
                        let upper = if i == 0 {
                            15360
                        } else if i == 1 {
                            24 | (mode.max(0) as u32) << 5
                        } else {
                            0
                        };
                        g.gp0((p.u + 4) as u8 as u32 | ((p.v + 1) as u8 as u32) << 8 | upper << 16);
                    }
                    let expected: Vec<_> = (0..32)
                        .flat_map(|y| g.vram()[y * 1024..y * 1024 + 32].iter().copied())
                        .collect();
                    assert_eq!(
                        canvas.words, expected,
                        "mirror={mirror},raw={raw},mode={mode},skew={skew}"
                    );
                }
            }
        }
    }
}
#[test]
fn hd_uses_subtexels_and_rejects_before_mutating() {
    let mat = material();
    let pixels: Vec<_> = (0..1024).map(|i| 0x8000 | (i % 31 + 1) as u16).collect();
    let image = Image {
        width: 32,
        height: 32,
        scale: 4,
        pixels: ImagePixels::Colors(pixels.clone()),
    };
    let points = std::array::from_fn(|i| Point {
        x: [2, 10, 2, 10][i],
        y: [2, 2, 10, 10][i],
        u: [0, 8, 0, 8][i],
        v: [0, 0, 8, 8][i],
        color: [128; 3],
    });
    let d = Draw {
        material: 0,
        points,
        raw: true,
        blend: -1,
    };
    let mut c = Canvas {
        width: 64,
        height: 64,
        words: vec![0; 4096],
    };
    draw(
        &mut c,
        4,
        std::slice::from_ref(&mat),
        &[Some(image.clone())],
        &[d],
    )
    .unwrap();
    assert_eq!(c.words[8 * 64 + 8], pixels[0]);
    assert_eq!(c.words[8 * 64 + 9], pixels[1]);
    assert_eq!(c.words[9 * 64 + 8], pixels[32]);
    let before = c.words.clone();
    let invalid = Draw {
        material: 1,
        points,
        raw: true,
        blend: -1,
    };
    assert!(draw(
        &mut c,
        4,
        std::slice::from_ref(&mat),
        &[Some(image.clone())],
        &[invalid]
    )
    .is_err());
    assert_eq!(c.words, before);
    let bad = Image {
        scale: usize::MAX,
        ..image
    };
    assert!(draw(&mut c, 4, std::slice::from_ref(&mat), &[Some(bad)], &[]).is_err());
    assert_eq!(c.words, before);
}
