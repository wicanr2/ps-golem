//! Typed display controls, 026 §16.16, GPL-3.0-or-later.
//! Own GPU register contract; no GP1 parsing, pixel reads or cache changes.
use super::Gpu;
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum DisplayControl {
    Origin { x: u16, y: u16 },
    Ntsc256x240,
}
impl Gpu {
    pub fn apply_display_control(&mut self, job: DisplayControl) -> Result<(), String> {
        match job {
            DisplayControl::Origin { x, y } => {
                if x >= 1024 || x & 1 != 0 || y >= 512 {
                    return Err("顯示原點未對齊或越界".into());
                }
                self.display_x = x as u32;
                self.display_y = y as u32;
            }
            DisplayControl::Ntsc256x240 => self.display_mode = 0,
        }
        Ok(())
    }
}
