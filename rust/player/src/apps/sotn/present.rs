//! Bounded ST0 field publication, 026 §16.14.3, GPL-3.0-or-later.
//! Ready source, completed submission and scanout selection are distinct.
use super::field3d::Epoch;
use psgolem_core::{gpu::Scanout, scene::Canvas};
use std::sync::Arc;
type Result<T> = std::result::Result<T, String>;
pub struct Picture {
    pub epoch: Epoch,
    pub built_field: u64,
    pub origin: [u32; 2],
    pub width: u32,
    pub height: u32,
    pub rgba: Arc<Vec<u8>>,
}
impl Picture {
    pub fn from_canvas(
        epoch: Epoch,
        built_field: u64,
        origin: [u32; 2],
        canvas: Canvas,
    ) -> Result<Self> {
        let expected = match epoch.draw_buffer {
            0x8003cb08 => [0, 0],
            0x800542fc => [256, 0],
            _ => return Err("未知場景buffer".into()),
        };
        if origin != expected
            || canvas.width != 1024
            || canvas.height != 960
            || canvas.words.len() != 1024 * 960
            || built_field == 0
        {
            return Err("場景目標或畫布形狀不符".into());
        }
        let mut rgba = canvas.rgba();
        for pixel in rgba.chunks_exact_mut(4) {
            pixel[3] = 255;
        }
        Ok(Self {
            epoch,
            built_field,
            origin,
            width: 1024,
            height: 960,
            rgba: Arc::new(rgba),
        })
    }
}
#[derive(Default)]
pub struct Frames {
    pending: Option<Arc<Picture>>,
    slots: [Option<Arc<Picture>>; 2],
    last_counter: Option<u32>,
    last_field: Option<u64>,
    front: Option<Arc<Picture>>,
}
impl Frames {
    pub fn mark(&self) -> String {
        format!(
            "{:?} {:?} {:?} {:?} {:?}",
            self.last_counter,
            self.last_field,
            self.pending.as_ref().map(|p| p.epoch),
            self.slots.each_ref().map(|p| p.as_ref().map(|p| p.epoch)),
            self.front.as_ref().map(|p| p.epoch)
        )
    }
    pub fn stage(&mut self, picture: Picture) -> Result<()> {
        let valid_origin = match picture.epoch.draw_buffer {
            0x8003cb08 => Some([0, 0]),
            0x800542fc => Some([256, 0]),
            _ => None,
        };
        if valid_origin != Some(picture.origin)
            || picture.width != 1024
            || picture.height != 960
            || picture.rgba.len() != 1024 * 960 * 4
            || picture.built_field == 0
        {
            return Err("圖像來源或邊界不符".into());
        }
        if self.pending.is_some()
            || self
                .last_counter
                .is_some_and(|v| v.checked_add(1) != Some(picture.epoch.counter))
        {
            return Err("前場尚未提交或epoch不連續".into());
        }
        self.last_counter = Some(picture.epoch.counter);
        self.pending = Some(Arc::new(picture));
        Ok(())
    }
    /// An unmodelled submission overwrites this native target. Drop its old HD.
    pub fn invalidate(&mut self, origin: [u32; 2]) -> Result<()> {
        if origin[1] != 0 || ![0, 256].contains(&origin[0]) {
            return Err("未知覆寫目標".into());
        }
        self.slots[(origin[0] / 256) as usize] = None;
        if self.pending.as_ref().is_some_and(|p| p.origin == origin) {
            self.pending = None;
        }
        Ok(())
    }
    pub fn submit(&mut self, epoch: Epoch, field: u64) -> Result<()> {
        let p = self.pending.as_ref().ok_or("沒有待提交場景")?;
        if p.epoch != epoch || p.built_field.checked_add(1) != Some(field) {
            return Err("提交epoch或場邊界不符".into());
        }
        let slot = (p.origin[0] / 256) as usize;
        self.slots[slot] = self.pending.take();
        Ok(())
    }
    /// Call once at each complete field. Repeats return the frozen same choice.
    /// None means this entire field remains native; never upgrade it mid-field.
    pub fn present(
        &mut self,
        field: u64,
        scanout: Scanout,
        hd: bool,
    ) -> Result<Option<Arc<Picture>>> {
        if self.last_field.is_some_and(|f| field < f) {
            return Err("發布場數倒退".into());
        }
        if self.last_field == Some(field) {
            return Ok(self.front.clone());
        }
        let valid = hd
            && scanout.enabled
            && !scanout.depth24
            && !scanout.interlaced
            && scanout.width == 256
            && scanout.height == 240
            && scanout.origin[1] == 0
            && [0, 256].contains(&scanout.origin[0]);
        self.front = if valid {
            self.slots[(scanout.origin[0] / 256) as usize]
                .as_ref()
                .filter(|p| {
                    p.built_field
                        .checked_add(1)
                        .is_some_and(|submitted| submitted < field)
                })
                .cloned()
        } else {
            None
        };
        self.last_field = Some(field);
        Ok(self.front.clone())
    }
}
