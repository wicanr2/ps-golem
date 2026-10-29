//! Source/timing/ownership control for one movie, spec027 section14.
//! GPL-3.0-or-later. No Machine, input routing or native-state writes.
use super::{
    source::{Selection, SourceProfile, SourceTracker},
    Frame, TimedPlayer,
};
use crate::cdrom::SectorReads;

struct Prepared {
    key: String,
    player: TimedPlayer,
}
pub struct Session {
    sources: SourceTracker,
    prepared: Option<Prepared>,
    clock_hz: u32,
    epoch: Option<u64>,
    last_cycle: Option<u64>,
    paused: bool,
}
impl Session {
    pub fn new(profiles: Vec<SourceProfile>, clock_hz: u32) -> Result<Self, String> {
        if clock_hz == 0 {
            return Err("Movie clock must be nonzero".into());
        }
        Ok(Self {
            sources: SourceTracker::new(profiles)?,
            prepared: None,
            clock_hz,
            epoch: None,
            last_cycle: None,
            paused: false,
        })
    }
    pub fn active(&self) -> bool {
        self.sources.active()
    }
    pub fn selection(&self) -> &Selection {
        self.sources.current()
    }
    pub fn pending_key(&self) -> Option<&str> {
        self.sources.pending_key()
    }
    pub fn paused(&self) -> bool {
        self.paused
    }
    pub fn has_decoder(&self) -> bool {
        self.prepared.is_some()
    }
    pub fn decoded_frames(&self) -> usize {
        self.prepared
            .as_ref()
            .map_or(0, |p| p.player.decoded_frames())
    }
    pub fn needs_prepare(&self) -> Option<&str> {
        if self.active() {
            return None;
        }
        let key = self.pending_key()?;
        if self.prepared.as_ref().is_some_and(|p| p.key == key) {
            None
        } else {
            Some(key)
        }
    }
    fn clear_prepared(&mut self) -> Result<(), String> {
        if let Some(mut p) = self.prepared.take() {
            p.player.stop()
        } else {
            Ok(())
        }
    }
    pub fn observe_batch(&mut self, batch: &SectorReads) -> Result<(), String> {
        self.sources.observe_batch(batch);
        if !self.active()
            && self
                .prepared
                .as_ref()
                .is_some_and(|p| Some(p.key.as_str()) != self.pending_key())
        {
            self.clear_prepared()?;
        }
        Ok(())
    }
    /// Caller verifies source/movie identity and prepares the first frame.
    /// A rejected incoming player is dropped and reaped without replacing an
    /// active session. No filesystem/asset-pack policy is embedded here.
    pub fn prepare(&mut self, key: &str, mut player: TimedPlayer) -> Result<(), String> {
        if self.active() || self.pending_key() != Some(key) {
            return Err("Movie preparation does not match an idle pending source".into());
        }
        if player.is_stopped() || player.is_finished() || player.current().is_none() {
            return Err("Movie preparation is not ready".into());
        }
        self.clear_prepared()?;
        player.set_paused(self.paused);
        self.prepared = Some(Prepared {
            key: key.into(),
            player,
        });
        Ok(())
    }
    /// Called only at the caller's observed native movie-start boundary.
    /// Repeated calls preserve the committed source and original epoch.
    pub fn begin(&mut self, cycle: u64) -> &Selection {
        if !self.active() {
            let ready = self.prepared.as_ref().map(|p| p.key.clone());
            self.sources.begin(|key| ready.as_deref() == Some(key));
            self.epoch = Some(cycle);
            self.last_cycle = Some(cycle);
        }
        self.selection()
    }
    /// The caller must stop guest stepping/audio admission while paused.
    pub fn set_paused(&mut self, paused: bool) {
        self.paused = paused;
        if let Some(p) = self.prepared.as_mut() {
            p.player.set_paused(paused)
        }
    }
    pub fn present_at_cycles(&mut self, cycle: u64) -> Result<Option<&Frame>, String> {
        if !self.active() {
            return Ok(None);
        }
        let previous = self.last_cycle.ok_or("Active movie has no clock")?;
        if cycle < previous {
            return Err("Backward movie time requires a new session".into());
        }
        if self.paused && cycle != previous {
            return Err("Guest movie clock advanced while paused".into());
        }
        if !matches!(self.selection(), Selection::Hd(_)) {
            self.last_cycle = Some(cycle);
            return Ok(None);
        }
        let elapsed = cycle
            .checked_sub(self.epoch.ok_or("Active movie has no epoch")?)
            .ok_or("Movie precedes its epoch")?;
        let available = match self.prepared.as_mut() {
            Some(p) => p
                .player
                .present_at_cycles(elapsed, self.clock_hz)
                .map(|f| f.is_some()),
            None => Err("Committed HD movie has no decoder".into()),
        };
        match available {
            Ok(true) => {
                self.last_cycle = Some(cycle);
                Ok(self.prepared.as_ref().and_then(|p| p.player.current()))
            }
            state => {
                let error = match state {
                    Err(e) => e,
                    _ => "HD movie ended before native movie exit".into(),
                };
                let stopped = self.abort_hd();
                self.last_cycle = Some(cycle);
                match stopped {
                    Ok(()) => Err(error),
                    Err(e) => Err(format!("{error}; {e}")),
                }
            }
        }
    }
    /// A fault is one-way native fallback until the actual native end.
    pub fn abort_hd(&mut self) -> Result<(), String> {
        self.sources.abort_hd();
        self.clear_prepared()
    }
    /// Called on the actual native exit, whether natural or accepted skip.
    /// A button press alone must never call this method.
    pub fn end(&mut self) -> Result<(), String> {
        let stopped = self.clear_prepared();
        self.sources.end();
        self.epoch = None;
        self.last_cycle = None;
        self.paused = false;
        stopped
    }
}
impl Drop for Session {
    fn drop(&mut self) {
        let _ = self.end();
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::cdrom::SectorRead;
    use std::{
        env,
        path::{Path, PathBuf},
    };
    const CLOCK: u32 = 33868800;
    fn profiles() -> Vec<SourceProfile> {
        vec![
            SourceProfile {
                key: "a".into(),
                lba: 10,
                sectors: 10,
                width: 320,
                height: 240,
            },
            SourceProfile {
                key: "b".into(),
                lba: 20,
                sectors: 10,
                width: 320,
                height: 240,
            },
        ]
    }
    fn observe(s: &mut Session, lba: i32) {
        let mut raw = [0; 2352];
        raw[1..11].fill(255);
        raw[15] = 2;
        raw[16..20].copy_from_slice(&[0, 1, 2, 0]);
        raw[20..24].copy_from_slice(&[0, 1, 2, 0]);
        for (i, v) in [0x80010160u32, 2 << 16, 1, 3000, 240 << 16 | 320, 0, 0, 0]
            .into_iter()
            .enumerate()
        {
            raw[24 + i * 4..28 + i * 4].copy_from_slice(&v.to_le_bytes());
        }
        s.observe_batch(&SectorReads {
            reads: vec![SectorRead::from_sector(lba, &raw)],
            ..Default::default()
        })
        .unwrap();
    }
    fn fixture() -> Option<PathBuf> {
        match env::var("PSGOLEM_RUST_MOVIE_FIXTURES") {
            Ok(root) => Some(PathBuf::from(root).join("hd-movie-h264-with-audio-v1.mp4")),
            Err(_) => {
                eprintln!("SKIP: select retained genuine CFR45 H264 source");
                None
            }
        }
    }
    fn player(path: &Path) -> TimedPlayer {
        TimedPlayer::open(
            Path::new("/usr/bin/ffmpeg"),
            Path::new("/usr/bin/ffprobe"),
            path,
        )
        .unwrap()
    }
    #[test]
    fn session_configuration_unready_and_fixed_native_choice() {
        assert!(Session::new(profiles(), 0).is_err());
        let mut s = Session::new(profiles(), CLOCK).unwrap();
        observe(&mut s, 10);
        assert_eq!(s.needs_prepare(), Some("a"));
        assert_eq!(s.begin(50), &Selection::Native);
        observe(&mut s, 20);
        assert_eq!(s.begin(100), &Selection::Native);
        assert!(s.present_at_cycles(100).unwrap().is_none());
        s.end().unwrap();
        assert!(!s.active() && !s.has_decoder());
        assert_eq!(s.pending_key(), None);
    }
    #[test]
    fn session_prepare_rejects_wrong_or_late_sources() {
        let Some(path) = fixture() else { return };
        let mut s = Session::new(profiles(), CLOCK).unwrap();
        observe(&mut s, 10);
        assert!(s.prepare("b", player(&path)).is_err());
        assert!(!s.has_decoder());
        s.begin(0);
        assert!(s.prepare("a", player(&path)).is_err());
        assert_eq!(s.selection(), &Selection::Native);
        s.end().unwrap();
        observe(&mut s, 10);
        s.prepare("a", player(&path)).unwrap();
        assert_eq!(s.needs_prepare(), None);
        assert_eq!(s.begin(0), &Selection::Hd("a".into()));
        assert!(s.prepare("a", player(&path)).is_err());
        assert!(s.has_decoder());
    }
    #[test]
    fn session_epoch_pause_resume_and_actual_end() {
        let Some(path) = fixture() else { return };
        let mut s = Session::new(profiles(), CLOCK).unwrap();
        observe(&mut s, 10);
        s.prepare("a", player(&path)).unwrap();
        s.begin(1000);
        assert_eq!(s.present_at_cycles(1000).unwrap().unwrap().index, 0);
        s.begin(9000);
        let before = s.present_at_cycles(1000).unwrap().unwrap().rgb.clone();
        s.set_paused(true);
        for _ in 0..100 {
            assert_eq!(s.present_at_cycles(1000).unwrap().unwrap().rgb, before);
        }
        assert!(s.present_at_cycles(1001).is_err());
        assert!(s.paused());
        assert_eq!(s.decoded_frames(), 1);
        s.set_paused(false);
        assert_eq!(
            s.present_at_cycles(1000 + CLOCK as u64 / 15)
                .unwrap()
                .unwrap()
                .index,
            1
        );
        assert!(s.present_at_cycles(1000).is_err());
        assert_eq!(s.selection(), &Selection::Hd("a".into()));
        s.end().unwrap();
        s.end().unwrap();
        assert!(!s.has_decoder() && !s.active());
    }
    #[test]
    fn session_early_eof_and_fault_stay_native_until_end() {
        let Some(path) = fixture() else { return };
        let mut s = Session::new(profiles(), CLOCK).unwrap();
        observe(&mut s, 10);
        s.prepare("a", player(&path)).unwrap();
        s.begin(0);
        assert!(s.present_at_cycles(4 * CLOCK as u64).is_err());
        assert!(!s.has_decoder());
        observe(&mut s, 20);
        assert_eq!(s.begin(5 * CLOCK as u64), &Selection::Native);
        s.end().unwrap();
        observe(&mut s, 20);
        s.prepare("b", player(&path)).unwrap();
        assert_eq!(s.begin(6 * CLOCK as u64), &Selection::Hd("b".into()));
        s.abort_hd().unwrap();
        assert!(!s.has_decoder());
        assert_eq!(s.selection(), &Selection::Native);
    }
    #[test]
    fn session_obsolete_preparations_are_stopped() {
        let Some(path) = fixture() else { return };
        let mut s = Session::new(profiles(), CLOCK).unwrap();
        observe(&mut s, 10);
        s.prepare("a", player(&path)).unwrap();
        assert!(s.has_decoder());
        observe(&mut s, 20);
        assert!(!s.has_decoder());
        assert_eq!(s.needs_prepare(), Some("b"));
        s.prepare("b", player(&path)).unwrap();
        s.observe_batch(&SectorReads {
            discontinuity: true,
            ..Default::default()
        })
        .unwrap();
        assert!(!s.has_decoder());
    }
}
