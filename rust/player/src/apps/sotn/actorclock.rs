//! Ordinary actor resource clock, 026 §16.7.2, GPL-3.0-or-later.
//! Own 008/026 CPU/cache model at 4e2fad8. Original PCs label resource
//! footprints only: no opcode fetch, guest execution, or saved-trace replay.
use super::actorstate::{ActorDataPlan, Part};
use psgolem_core::{cpu::State, gpu::transfer::TextureContext};
use sha2::{Digest, Sha256};
type Result<T> = std::result::Result<T, String>;
fn h(r: &[u8], a: u32) -> u16 {
    let i = (a & 0x1fffff) as usize;
    u16::from_le_bytes(r[i..i + 2].try_into().unwrap())
}
fn byte(r: &[u8], a: u32) -> u8 {
    r[(a & 0x1fffff) as usize]
}
// READ_SITES and SDK_IDENTITIES are inserted from the reviewed evidence
// manifest before compilation. They contain resource metadata, not opcodes.
const READ_SITES: &[(u32, u32)] = &[
    (0x1298c, 4),
    (0x12a7c, 4),
    (0x12a80, 4),
    (0x12ae8, 4),
    (0x12af0, 4),
    (0x12af4, 4),
    (0x12b04, 4),
    (0x12b08, 4),
    (0x12b0c, 4),
    (0x12b10, 4),
    (0x12b14, 4),
    (0x12b54, 4),
    (0x12b5c, 4),
    (0x12b60, 4),
    (0x12b70, 4),
    (0x12b74, 4),
    (0x12b78, 4),
    (0x13d90, 4),
    (0x13d9c, 4),
    (0x13da8, 4),
    (0x13db8, 4),
    (0x13de8, 4),
    (0x13dfc, 4),
    (0x13e44, 4),
    (0x13e54, 4),
    (0x13e68, 4),
    (0x13ebc, 4),
    (0x13ed8, 4),
    (0x13fb4, 4),
    (0x13fb8, 4),
    (0x13fbc, 4),
    (0x13ff4, 4),
    (0x14000, 4),
    (0x1400c, 4),
    (0x14018, 4),
    (0x14050, 4),
    (0x14064, 4),
    (0x140ac, 4),
    (0x140c0, 4),
    (0x14104, 4),
    (0x1410c, 2),
    (0x14158, 4),
    (0x14168, 4),
    (0x14178, 4),
    (0x14190, 4),
    (0x14194, 4),
    (0x141a4, 4),
    (0x141a8, 4),
    (0x141c4, 4),
    (0x141d0, 4),
    (0x141f0, 4),
    (0x14200, 4),
    (0x14214, 4),
    (0x14224, 4),
    (0x14234, 4),
    (0x14238, 4),
    (0x1423c, 4),
    (0x14240, 4),
    (0x14244, 4),
    (0x14248, 4),
    (0x145d8, 4),
    (0x145e8, 4),
    (0x145f8, 4),
    (0x14608, 4),
    (0x14624, 4),
    (0x14634, 4),
    (0x1463c, 2),
    (0x146ac, 4),
    (0x146d0, 4),
    (0x146d8, 4),
    (0x146ec, 4),
    (0x146f4, 2),
    (0x1470c, 4),
    (0x14730, 4),
    (0x14738, 2),
    (0x148b0, 4),
    (0x148b4, 4),
    (0x148b8, 4),
    (0x148bc, 4),
    (0x148c0, 4),
    (0x148c4, 4),
    (0x148c8, 4),
    (0x14f64, 4),
    (0x1530c, 4),
    (0x15314, 4),
    (0x15328, 2),
    (0x1532c, 2),
    (0x15334, 4),
    (0x1534c, 4),
    (0x15438, 4),
    (0x1543c, 4),
    (0x15440, 4),
    (0x1567c, 4),
    (0x15684, 2),
    (0xeb548, 4),
    (0xeb550, 4),
    (0xeb57c, 4),
    (0xeb588, 4),
    (0xeb594, 4),
    (0xeb5b0, 4),
    (0xeb5ec, 4),
    (0xeb620, 4),
    (0xeb65c, 4),
    (0xeb928, 4),
    (0xeb92c, 4),
    (0xeb930, 4),
    (0xeb934, 4),
    (0xeb938, 4),
    (0xeb93c, 4),
    (0xeb940, 4),
    (0xeb944, 4),
    (0xeb948, 4),
    (0xeb94c, 4),
    (0xeb9d0, 4),
    (0xeba2c, 4),
    (0xeba4c, 4),
    (0xeba58, 4),
    (0xeba74, 4),
    (0xeba94, 4),
    (0xebaac, 4),
    (0xebab4, 4),
    (0xebac0, 4),
    (0xebad0, 4),
    (0xebadc, 4),
    (0xebb40, 4),
    (0xebb4c, 4),
    (0xebb60, 4),
    (0xebb6c, 4),
    (0xebb78, 4),
    (0xebb80, 4),
    (0xebb94, 4),
    (0xebbb8, 4),
    (0xebbec, 4),
    (0xebc0c, 4),
    (0xebc28, 4),
    (0xebc38, 4),
    (0xebc44, 4),
    (0xebc4c, 4),
    (0xebc50, 4),
    (0xebc60, 4),
    (0xebc64, 4),
    (0xebc74, 4),
    (0xebc80, 4),
    (0xebc8c, 4),
    (0xebc98, 4),
    (0xebc9c, 4),
    (0xebd0c, 4),
    (0xebd10, 4),
    (0xebd1c, 4),
    (0xebd28, 4),
    (0xebd50, 4),
    (0xebd5c, 4),
    (0xebd60, 4),
    (0xebd6c, 4),
    (0xebd84, 4),
    (0xebd88, 4),
    (0xebd8c, 4),
    (0xebecc, 4),
    (0xebefc, 4),
    (0xebf00, 4),
    (0xebf04, 4),
    (0xebf08, 4),
    (0xebf18, 4),
    (0xebf30, 4),
    (0xebf3c, 4),
    (0xebf5c, 4),
    (0xebf84, 4),
    (0xebf94, 4),
    (0xebfa0, 4),
    (0xebfac, 4),
    (0xebfb8, 4),
    (0xebfc4, 4),
    (0xebfd0, 4),
    (0xebfdc, 4),
    (0xebfe8, 4),
    (0xebff4, 4),
    (0xec000, 4),
    (0xec00c, 4),
    (0xec01c, 4),
    (0xec048, 4),
    (0xec140, 4),
    (0xec154, 4),
    (0xec158, 4),
    (0xec16c, 4),
    (0xec17c, 4),
    (0xec1a0, 4),
    (0xec1d4, 4),
    (0xec1e8, 4),
    (0xec1ec, 4),
    (0xec1f4, 4),
    (0xec218, 4),
    (0xec224, 4),
    (0xec230, 4),
    (0xec23c, 4),
    (0xec328, 4),
    (0xec420, 4),
    (0xec450, 4),
    (0xec454, 4),
    (0xec458, 4),
    (0xec45c, 4),
    (0xec470, 4),
    (0xec488, 4),
    (0xec494, 4),
    (0xec498, 4),
    (0xec49c, 4),
    (0xec4a4, 4),
    (0xec5c0, 4),
    (0xec5e0, 4),
    (0xec620, 4),
    (0xec714, 4),
    (0xec73c, 4),
    (0xec740, 4),
    (0xec760, 4),
    (0xec788, 4),
    (0xec958, 4),
    (0xec97c, 4),
    (0xec980, 4),
    (0xec984, 4),
    (0xec988, 4),
    (0xec98c, 4),
    (0xec990, 4),
    (0xec994, 4),
    (0xec998, 4),
    (0xec99c, 4),
    (0xec9a0, 4),
];
const SDK_IDENTITIES: &[(usize, usize, &str)] = &[
    (
        0x145d0,
        76,
        "96a75f8c9ecd2647157165e1b5f1a002c0f55738d9bc2b67e6aba17e0e6f3bcc",
    ),
    (
        0x1461c,
        48,
        "cf877940e0440a6e5ff86ee02b35b58cee5215710a88232707183dd099388b2b",
    ),
    (
        0x15678,
        28,
        "8ec255b7358f8c65187341acc1e725f18ea7cb1bcba9b6a119e95d616dc159ba",
    ),
    (
        0x12b24,
        100,
        "3b7c31754b301c6149f2d5777db097d27bc67ee0d0526e094de6a1b486f611ef",
    ),
    (
        0x12a90,
        148,
        "675e331b54edd6d998a8f762ad3eb25905d09f06fc382e36d2e63f904a9e576c",
    ),
];
const MULT: [u64; 24] = [
    14, 14, 14, 14, 14, 14, 14, 14, 14, 14, 14, 14, 10, 10, 10, 10, 10, 10, 10, 10, 10, 7, 7, 7,
];
struct Clock {
    state: State,
    costs: Vec<u32>,
    width: u32,
    scale_x: u32,
    scale_y: u32,
}
impl Clock {
    fn run(&mut self, first: u32, last: u32) -> Result<()> {
        for pc in (first..=last).step_by(4) {
            if self.costs.len() >= 100000 {
                return Err("物件時間量子超額".into());
            }
            let word = (pc >> 2) & 3;
            let line = &mut self.state.icache[((pc >> 4) & 255) as usize];
            let mut cost = 1u64;
            if line.tag != pc >> 12 || line.valid & (1 << word) == 0 {
                let fill = ((15 << word) & 15) as u8;
                line.valid = if line.tag == pc >> 12 {
                    line.valid | fill
                } else {
                    fill
                };
                line.tag = pc >> 12;
                cost += (7 - word) as u64;
            }
            if let Ok(i) = READ_SITES.binary_search_by_key(&pc, |x| x.0) {
                cost += READ_SITES[i].1 as u64;
            }
            if [0x140d0, 0xeb5d4, 0xeb5f8, 0xeb644, 0xeb668].contains(&pc) {
                cost = cost.max(self.state.muldiv_done.saturating_sub(self.state.now));
            }
            if cost > 32 {
                return Err("物件單步等待超界".into());
            }
            self.state.now = self.state.now.checked_add(cost).ok_or("物件時鐘溢位")?;
            self.state.cost = cost;
            self.costs.push(cost as u32);
            let operand = match pc {
                0x140c8 => Some(self.width),
                0xeb5c0 | 0xeb5f4 => Some(self.scale_x),
                0xeb630 | 0xeb664 => Some(self.scale_y),
                _ => None,
            };
            if let Some(a) = operand {
                let key = a ^ ((a as i32 >> 31) as u32);
                self.state.muldiv_done = self
                    .state
                    .now
                    .checked_add(MULT[(key | 0x400).leading_zeros() as usize])
                    .ok_or("物件乘法期限溢位")?;
            }
        }
        Ok(())
    }
    fn ranges(&mut self, spans: &[(u32, u32)]) -> Result<()> {
        for &(a, b) in spans {
            self.run(a, b)?;
        }
        Ok(())
    }
    fn rect_check(&mut self) -> Result<()> {
        self.ranges(&[(0x12978, 0x129ac), (0x12a7c, 0x12a8c)])
    }
    fn submit_enter(&mut self) -> Result<()> {
        self.ranges(&[
            (0x14670, 0x146a4),
            (0x15678, 0x15690),
            (0x146a8, 0x146b8),
            (0x146cc, 0x14718),
            (0x1472c, 0x14748),
            (0x14764, 0x1476c),
        ])
    }
    fn submit_leave(&mut self) -> Result<()> {
        self.ranges(&[
            (0x14770, 0x14774),
            (0x15678, 0x15690),
            (0x14778, 0x1477c),
            (0x148b0, 0x148d4),
        ])
    }
    fn upload(&mut self, width: u16, height: u16) -> Result<()> {
        let words = (width as usize * height as usize).div_ceil(2);
        let blocks = words / 16;
        let remainder = words % 16;
        if blocks == 0 {
            return Err("時鐘尚未驗證CPU-only圖像上傳".into());
        }
        self.width = width as u32;
        self.run(0x12b24, 0x12b48)?;
        self.rect_check()?;
        self.run(0x12b4c, 0x12b6c)?;
        self.submit_enter()?;
        self.ranges(&[
            (0x13fcc, 0x13ff0),
            (0x14f40, 0x14f4c),
            (0x15308, 0x15354),
            (0x15438, 0x1544c),
            (0x14f50, 0x14f70),
            (0x13ff4, 0x1403c),
            (0x14050, 0x14070),
            (0x14084, 0x1408c),
            (0x140ac, 0x140b4),
            (0x140c0, 0x140e8),
            (0x140f4, 0x1411c),
            (0x14154, 0x14180),
            (0x14188, 0x141bc),
        ])?;
        if remainder > 0 {
            self.run(0x141c0, 0x141c0)?;
            for _ in 0..remainder {
                self.run(0x141c4, 0x141e0)?;
            }
        }
        self.run(0x141e4, 0x14254)?;
        self.submit_leave()?;
        self.run(0x12b70, 0x12b84)
    }
    fn clear(&mut self, x: u16) -> Result<()> {
        self.run(0x12a90, 0x12ac4)?;
        self.rect_check()?;
        self.run(0x12ac8, 0x12b00)?;
        self.submit_enter()?;
        self.ranges(&[
            (0x13d80, 0x13dd4),
            (0x13de8, 0x13e08),
            (0x13e1c, 0x13e24),
            (0x13e44, 0x13e4c),
            (0x13e54, 0x13e64),
        ])?;
        if x & 63 == 0 {
            self.run(0x13e68, 0x13e78)?;
        }
        self.ranges(&[
            (0x13e7c, 0x13f10),
            (0x1461c, 0x14648),
            (0x13f14, 0x13f24),
            (0x1461c, 0x14648),
            (0x13f28, 0x13f3c),
            (0x1461c, 0x14648),
            (0x13f40, 0x13f50),
            (0x13fa0, 0x13fac),
            (0x145d0, 0x14618),
            (0x13fb0, 0x13fc8),
        ])?;
        self.submit_leave()?;
        self.run(0x12b04, 0x12b20)
    }
    fn transform(&mut self, r: &[u8], slot: usize, flags: u8, flip: u32) -> Result<()> {
        let a = 0x800733d8 + slot as u32 * 188;
        self.scale_x = h(r, a + 0x1a) as i16 as i32 as u32;
        self.scale_y = h(r, a + 0x1c) as i16 as i32 as u32;
        self.run(0xeb544, 0xeb578)?;
        if flip == 0 {
            self.run(0xeb588, 0xeb590)?;
        } else {
            self.run(0xeb57c, 0xeb584)?;
        }
        self.run(0xeb594, 0xeb5a8)?;
        if flags & 1 != 0 {
            self.run(0xeb5ac, 0xeb60c)?;
        }
        self.run(0xeb610, 0xeb618)?;
        if flags & 2 != 0 {
            self.run(0xeb61c, 0xeb67c)?;
        }
        self.ranges(&[(0xeb680, 0xeb688), (0xeb928, 0xeb958)])
    }
    fn packed(&mut self, r: &[u8], slot: usize, p: &Part) -> Result<()> {
        let a = 0x800733d8 + slot as u32 * 188;
        if !matches!(p.flip, 0 | 2) || h(r, a + 0x14) > 1 {
            return Err("packed時鐘分支不在已驗範圍".into());
        }
        let bank = h(r, a + 0x54);
        self.run(0xebb94, 0xebbdc)?;
        if bank & 0x8000 == 0 {
            self.run(0xebbe0, 0xebbe4)?;
            if bank == 13 {
                self.run(0xebbe8, 0xebbf4)?;
                self.run(0xebc44, 0xebc44)?;
            } else {
                self.run(0xebc20, 0xebc30)?;
            }
        } else {
            self.run(0xebbf8, 0xebc04)?;
            if bank & 0x7fff == 1 {
                self.run(0xebc08, 0xebc14)?;
                self.run(0xebc3c, 0xebc44)?;
            } else {
                self.run(0xebc18, 0xebc1c)?;
                if bank & 0x7fff == 16 {
                    self.run(0xebc20, 0xebc30)?;
                } else {
                    self.run(0xebc34, 0xebc44)?;
                }
            }
        }
        self.run(0xebc48, 0xebccc)?;
        if h(r, a + 0x56) & 0x8000 == 0 {
            self.run(0xebcd0, 0xebcf8)?;
            self.upload(p.width / 4, p.height)?;
            self.run(0xebcfc, 0xebd3c)?;
            self.clear(h(r, p.cache) + p.width / 4)?;
            self.run(0xebd40, 0xebd80)?;
            self.clear(h(r, p.cache) - 1)?;
            self.run(0xebd84, 0xebd8c)?;
        }
        self.run(0xebd90, 0xebda0)?;
        if p.flip & 2 == 0 {
            self.run(0xebdb4, 0xebdc0)?;
        } else {
            self.run(0xebda4, 0xebdb0)?;
        }
        self.run(0xebdc4, 0xebde4)?;
        if p.flip == 0 {
            self.run(0xebe34, 0xebe6c)?;
        } else {
            self.run(0xebde8, 0xebe30)?;
        }
        self.run(0xebe70, 0xebe80)?;
        if p.flip == 0 {
            self.run(0xebea8, 0xebec0)?;
        } else {
            self.run(0xebe84, 0xebea4)?;
        }
        self.run(0xebec4, 0xebef8)?;
        self.transform(r, slot, p.flags, p.flip)?;
        self.run(0xebefc, 0xebf14)?;
        if h(r, a + 0x16) & 0x8000 == 0 {
            self.run(0xebf18, 0xebf20)?;
        }
        self.run(0xebf24, 0xebf48)?;
        if byte(r, a + 0x18) != 0 {
            self.run(0xebf4c, 0xebf70)?;
        } else {
            self.run(0xebf74, 0xebf80)?;
        }
        self.run(0xebf84, 0xebf90)?;
        if byte(r, p.cache + 0x1b) != 0 {
            self.run(0xebf94, 0xec02c)?;
        } else {
            self.run(0xec030, 0xec040)?;
            if p.flags & 8 != 0 {
                self.run(0xec044, 0xec080)?;
                if p.flags & 0x10 != 0 {
                    self.run(0xec084, 0xec090)?;
                }
                self.run(0xec094, 0xec0a4)?;
                if p.flags & 0x20 != 0 {
                    self.run(0xec0a8, 0xec0b4)?;
                }
                self.run(0xec0b8, 0xec0c8)?;
                if p.flags & 0x40 != 0 {
                    self.run(0xec0cc, 0xec0d8)?;
                }
                self.run(0xec0dc, 0xec0e4)?;
            } else {
                self.run(0xec0e8, 0xec0f0)?;
            }
            self.run(0xec0f4, 0xec0f4)?;
        }
        self.run(0xec0f8, 0xec100)?;
        for _ in 0..3 {
            self.run(0xec104, 0xec12c)?;
        }
        self.run(0xec130, 0xec1b4)
    }
    fn ordinary(&mut self, r: &[u8], slot: usize, p: &Part) -> Result<()> {
        let f = p.part_flags;
        let a = 0x800733d8 + slot as u32 * 188;
        if !matches!(p.flip, 0 | 2) || h(r, a + 0x14) > 1 {
            return Err("普通部件時鐘分支不在已驗範圍".into());
        }
        self.run(0xec1d0, 0xec250)?;
        if f & 4 != 0 {
            self.run(0xec254, 0xec26c)?;
            if f & 2 != 0 {
                self.run(0xec270, 0xec280)?;
            }
        }
        self.run(0xec284, 0xec288)?;
        if f & 8 != 0 {
            self.run(0xec28c, 0xec2a4)?;
            if f & 1 != 0 {
                self.run(0xec2a8, 0xec2b8)?;
            }
        }
        self.run(0xec2bc, 0xec2c0)?;
        if f & 16 != 0 {
            self.run(0xec2c4, 0xec2dc)?;
            if f & 2 == 0 {
                self.run(0xec2e0, 0xec2f0)?;
            }
        }
        self.run(0xec2f4, 0xec2f8)?;
        if f & 32 != 0 {
            self.run(0xec2fc, 0xec314)?;
            if f & 1 == 0 {
                self.run(0xec318, 0xec324)?;
            }
        }
        self.run(0xec328, 0xec334)?;
        if h(r, a + 0x14) == 0 {
            self.run(0xec348, 0xec354)?;
        } else {
            self.run(0xec338, 0xec344)?;
        }
        self.run(0xec358, 0xec378)?;
        if p.flip == 0 {
            self.run(0xec3c8, 0xec400)?;
        } else {
            self.run(0xec37c, 0xec3c4)?;
        }
        self.run(0xec404, 0xec418)?;
        if p.flags & 3 != 0 {
            self.run(0xec41c, 0xec44c)?;
            self.transform(r, slot, p.flags, p.flip)?;
            self.run(0xec450, 0xec458)?;
        }
        self.run(0xec45c, 0xec46c)?;
        if h(r, a + 0x16) & 0x8000 == 0 {
            self.run(0xec470, 0xec478)?;
        }
        self.run(0xec47c, 0xec4b0)?;
        if p.page_low & 1 != 0 {
            self.run(0xec4b4, 0xec4b8)?;
        }
        self.run(0xec4bc, 0xec4c0)?;
        if p.page_low & 2 != 0 {
            self.run(0xec4c4, 0xec4c8)?;
        }
        self.run(0xec4cc, 0xec4d0)?;
        if f & 4 != 0 {
            self.run(0xec4d4, 0xec4d4)?;
        }
        self.run(0xec4d8, 0xec4dc)?;
        if f & 8 != 0 {
            self.run(0xec4e0, 0xec4e0)?;
        }
        self.run(0xec4e4, 0xec4e8)?;
        if f & 16 != 0 {
            self.run(0xec4ec, 0xec4ec)?;
        }
        self.run(0xec4f0, 0xec4f4)?;
        if f & 32 != 0 {
            self.run(0xec4f8, 0xec4f8)?;
        }
        self.run(0xec4fc, 0xec50c)?;
        if (f as u32 & 2) ^ p.flip == 0 {
            self.run(0xec510, 0xec514)?;
            if f & 1 == 0 {
                self.run(0xec518, 0xec538)?;
            } else {
                self.run(0xec53c, 0xec55c)?;
                self.run(0xec5bc, 0xec5bc)?;
            }
        } else {
            self.run(0xec560, 0xec564)?;
            if f & 1 == 0 {
                self.run(0xec568, 0xec590)?;
            } else {
                self.run(0xec594, 0xec5bc)?;
            }
        }
        self.run(0xec5c0, 0xec5cc)?;
        if byte(r, a + 0x18) != 0 {
            self.run(0xec5d0, 0xec5f4)?;
        } else {
            self.run(0xec5f8, 0xec604)?;
        }
        self.run(0xec608, 0xec618)?;
        if p.flags & 8 != 0 {
            self.run(0xec61c, 0xec658)?;
            if p.flags & 0x10 != 0 {
                self.run(0xec65c, 0xec668)?;
            }
            self.run(0xec66c, 0xec67c)?;
            if p.flags & 0x20 != 0 {
                self.run(0xec680, 0xec68c)?;
            }
            self.run(0xec690, 0xec6a0)?;
            if p.flags & 0x40 != 0 {
                self.run(0xec6a4, 0xec6b0)?;
            }
            self.run(0xec6b4, 0xec6bc)?;
        } else {
            self.run(0xec6c0, 0xec6c8)?;
        }
        self.run(0xec6cc, 0xec6d8)?;
        for _ in 0..3 {
            self.run(0xec6dc, 0xec704)?;
        }
        self.run(0xec708, 0xec720)?;
        if f & 0x100 != 0 {
            self.run(0xec724, 0xec724)?;
        }
        self.ranges(&[(0xec728, 0xec7a8), (0xec938, 0xec954)])
    }
    fn scene(&mut self, r: &[u8]) -> Result<()> {
        let word = |a: u32| {
            let i = (a & 0x1fffff) as usize;
            u32::from_le_bytes(r[i..i + 4].try_into().unwrap())
        };
        self.run(0xeb998, 0xeba48)?;
        for slot in 0..256 {
            let a = 0x800733d8 + slot as u32 * 188;
            let bank = h(r, a + 0x54);
            let frame = h(r, a + 0x56);
            self.run(0xeba4c, 0xeba70)?;
            if bank != 0 && frame != 0 {
                let flags = byte(r, a + 0x19);
                self.run(0xeba74, 0xeba88)?;
                if flags & 0x80 != 0 {
                    self.run(0xeba8c, 0xebaa8)?;
                    if (slot as u32 ^ word(0x8003c998)) & 1 != 0 {
                        self.run(0xec958, 0xec978)?;
                        continue;
                    }
                }
                self.run(0xebaac, 0xebaf4)?;
                let x = h(r, a + 2).wrapping_add(h(r, 0x8006c39c));
                if x.wrapping_add(512) > 1024 {
                    self.run(0xec958, 0xec978)?;
                    continue;
                }
                self.run(0xebaf8, 0xebb08)?;
                let y = h(r, a + 6).wrapping_add(h(r, 0x8006c3a0));
                if y.wrapping_add(512) > 1024 {
                    self.run(0xec958, 0xec978)?;
                    continue;
                }
                self.run(0xebb0c, 0xebb1c)?;
                if frame & 0x7fff == 0 {
                    self.run(0xec958, 0xec978)?;
                    continue;
                }
                self.run(0xebb20, 0xebb34)?;
                if bank & 0x8000 == 0 {
                    self.run(0xebb38, 0xebb54)?;
                } else {
                    self.run(0xebb58, 0xebb78)?;
                }
                self.run(0xebb7c, 0xebb90)?;
                let p = Part::read(r, slot, 0)?;
                if p.packed {
                    self.packed(r, slot, &p)?;
                } else {
                    self.run(0xec1b8, 0xec1c8)?;
                    if p.n != 0 {
                        self.run(0xec1cc, 0xec1cc)?;
                        for part in 0..p.n as usize {
                            let p = Part::read(r, slot, part)?;
                            self.ordinary(r, slot, &p)?;
                        }
                    }
                }
            }
            self.run(0xec958, 0xec978)?;
        }
        self.run(0xec97c, 0xec9ac)
    }
}
pub struct ActorClockPlan {
    pub data: ActorDataPlan,
    endpoint: State,
    costs: Vec<u32>,
    biu: u32,
}
impl ActorClockPlan {
    pub fn prepare(
        ram: &[u8],
        scratch: &[u8],
        before: &State,
        context: TextureContext,
        irq: u32,
        biu: u32,
    ) -> Result<Self> {
        let data = ActorDataPlan::prepare(ram, scratch, before, context, irq)?;
        if biu != 125320 || before.icache.iter().any(|l| l.valid & !15 != 0) {
            return Err("物件BIU／cache不在支援範圍".into());
        }
        for &(a, n, digest) in SDK_IDENTITIES {
            if format!("{:x}", Sha256::digest(&ram[a..a + n])) != digest {
                return Err("時鐘SDK來源身份不符".into());
            }
        }
        let mut c = Clock {
            state: before.clone(),
            costs: Vec::with_capacity(16000),
            width: 0,
            scale_x: 0,
            scale_y: 0,
        };
        c.scene(ram)?;
        data.architecture.apply(&mut c.state);
        Ok(Self {
            data,
            endpoint: c.state,
            costs: c.costs,
            biu,
        })
    }
    /// Atomic same-state SDK transaction; the scene's quads stay typed data.
    pub fn commit(&self, m: &mut psgolem_core::machine::Machine) -> Result<()> {
        use psgolem_core::{cpu::Bus, machine::phase::RamWordPatch};
        self.data.validate(
            m.bus.ram(),
            m.bus.scratch(),
            &m.cpu.state,
            m.bus.gpu.texture_context(),
            m.bus.irq_registers().1,
        )?;
        if m.bus.biu() != self.biu {
            return Err("物件BIU計畫已過期".into());
        }
        let patches: Vec<_> = m
            .bus
            .ram()
            .chunks_exact(4)
            .zip(self.data.geometry.ram.chunks_exact(4))
            .enumerate()
            .filter(|(_, (a, b))| a != b)
            .map(|(i, (_, b))| RamWordPatch {
                offset: (i * 4) as u32,
                value: u32::from_le_bytes(b.try_into().unwrap()),
            })
            .collect();
        let jobs: Vec<_> = self
            .data
            .geometry
            .images
            .iter()
            .map(|j| j.job.clone())
            .collect();
        if jobs.is_empty() {
            return m.commit_cpu_data_phase(
                &self.endpoint,
                &patches,
                &self.data.scratch,
                &self.costs,
                100000,
            );
        }
        m.commit_texture_data_phase(
            &self.endpoint,
            &patches,
            &self.data.scratch,
            &self.costs,
            100000,
            &jobs,
        )
    }
    pub fn endpoint(&self) -> &State {
        &self.endpoint
    }
    pub fn costs(&self) -> &[u32] {
        &self.costs
    }
}
