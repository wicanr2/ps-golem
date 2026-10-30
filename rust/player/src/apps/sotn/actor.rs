//! Typed SotN animation data, spec026 §16.6, GPL-3.0-or-later.
//! Own Go model: material tools/sotn_actor_plan_research.go, 6c3911f.
//! Original KSEG0 sub_800EB998 is evidence only. No CPU or GP0 execution.
use psgolem_core::gpu::transfer::{Rect, TextureJob};
use sha2::{Digest, Sha256};
type Result<T> = std::result::Result<T, String>;
struct Ram(Vec<u8>);
impl Ram {
    fn at(&self, a: u32, n: usize) -> Result<usize> {
        let p = (a & 0x1fff_ffff) as usize;
        if p >= self.0.len() || n > self.0.len() - p {
            return Err(format!("物件資料超界 {a:08x}/{n}"));
        }
        Ok(p)
    }
    fn byte(&self, a: u32) -> Result<u8> {
        Ok(self.0[self.at(a, 1)?])
    }
    fn half(&self, a: u32) -> Result<u16> {
        let i = self.at(a, 2)?;
        Ok(u16::from_le_bytes(self.0[i..i + 2].try_into().unwrap()))
    }
    fn word(&self, a: u32) -> Result<u32> {
        let i = self.at(a, 4)?;
        Ok(u32::from_le_bytes(self.0[i..i + 4].try_into().unwrap()))
    }
    fn pointer(&self, a: u32) -> Result<u32> {
        let v = self.word(a)?;
        let i = self.at(v, 1)?;
        Ok(0x80000000 | i as u32)
    }
    fn put(&mut self, a: u32, v: u32) -> Result<()> {
        let i = self.at(a, 4)?;
        self.0[i..i + 4].copy_from_slice(&v.to_le_bytes());
        Ok(())
    }
    fn put_half(&mut self, a: u32, v: u16) -> Result<()> {
        let i = self.at(a, 2)?;
        self.0[i..i + 2].copy_from_slice(&v.to_le_bytes());
        Ok(())
    }
}
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
pub struct Vertex {
    pub x: i16,
    pub y: i16,
    pub u: u8,
    pub v: u8,
    pub color: [u8; 3],
}
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Quad {
    pub entity: usize,
    pub part: usize,
    pub order: usize,
    pub address: u32,
    pub page: u16,
    pub clut: u16,
    pub command: u8,
    pub vertices: [Vertex; 4],
    pub words: [u32; 12],
}
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct ImageJob {
    pub entity: usize,
    pub source: u32,
    pub job: TextureJob,
}
pub struct ActorPlan {
    pub ram: Vec<u8>,
    pub quads: Vec<Quad>,
    pub images: Vec<ImageJob>,
    pub active: usize,
    pub hidden: usize,
    pub count_before: u32,
    pub count_after: u32,
}
fn half(b: &[u8], i: usize) -> u16 {
    u16::from_le_bytes(b[i..i + 2].try_into().unwrap())
}
fn put_half(b: &mut [u8], i: usize, v: u16) {
    b[i..i + 2].copy_from_slice(&v.to_le_bytes());
}
fn add(a: u16, b: u16) -> u16 {
    a.wrapping_add(b)
}
fn sub(a: u16, b: u16) -> u16 {
    a.wrapping_sub(b)
}
impl ActorPlan {
    pub fn prepare(input: &[u8], scratch: &[u8]) -> Result<Self> {
        if input.len() != 2 << 20
            || scratch.len() != 1024
            || format!("{:x}", Sha256::digest(&input[0xeb998..0xec9b0]))
                != "5345790c96491fc6f21610d3ff975bc7311c46515f660dea3e0d1a96c66c39d7"
        {
            return Err("物件來源大小或原routine身份不符".into());
        }
        let mut r = Ram(input.to_vec());
        let buffer = r.word(0x8006c37c)?;
        if ![0x8003cb08, 0x800542fc].contains(&buffer) {
            return Err("未知物件緩衝".into());
        }
        let mut count = r.word(0x80097930)?;
        if count > 768 {
            return Err("物件圖元數超界".into());
        }
        let before = count;
        let ot = buffer + 0x474;
        r.at(ot, 512 * 4)?;
        r.at(buffer + 0x3c74, 768 * 52)?;
        let mut plan = Self {
            ram: Vec::new(),
            quads: Vec::new(),
            images: Vec::new(),
            active: 0,
            hidden: 0,
            count_before: before,
            count_after: before,
        };
        let mut packet = scratch[..52].to_vec();
        packet[3] = 12;
        packet[7] = 0x3c;
        let xy = [8, 20, 32, 44];
        let uv = [12, 24, 36, 48];
        let rgb = [4, 16, 28, 40];
        for slot in 0..256 {
            let a = 0x800733d8 + slot as u32 * 188;
            let bank = r.half(a + 0x54)?;
            let frame = r.half(a + 0x56)?;
            if bank == 0 || frame == 0 {
                continue;
            }
            plan.active += 1;
            let flags = r.byte(a + 0x19)?;
            if flags & 0x80 != 0 && (slot as u32 ^ r.word(0x8003c998)?) & 1 != 0 {
                plan.hidden += 1;
                continue;
            }
            let x = add(r.half(a + 2)?, r.half(0x8006c39c)?);
            let y = add(r.half(a + 6)?, r.half(0x8006c3a0)?);
            if add(x, 512) > 1024 || add(y, 512) > 1024 || frame & 0x7fff == 0 {
                plan.hidden += 1;
                continue;
            }
            if flags & 4 != 0 || bank == 15 {
                return Err("旋轉或bank15複製尚未證實".into());
            }
            let table = if bank & 0x8000 == 0 {
                r.pointer(0x800a3b80 + bank as u32 * 4)?
            } else {
                let t = r.pointer(0x8003c788)?;
                r.at(t, (bank as usize & 0x7fff) * 4 + 4)?;
                r.pointer(t + (bank as u32 & 0x7fff) * 4)?
            };
            r.at(table, (frame as usize & 0x7fff) * 4 + 4)?;
            let frame_data = r.pointer(table + (frame as u32 & 0x7fff) * 4)?;
            let n = r.half(frame_data)?;
            let packed = n & 0x8000 != 0;
            let parts = if packed { 1 } else { n as usize };
            if parts > 768 {
                return Err("動畫部件數超界".into());
            }
            r.at(frame_data, if packed { 8 } else { 2 + parts * 22 })?;
            for part in 0..parts {
                if count >= 768 {
                    break;
                }
                let (
                    mut dx,
                    mut dy,
                    mut w,
                    mut h,
                    page,
                    part_palette,
                    mut u0,
                    mut v0,
                    mut u1,
                    mut v1,
                    part_flags,
                );
                let mut flip = (r.half(a + 0x14)? as u32) << 1;
                let mut colors = None;
                if packed {
                    let cache = 0x80097d1c + r.half(a + 0x5a)? as u32 * 40;
                    r.at(cache, 40)?;
                    flip ^= r.byte(cache + 0x23)? as u32;
                    let id = (n & 0x7fff) as u32;
                    let sources = match bank {
                        13 => r.pointer(0x8013c004)?,
                        0x8001 => r.pointer(0x8003c7a0)?,
                        b if b & 0x8000 != 0 && b != 0x8010 => r.pointer(0x8003c7a4)?,
                        _ => 0x8013c020,
                    };
                    r.at(sources, (id as usize + 1) * 4)?;
                    let source = r.pointer(sources + id * 4)?;
                    r.at(source, 4)?;
                    w = r.byte(source)? as u16;
                    h = r.byte(source + 1)? as u16;
                    dx = add(r.half(frame_data + 2)?, r.byte(source + 2)? as u16);
                    dy = add(r.half(frame_data + 4)?, r.byte(source + 3)? as u16);
                    part_palette = r.half(frame_data + 6)?;
                    page = r.byte(cache + 0x1f)? as u16;
                    u0 = (r.half(cache)? & 0x3f) as i32 * 4;
                    v0 = r.byte(cache + 2)? as i32;
                    u1 = u0 + w as i32;
                    v1 = v0 + h as i32;
                    part_flags = 0;
                    if frame & 0x8000 == 0 {
                        if w == 0 || w % 4 != 0 || h == 0 {
                            return Err("packed圖像形狀未支援".into());
                        }
                        r.put_half(cache + 4, w / 4)?;
                        r.put_half(cache + 6, h)?;
                        let rect = Rect {
                            x: r.half(cache)?,
                            y: r.half(cache + 2)?,
                            width: w / 4,
                            height: h,
                        };
                        let offset = r.at(source + 4, w as usize / 4 * h as usize * 2)?;
                        let words = r.0[offset..offset + w as usize / 4 * h as usize * 2]
                            .chunks_exact(2)
                            .map(|v| u16::from_le_bytes([v[0], v[1]]))
                            .collect();
                        plan.images.push(ImageJob {
                            entity: slot,
                            source: source + 4,
                            job: TextureJob::Upload { rect, words },
                        });
                        plan.images.push(ImageJob {
                            entity: slot,
                            source: 0,
                            job: TextureJob::Clear {
                                rect: Rect {
                                    x: add(rect.x, rect.width),
                                    y: sub(rect.y, 1),
                                    width: 1,
                                    height: add(rect.height, 4),
                                },
                            },
                        });
                        plan.images.push(ImageJob {
                            entity: slot,
                            source: 0,
                            job: TextureJob::Clear {
                                rect: Rect {
                                    x: sub(rect.x, 1),
                                    y: add(rect.y, rect.height),
                                    width: add(rect.width, 2),
                                    height: 4,
                                },
                            },
                        });
                    }
                    if r.byte(cache + 0x1b)? != 0 {
                        let mut c = [[0; 3]; 4];
                        for (i, v) in c.iter_mut().enumerate() {
                            let p = cache + 0x18 + i as u32 * 4;
                            *v = [r.byte(p)?, r.byte(p + 2)?, r.byte(p + 1)?];
                        }
                        colors = Some(c);
                    }
                } else {
                    let p = frame_data + 2 + part as u32 * 22;
                    part_flags = r.half(p)?;
                    dx = r.half(p + 2)?;
                    dy = r.half(p + 4)?;
                    w = r.half(p + 6)?;
                    h = r.half(p + 8)?;
                    let mut ppage = add(r.half(p + 12)?, r.half(a + 0x5a)?);
                    part_palette = r.half(p + 10)?;
                    u0 = r.byte(p + 14)? as i32;
                    v0 = r.byte(p + 16)? as i32;
                    u1 = r.byte(p + 18)? as i32;
                    v1 = r.byte(p + 20)? as i32;
                    if ppage & 1 != 0 {
                        u0 -= 128;
                        u1 -= 128;
                    }
                    if ppage & 2 != 0 {
                        v0 -= 128;
                        v1 -= 128;
                    }
                    ppage >>= 2;
                    page = ppage;
                    if part_flags & 4 != 0 {
                        w = sub(w, 1);
                        if part_flags & 2 != 0 {
                            dx = add(dx, 1);
                        }
                        u1 -= 1;
                    }
                    if part_flags & 8 != 0 {
                        h = sub(h, 1);
                        if part_flags & 1 != 0 {
                            dy = add(dy, 1);
                        }
                        v1 -= 1;
                    }
                    if part_flags & 16 != 0 {
                        w = sub(w, 1);
                        if part_flags & 2 == 0 {
                            dx = add(dx, 1);
                        }
                        u0 += 1;
                    }
                    if part_flags & 32 != 0 {
                        h = sub(h, 1);
                        if part_flags & 1 == 0 {
                            dy = add(dy, 1);
                        }
                        v0 += 1;
                    }
                }
                let left = if if packed {
                    flip & 2 != 0
                } else {
                    r.half(a + 0x14)? != 0
                } {
                    sub(x, dx)
                } else {
                    add(x, dx)
                };
                let top = add(y, dy);
                let (l, right) = if flip != 0 {
                    (add(sub(left, w), 1), add(left, 1))
                } else {
                    (left, add(left, w))
                };
                for (i, &p) in xy.iter().enumerate() {
                    put_half(&mut packet, p, if i & 1 != 0 { right } else { l });
                    put_half(&mut packet, p + 2, if i >= 2 { add(top, h) } else { top });
                }
                let px = if flip != 0 {
                    sub(x, r.half(a + 0x20)?)
                } else {
                    add(x, r.half(a + 0x20)?)
                };
                let py = add(y, r.half(a + 0x22)?);
                for &p in &xy {
                    if flags & 1 != 0 {
                        let delta = sub(half(&packet, p), px) as i16 as i32;
                        put_half(
                            &mut packet,
                            p,
                            (px as i32 + ((delta * r.half(a + 0x1a)? as i16 as i32) >> 8)) as u16,
                        );
                    }
                    if flags & 2 != 0 {
                        let delta = sub(half(&packet, p + 2), py) as i16 as i32;
                        put_half(
                            &mut packet,
                            p + 2,
                            (py as i32 + ((delta * r.half(a + 0x1c)? as i16 as i32) >> 8)) as u16,
                        );
                    }
                }
                let (x0, x1) = if (part_flags as u32 & 2) ^ flip != 0 {
                    (u1 - 1, u0 - 1)
                } else {
                    (u0, u1)
                };
                let (y0, y1) = if !packed && part_flags & 1 != 0 {
                    (v1 - 1, v0 - 1)
                } else {
                    (v0, v1)
                };
                for (i, &p) in uv.iter().enumerate() {
                    packet[p] = if i & 1 != 0 { x1 } else { x0 } as u8;
                    packet[p + 1] = if i >= 2 { y1 } else { y0 } as u8;
                }
                let mut palette = r.half(a + 0x16)?;
                palette = if palette & 0x8000 == 0 {
                    add(palette, part_palette)
                } else {
                    palette & 0x7fff
                };
                put_half(&mut packet, 14, r.half(0x8003c104 + palette as u32 * 2)?);
                let blend = r.byte(a + 0x18)?;
                let page = if blend != 0 {
                    packet[7] |= 2;
                    add(page, (blend & 0x60) as u16)
                } else {
                    packet[7] &= 0xfd;
                    page
                };
                put_half(&mut packet, 26, page);
                if let Some(c) = colors {
                    for (p, color) in rgb.iter().zip(c) {
                        packet[*p..*p + 3].copy_from_slice(&color);
                    }
                    packet[7] &= 0xfe;
                } else if flags & 8 != 0 {
                    for &p in &rgb {
                        for c in 0..3 {
                            packet[p + c] = if flags & (16 << c) != 0 {
                                128
                            } else {
                                r.byte(a + 0x6c)?
                            };
                        }
                    }
                    packet[7] &= 0xfe;
                } else {
                    packet[7] |= 1;
                }
                let mut order = r.half(a + 0x24)? as usize;
                if part_flags & 0x100 != 0 {
                    order += 4;
                }
                if order >= 512 {
                    return Err("物件排序超界".into());
                }
                let address = buffer + 0x3c74 + count * 52;
                let bucket = ot + order as u32 * 4;
                packet[..4]
                    .copy_from_slice(&(0x0c000000 | r.word(bucket)? & 0xffffff).to_le_bytes());
                let at = r.at(address, 52)?;
                r.0[at..at + 52].copy_from_slice(&packet);
                r.put(bucket, r.word(bucket)? & 0xff000000 | address & 0xffffff)?;
                let vertices = std::array::from_fn(|i| Vertex {
                    x: half(&packet, xy[i]) as i16,
                    y: half(&packet, xy[i] + 2) as i16,
                    u: packet[uv[i]],
                    v: packet[uv[i] + 1],
                    color: packet[rgb[i]..rgb[i] + 3].try_into().unwrap(),
                });
                let words = std::array::from_fn(|i| {
                    u32::from_le_bytes(packet[4 + i * 4..8 + i * 4].try_into().unwrap())
                });
                plan.quads.push(Quad {
                    entity: slot,
                    part,
                    order,
                    address,
                    page,
                    clut: half(&packet, 14),
                    command: packet[7],
                    vertices,
                    words,
                });
                count += 1;
            }
        }
        r.put(0x80097930, count)?;
        plan.count_after = count;
        plan.ram = r.0;
        Ok(plan)
    }
}
