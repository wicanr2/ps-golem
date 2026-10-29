//! Source choice from verified ISO extents and actual CD pipeline reads.
//! Spec027 section12, GPL-3.0-or-later. Caller validates asset SHA/bindings;
//! this module contains no game addresses, fixed LBAs or movie-pack format.
use crate::cdrom::{SectorRead, SectorReads};
use std::collections::BTreeSet;

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct SourceProfile {
    pub key: String,
    pub lba: u32,
    pub sectors: u32,
    pub width: u16,
    pub height: u16,
}
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Selection {
    Native,
    Hd(String),
}

pub struct SourceTracker {
    profiles: Vec<SourceProfile>,
    pending: Option<usize>,
    active: bool,
    selection: Selection,
}
impl SourceTracker {
    pub fn new(mut profiles: Vec<SourceProfile>) -> Result<Self, String> {
        if profiles.len() > 64 {
            return Err("More than 64 movie source profiles".into());
        }
        let mut keys = BTreeSet::new();
        for p in &profiles {
            if p.key.is_empty()
                || p.key.len() > 255
                || p.key.chars().any(char::is_control)
                || !keys.insert(p.key.clone())
                || p.sectors == 0
                || p.width == 0
                || p.height == 0
                || p.lba as u64 + p.sectors as u64 > i32::MAX as u64 + 1
            {
                return Err("Invalid or duplicate movie source profile".into());
            }
        }
        profiles.sort_by_key(|p| p.lba);
        if profiles
            .windows(2)
            .any(|w| w[0].lba as u64 + w[0].sectors as u64 > w[1].lba as u64)
        {
            return Err("Overlapping movie source extents".into());
        }
        Ok(Self {
            profiles,
            pending: None,
            active: false,
            selection: Selection::Native,
        })
    }
    pub fn pending_key(&self) -> Option<&str> {
        self.pending.map(|i| self.profiles[i].key.as_str())
    }
    pub fn current(&self) -> &Selection {
        &self.selection
    }
    pub fn active(&self) -> bool {
        self.active
    }
    pub fn observe_batch(&mut self, batch: &SectorReads) {
        // The committed movie is independent of prefetches for other files.
        // Never upgrade a native session after a late preparation finishes.
        if self.active {
            return;
        }
        if batch.discontinuity || batch.dropped != 0 {
            self.pending = None;
            return;
        }
        for read in &batch.reads {
            self.observe(read);
        }
    }
    fn observe(&mut self, read: &SectorRead) {
        if !read.valid_mode2() || read.lba < 0 {
            self.pending = None;
            return;
        }
        let Some(index) = self.profiles.iter().position(|p| {
            p.lba as u64 <= read.lba as u64 && (read.lba as u64) < p.lba as u64 + p.sectors as u64
        }) else {
            self.pending = None;
            return;
        };
        if let Some(video) = read.video_header() {
            let p = &self.profiles[index];
            if (video.width, video.height) == (p.width, p.height)
                && video.chunks as u32 <= p.sectors
            {
                self.pending = Some(index);
            } else {
                self.pending = None;
            }
        } else if read.submode & 14 != 4 || self.pending != Some(index) {
            self.pending = None;
        }
    }
    pub fn begin(&mut self, ready: impl FnOnce(&str) -> bool) -> &Selection {
        if self.active {
            return &self.selection;
        }
        self.selection = match self.pending {
            Some(index) if ready(&self.profiles[index].key) => {
                Selection::Hd(self.profiles[index].key.clone())
            }
            _ => Selection::Native,
        };
        self.active = true;
        &self.selection
    }
    /// A decoder fault can only fall back for the remainder of this session.
    pub fn abort_hd(&mut self) {
        self.selection = Selection::Native;
    }
    pub fn end(&mut self) {
        self.pending = None;
        self.active = false;
        self.selection = Selection::Native;
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::disc::{Disc, SECTOR_SIZE};
    use std::{env, fs, path::PathBuf};
    fn profile(key: &str, lba: u32) -> SourceProfile {
        SourceProfile {
            key: key.into(),
            lba,
            sectors: 10,
            width: 320,
            height: 240,
        }
    }
    fn read(lba: i32, submode: u8) -> SectorRead {
        let mut raw = [0u8; SECTOR_SIZE];
        raw[1..11].fill(255);
        raw[15] = 2;
        raw[16..20].copy_from_slice(&[0, 1, submode, 0]);
        raw[20..24].copy_from_slice(&[0, 1, submode, 0]);
        for (i, value) in [0x80010160u32, 2 << 16, 1, 3000, 240 << 16 | 320, 0, 0, 0]
            .iter()
            .enumerate()
        {
            raw[24 + i * 4..28 + i * 4].copy_from_slice(&value.to_le_bytes());
        }
        SectorRead::from_sector(lba, &raw)
    }
    fn batch(read: SectorRead) -> SectorReads {
        SectorReads {
            reads: vec![read],
            ..Default::default()
        }
    }
    #[test]
    fn source_profiles_reject_ambiguous_ranges_and_keys() {
        assert!(SourceTracker::new(vec![profile("a", 10), profile("b", 19)]).is_err());
        assert!(SourceTracker::new(vec![profile("a", 10), profile("a", 20)]).is_err());
        for p in [
            SourceProfile {
                sectors: 0,
                ..profile("a", 10)
            },
            SourceProfile {
                width: 0,
                ..profile("a", 10)
            },
            profile("", 10),
            profile("a\n", 10),
            profile("a", u32::MAX),
        ] {
            assert!(SourceTracker::new(vec![p]).is_err());
        }
        assert!(
            SourceTracker::new((0..65).map(|i| profile(&i.to_string(), i * 10)).collect()).is_err()
        );
        assert!(SourceTracker::new(vec![]).is_ok());
        assert!(SourceTracker::new(vec![profile("b", 20), profile("a", 10)]).is_ok());
    }
    #[test]
    fn source_format_checks_bounds_and_accepts_data_or_video() {
        for kind in [2, 8] {
            assert_eq!(read(10, kind).video_header().unwrap().frame, 1);
        }
        assert!(read(10, 4).video_header().is_none());
        let mut bad = read(10, 2);
        bad.xa_headers_match = false;
        assert!(bad.video_header().is_none());
        bad = read(10, 2);
        bad.sync_valid = false;
        assert!(bad.video_header().is_none());
        bad = read(10, 2);
        bad.channel = 32;
        assert!(bad.video_header().is_none());
        for (word, value) in [(0, 0), (1, 0), (1, 1 << 16 | 1), (3, 0), (3, 4033), (4, 0)] {
            let mut bad = read(10, 2);
            bad.header[word] = value;
            assert!(bad.video_header().is_none());
        }
    }
    #[test]
    fn source_audio_unknown_and_discontinuity_do_not_reuse_stale_source() {
        let mut t = SourceTracker::new(vec![profile("a", 10), profile("b", 20)]).unwrap();
        t.observe_batch(&batch(read(10, 2)));
        assert_eq!(t.pending_key(), Some("a"));
        t.observe_batch(&batch(read(11, 4)));
        assert_eq!(t.pending_key(), Some("a"));
        t.observe_batch(&batch(read(0, 8)));
        assert_eq!(t.pending_key(), None);
        t.observe_batch(&batch(read(10, 2)));
        t.observe_batch(&SectorReads {
            discontinuity: true,
            ..Default::default()
        });
        assert_eq!(t.pending_key(), None);
        t.observe_batch(&batch(read(20, 8)));
        assert_eq!(t.pending_key(), Some("b"));
        t.observe_batch(&SectorReads {
            dropped: 1,
            reads: vec![read(10, 2)],
            ..Default::default()
        });
        assert_eq!(t.pending_key(), None);
        t.observe_batch(&batch(read(10, 2)));
        let mut bad = read(11, 2);
        bad.header[0] = 0;
        t.observe_batch(&batch(bad));
        assert_eq!(t.pending_key(), None);
    }
    #[test]
    fn source_choice_is_fixed_until_end_including_late_ready_and_faults() {
        let mut t = SourceTracker::new(vec![profile("a", 10), profile("b", 20)]).unwrap();
        t.observe_batch(&batch(read(10, 2)));
        assert_eq!(t.begin(|_| false), &Selection::Native);
        t.observe_batch(&batch(read(20, 2)));
        assert_eq!(t.begin(|_| true), &Selection::Native);
        t.end();
        assert_eq!(t.pending_key(), None);
        t.observe_batch(&batch(read(10, 2)));
        assert_eq!(t.begin(|k| k == "a"), &Selection::Hd("a".into()));
        t.observe_batch(&SectorReads {
            reads: vec![read(20, 2)],
            dropped: 1,
            discontinuity: true,
        });
        assert_eq!(t.current(), &Selection::Hd("a".into()));
        t.abort_hd();
        assert_eq!(t.begin(|_| true), &Selection::Native);
        t.end();
        t.observe_batch(&batch(read(20, 2)));
        assert_eq!(t.begin(|_| true), &Selection::Hd("b".into()));
    }
    #[test]
    fn source_actual_28150_reads_and_iso_extents_select_both_observed_movies() {
        let (Ok(root), Ok(cue)) = (
            env::var("PSGOLEM_RUST_MOVIE_FIXTURES"),
            env::var("PSGOLEM_RUST_MOVIE_SOURCE_CUE"),
        ) else {
            eprintln!("SKIP: select genuine source-read reference and original disc");
            return;
        };
        let b = fs::read(PathBuf::from(root).join("hd-movie-source-reference-v4.bin")).unwrap();
        assert_eq!(&b[..8], b"PSGSRC01");
        let mut pos = 8;
        fn u32_at(b: &[u8], p: &mut usize) -> u32 {
            let v = u32::from_le_bytes(b[*p..*p + 4].try_into().unwrap());
            *p += 4;
            v
        }
        fn u64_at(b: &[u8], p: &mut usize) -> u64 {
            let v = u64::from_le_bytes(b[*p..*p + 8].try_into().unwrap());
            *p += 8;
            v
        }
        let profiles = u32_at(&b, &mut pos);
        let count = u32_at(&b, &mut pos);
        assert_eq!((profiles, count), (4, 28150));
        let disc = Disc::open(cue).unwrap();
        let mut list = Vec::new();
        for _ in 0..profiles {
            let n = u32_at(&b, &mut pos) as usize;
            let key = String::from_utf8(b[pos..pos + n].to_vec()).unwrap();
            pos += n;
            let lba = u32_at(&b, &mut pos);
            let sectors = u32_at(&b, &mut pos);
            let width = u16::from_le_bytes(b[pos..pos + 2].try_into().unwrap());
            pos += 2;
            let height = u16::from_le_bytes(b[pos..pos + 2].try_into().unwrap());
            pos += 2;
            let (actual, bytes) = disc.file_extent(&key).unwrap();
            assert_eq!((actual, bytes.div_ceil(2048)), (lba, sectors));
            list.push(SourceProfile {
                key,
                lba,
                sectors,
                width,
                height,
            });
        }
        assert!(disc.file_extent("../LOGO15XA.STR").is_err());
        let mut events = Vec::new();
        for _ in 0..count {
            let _field = u64_at(&b, &mut pos);
            let cycle = u64_at(&b, &mut pos);
            let lba = u32_at(&b, &mut pos) as i32;
            let flags = &b[pos..pos + 5];
            pos += 5;
            let words: [u32; 8] = std::array::from_fn(|_| u32_at(&b, &mut pos));
            let mut raw = [0; SECTOR_SIZE];
            disc.read_sector(lba, &mut raw);
            let read = SectorRead::from_sector(lba, &raw);
            assert_eq!(
                [
                    read.mode,
                    read.file,
                    read.channel,
                    read.submode,
                    read.coding
                ],
                flags
            );
            assert_eq!(read.header, words);
            events.push((cycle, read));
        }
        let sessions = u32_at(&b, &mut pos);
        assert_eq!(sessions, 2);
        let mut boundaries = Vec::new();
        for _ in 0..sessions {
            let start = u64_at(&b, &mut pos);
            let end = u64_at(&b, &mut pos);
            let index = u32_at(&b, &mut pos) as usize;
            boundaries.push((start, Some(list[index].key.clone())));
            boundaries.push((end, None));
        }
        assert_eq!(pos, b.len());
        let mut t = SourceTracker::new(list).unwrap();
        let mut at = 0;
        for (cycle, want) in boundaries {
            while at < events.len() && events[at].0 <= cycle {
                t.observe_batch(&batch(events[at].1.clone()));
                at += 1;
            }
            if let Some(key) = want {
                assert_eq!(t.pending_key(), Some(key.as_str()));
                assert_eq!(t.begin(|_| true), &Selection::Hd(key));
            } else {
                t.end();
            }
        }
        eprintln!("SOURCE_REPLAY: all28150 actual-sector headers and four ISO extents exact; both observed source choices passed");
    }
}
