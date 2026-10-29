//! Optional host-side CD pipeline metadata, spec027 section12.
//! GPL-3.0-or-later. No FIFO, IRQ, cycle or disc-content changes.
use crate::disc::SECTOR_SIZE;
use std::collections::VecDeque;

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct SectorRead {
    pub lba: i32,
    pub mode: u8,
    pub file: u8,
    pub channel: u8,
    pub submode: u8,
    pub coding: u8,
    pub sync_valid: bool,
    pub xa_headers_match: bool,
    pub header: [u32; 8],
}
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct VideoHeader {
    pub chunk: u16,
    pub chunks: u16,
    pub frame: u32,
    pub bytes: u32,
    pub width: u16,
    pub height: u16,
}
impl SectorRead {
    pub fn from_sector(lba: i32, raw: &[u8; SECTOR_SIZE]) -> Self {
        Self {
            lba,
            mode: raw[15],
            file: raw[16],
            channel: raw[17],
            submode: raw[18],
            coding: raw[19],
            sync_valid: raw[0] == 0 && raw[11] == 0 && raw[1..11].iter().all(|&x| x == 255),
            xa_headers_match: raw[16..20] == raw[20..24],
            header: std::array::from_fn(|i| {
                u32::from_le_bytes(raw[24 + i * 4..28 + i * 4].try_into().unwrap())
            }),
        }
    }
    pub fn valid_mode2(&self) -> bool {
        self.mode == 2 && self.sync_valid && self.xa_headers_match && self.channel < 32
    }
    /// Public format contract verified against FFmpeg n5.1.9 psxstr.c;
    /// metadata extraction only, no external codec implementation ported.
    pub fn video_header(&self) -> Option<VideoHeader> {
        if !self.valid_mode2()
            || !matches!(self.submode & 14, 2 | 8)
            || self.header[0] != 0x80010160
        {
            return None;
        }
        let chunk = self.header[1] as u16;
        let chunks = (self.header[1] >> 16) as u16;
        let width = self.header[4] as u16;
        let height = (self.header[4] >> 16) as u16;
        let bytes = self.header[3];
        if chunks == 0
            || chunk >= chunks
            || bytes == 0
            || bytes as u64 > chunks as u64 * 2016
            || width == 0
            || height == 0
        {
            return None;
        }
        Some(VideoHeader {
            chunk,
            chunks,
            frame: self.header[2],
            bytes,
            width,
            height,
        })
    }
}

#[derive(Clone, Debug, Default)]
pub struct SectorReads {
    pub reads: Vec<SectorRead>,
    pub dropped: u64,
    pub discontinuity: bool,
}
#[derive(Clone, Default)]
pub(super) struct ReadObserver {
    reads: VecDeque<SectorRead>,
    dropped: u64,
    discontinuity: bool,
}
impl ReadObserver {
    pub(super) fn reset(&mut self) {
        self.reads.clear();
        self.discontinuity = true;
    }
    pub(super) fn record(&mut self, lba: i32, raw: &[u8; SECTOR_SIZE]) {
        if self.reads.len() == 32 {
            self.reads.pop_front();
            self.dropped = self.dropped.saturating_add(1);
        }
        self.reads.push_back(SectorRead::from_sector(lba, raw));
    }
    pub(super) fn take(&mut self) -> SectorReads {
        SectorReads {
            reads: self.reads.drain(..).collect(),
            dropped: std::mem::take(&mut self.dropped),
            discontinuity: std::mem::take(&mut self.discontinuity),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn source_observer_is_optional_bounded_and_reports_reset() {
        let mut cd = super::super::Cdrom::new();
        assert!(cd.read_observer.is_none());
        assert!(cd.take_sector_reads().reads.is_empty());
        cd.enable_sector_reads(true);
        assert!(cd.take_sector_reads().discontinuity);
        let raw = [0; SECTOR_SIZE];
        for i in 0..40 {
            cd.read_observer.as_mut().unwrap().record(i, &raw);
        }
        let batch = cd.take_sector_reads();
        assert_eq!(batch.dropped, 8);
        assert_eq!(batch.reads.len(), 32);
        assert_eq!((batch.reads[0].lba, batch.reads[31].lba), (8, 39));
        assert!(cd.take_sector_reads().reads.is_empty());
        cd.soft_reset();
        assert!(cd.take_sector_reads().discontinuity);
        cd.enable_sector_reads(false);
        assert!(cd.read_observer.is_none());
    }
}
