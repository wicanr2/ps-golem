//! Integer movie timelines driven by caller-supplied emulation cycles.
//! Spec027 section9. GPL-3.0-or-later. No game addresses or native-state writes.
use super::{Frame, Player};
use std::{
    io::Read,
    path::Path,
    process::{Command, Stdio},
};

const MAX_METADATA: usize = 8 * 1024 * 1024;

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Timeline {
    numerator: u32,
    denominator: u32,
    timestamps: Vec<u64>,
    duration: u64,
}

impl Timeline {
    fn parse(text: &str, expected: usize) -> Result<Self, String> {
        if expected == 0 || expected > 100000 {
            return Err("Invalid expected timeline frame count".into());
        }
        let mut timestamps = Vec::with_capacity(expected);
        let mut base = None;
        let mut duration = None;
        for line in text.lines().filter(|s| !s.is_empty()) {
            let (key, value) = line.split_once('=').ok_or("Malformed timeline metadata")?;
            if let Some(index) = key
                .strip_prefix("frames.frame.")
                .and_then(|s| s.strip_suffix(".best_effort_timestamp"))
            {
                let index: usize = index.parse().map_err(|_| "Invalid timeline frame index")?;
                if index != timestamps.len() || index >= expected {
                    return Err("Duplicate, missing or excess timeline frame index".into());
                }
                timestamps.push(
                    value
                        .parse::<u64>()
                        .map_err(|_| "Invalid frame timestamp")?,
                );
            } else if key == "streams.stream.0.time_base" {
                if base.is_some() {
                    return Err("Duplicate timeline time base".into());
                }
                let value = value
                    .strip_prefix('"')
                    .and_then(|s| s.strip_suffix('"'))
                    .ok_or("Malformed timeline time base")?;
                let (n, d) = value
                    .split_once('/')
                    .ok_or("Malformed rational time base")?;
                base = Some((
                    n.parse::<u32>()
                        .map_err(|_| "Invalid time-base numerator")?,
                    d.parse::<u32>()
                        .map_err(|_| "Invalid time-base denominator")?,
                ));
            } else if key == "streams.stream.0.duration_ts" {
                if duration.is_some() {
                    return Err("Duplicate timeline duration".into());
                }
                duration = Some(
                    value
                        .parse::<u64>()
                        .map_err(|_| "Invalid timeline duration")?,
                );
            }
        }
        let (numerator, denominator) = base.ok_or("Missing timeline time base")?;
        let duration = duration.ok_or("Missing timeline duration")?;
        if numerator == 0
            || denominator == 0
            || timestamps.len() != expected
            || timestamps.first() != Some(&0)
            || timestamps.windows(2).any(|w| w[0] >= w[1])
            || duration <= *timestamps.last().ok_or("Empty timeline")?
        {
            return Err("Invalid timeline bounds, ordering or frame count".into());
        }
        Ok(Self {
            numerator,
            denominator,
            timestamps,
            duration,
        })
    }

    fn probe(ffprobe: &Path, path: &Path, frames: usize) -> Result<Self, String> {
        let mut child = Command::new(ffprobe)
            .args([
                "-v",
                "error",
                "-threads",
                "2",
                "-select_streams",
                "v:0",
                "-show_entries",
                "frame=best_effort_timestamp:stream=time_base,duration_ts",
                "-of",
                "flat",
            ])
            .arg(path)
            .stdin(Stdio::null())
            .stdout(Stdio::piped())
            .stderr(Stdio::null())
            .spawn()
            .map_err(|e| format!("Timeline ffprobe: {e}"))?;
        let mut bytes = Vec::new();
        let result = child
            .stdout
            .take()
            .ok_or("Missing timeline stdout")?
            .take((MAX_METADATA + 1) as u64)
            .read_to_end(&mut bytes);
        if let Err(e) = result {
            let _ = child.kill();
            let _ = child.wait();
            return Err(format!("Read timeline metadata: {e}"));
        }
        if bytes.len() > MAX_METADATA {
            let _ = child.kill();
            let _ = child.wait();
            return Err("Timeline metadata exceeds 8 MiB limit".into());
        }
        let status = child
            .wait()
            .map_err(|e| format!("Wait for timeline probe: {e}"))?;
        if !status.success() {
            return Err(format!("Timeline ffprobe exited {status}"));
        }
        let text = std::str::from_utf8(&bytes).map_err(|_| "Invalid timeline metadata encoding")?;
        Self::parse(text, frames)
    }

    pub fn time_base(&self) -> (u32, u32) {
        (self.numerator, self.denominator)
    }
    pub fn timestamps(&self) -> &[u64] {
        &self.timestamps
    }
    pub fn duration_ticks(&self) -> u64 {
        self.duration
    }

    /// Half-open source timestamp intervals. All products fit in u128 even
    /// at the full u64 timestamp/u32 clock and rational-component bounds.
    pub fn index_at_cycles(&self, elapsed: u64, clock_hz: u32) -> Result<Option<usize>, String> {
        if clock_hz == 0 {
            return Err("Movie clock must be nonzero".into());
        }
        let time = elapsed as u128 * self.denominator as u128;
        let tick = self.numerator as u128 * clock_hz as u128;
        if time >= self.duration as u128 * tick {
            return Ok(None);
        }
        let next = self
            .timestamps
            .partition_point(|&p| p as u128 * tick <= time);
        Ok(Some(next - 1))
    }
}

pub struct TimedPlayer {
    player: Player,
    timeline: Timeline,
    clock: Option<u32>,
    elapsed: Option<u64>,
    finished: bool,
    stopped: bool,
}

impl TimedPlayer {
    pub fn open(ffmpeg: &Path, ffprobe: &Path, path: &Path) -> Result<Self, String> {
        let player = Player::open(ffmpeg, ffprobe, path)?;
        // Any probe/parse error drops Player, owning and reaping its decoder.
        let timeline = Timeline::probe(ffprobe, path, player.info().frames)?;
        Ok(Self {
            player,
            timeline,
            clock: None,
            elapsed: None,
            finished: false,
            stopped: false,
        })
    }
    pub fn info(&self) -> &super::ClipInfo {
        self.player.info()
    }
    pub fn timeline(&self) -> &Timeline {
        &self.timeline
    }
    pub fn current(&self) -> Option<&Frame> {
        self.player.current()
    }
    pub fn decoded_frames(&self) -> usize {
        self.player.decoded_frames()
    }
    pub fn is_finished(&self) -> bool {
        self.finished
    }
    pub fn is_stopped(&self) -> bool {
        self.player.is_stopped()
    }
    pub fn set_paused(&mut self, paused: bool) {
        if !self.finished && !self.stopped {
            self.player.set_paused(paused);
        }
    }
    /// elapsed is measured from movie start by the caller's emulated clock.
    /// Pause freezes the image; the frontend must freeze emulation time too.
    pub fn present_at_cycles(
        &mut self,
        elapsed: u64,
        clock_hz: u32,
    ) -> Result<Option<&Frame>, String> {
        if self.stopped {
            return Err("Timed movie decoder is stopped".into());
        }
        if clock_hz == 0 {
            return Err("Movie clock must be nonzero".into());
        }
        if self.clock.is_some_and(|clock| clock != clock_hz) {
            return Err("Changing the movie clock requires a new session".into());
        }
        self.clock = Some(clock_hz);
        if self.finished {
            return Ok(None);
        }
        if self.player.paused() {
            return Ok(self.player.current());
        }
        if self.elapsed.is_some_and(|previous| elapsed < previous) {
            return Err("Backward movie time requires a new session".into());
        }
        let index = self.timeline.index_at_cycles(elapsed, clock_hz)?;
        self.elapsed = Some(elapsed);
        if let Some(index) = index {
            return self.player.present(index).map(Some);
        }
        self.player.stop()?;
        self.finished = true;
        Ok(None)
    }
    pub fn stop(&mut self) -> Result<(), String> {
        self.stopped = true;
        self.player.stop()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::{fs, path::PathBuf};

    #[test]
    fn rational_intervals_and_large_clocks_are_exact() {
        let t = Timeline {
            numerator: 1,
            denominator: 10,
            timestamps: vec![0, 2, 7],
            duration: 9,
        };
        for (time, want) in [
            (0, Some(0)),
            (19, Some(0)),
            (20, Some(1)),
            (69, Some(1)),
            (70, Some(2)),
            (89, Some(2)),
            (90, None),
            (u64::MAX, None),
        ] {
            assert_eq!(t.index_at_cycles(time, 100).unwrap(), want);
        }
        assert!(t.index_at_cycles(0, 0).is_err());
        let t = Timeline {
            numerator: 1,
            denominator: 1,
            timestamps: vec![0, u64::MAX - 2],
            duration: u64::MAX,
        };
        assert_eq!(t.index_at_cycles(u64::MAX - 3, 1).unwrap(), Some(0));
        assert_eq!(t.index_at_cycles(u64::MAX - 2, 1).unwrap(), Some(1));
        assert_eq!(t.index_at_cycles(u64::MAX, 1).unwrap(), None);
        let t = Timeline {
            numerator: u32::MAX,
            denominator: u32::MAX,
            timestamps: vec![0, u64::MAX - 2],
            duration: u64::MAX,
        };
        assert_eq!(t.index_at_cycles(u64::MAX, u32::MAX).unwrap(), Some(0));
    }

    #[test]
    fn malformed_timelines_are_rejected() {
        let good="frames.frame.0.best_effort_timestamp=0\nframes.frame.1.best_effort_timestamp=7\nstreams.stream.0.time_base=\"1/10\"\nstreams.stream.0.duration_ts=9\n";
        assert_eq!(Timeline::parse(good, 2).unwrap().timestamps(), &[0, 7]);
        for bad in [
            good.replace("=7", "=0"),
            good.replace("=7", "=-1"),
            good.replace("=7", "=N/A"),
            good.replace("frame.1", "frame.2"),
            good.replace("=0", "=1"),
            good.replace("1/10", "0/10"),
            good.replace("1/10", "1/0"),
            good.replace("duration_ts=9", "duration_ts=7"),
            good.replace("time_base=", "bad_base="),
            format!("{good}streams.stream.0.duration_ts=9\n"),
            format!("{good}streams.stream.0.time_base=\"1/10\"\n"),
        ] {
            assert!(Timeline::parse(&bad, 2).is_err(), "{bad}");
        }
        assert!(Timeline::parse(good, 1).is_err());
        assert!(Timeline::parse(good, 0).is_err());
        assert!(Timeline::parse(good, 100001).is_err());
    }

    // Independent oracle protocol: FFmpeg n5.1.9 framecrcenc.c, LGPL-2.1-or-later,
    // av_adler32_update starts at zero. Standard Adler32 computed here, no codec port.
    fn adler(bytes: &[u8]) -> u32 {
        let (mut a, mut b) = (0u64, 0u64);
        for block in bytes.chunks(5552) {
            for &value in block {
                a += value as u64;
                b += a;
            }
            a %= 65521;
            b %= 65521;
        }
        ((b << 16) | a) as u32
    }

    #[test]
    fn retained_cfr_and_vfr_follow_integer_cycles_without_duplicate_frames() {
        let Ok(root) = std::env::var("PSGOLEM_RUST_MOVIE_FIXTURES") else {
            eprintln!("SKIP: select retained real movie/timeline references");
            return;
        };
        let root = PathBuf::from(root);
        let ffmpeg =
            PathBuf::from(std::env::var("PSGOLEM_RUST_FFMPEG").unwrap_or("/usr/bin/ffmpeg".into()));
        let ffprobe = PathBuf::from(
            std::env::var("PSGOLEM_RUST_FFPROBE").unwrap_or("/usr/bin/ffprobe".into()),
        );
        for (case, filename) in [
            ("logo-vfr", "hd-movie-LOGO-full-HD-v2.mp4"),
            ("short-cfr", "hd-movie-h264-with-audio-v1.mp4"),
        ] {
            let data = fs::read(root.join(format!("hd-movie-timed-{case}-v3.bin"))).unwrap();
            assert_eq!(&data[..8], b"PSGTIME1");
            let word = |i| u32::from_le_bytes(data[i..i + 4].try_into().unwrap());
            let long = |i| u64::from_le_bytes(data[i..i + 8].try_into().unwrap());
            let count = word(24) as usize;
            let clock = word(28);
            let end = long(40);
            assert_eq!(data.len(), 48 + count * 20);
            let mut player = TimedPlayer::open(&ffmpeg, &ffprobe, &root.join(filename)).unwrap();
            assert_eq!(player.info().frames, count);
            assert_eq!(player.timeline().time_base(), (word(16), word(20)));
            assert_eq!(player.timeline().duration_ticks(), long(32));
            assert_eq!(player.decoded_frames(), 1);
            let mut previous = 0;
            for index in 0..count {
                let at = long(48 + index * 20 + 8);
                assert_eq!(player.timeline().timestamps()[index], long(48 + index * 20));
                if index > 0 {
                    let frame = player.present_at_cycles(at - 1, clock).unwrap().unwrap();
                    assert_eq!(frame.index, index - 1);
                    assert_eq!(adler(&frame.rgb), previous);
                }
                let want = word(48 + index * 20 + 16);
                let frame = player.present_at_cycles(at, clock).unwrap().unwrap();
                assert_eq!(frame.index, index);
                assert_eq!(adler(&frame.rgb), want, "{case} frame{index}");
                assert_eq!(
                    player.present_at_cycles(at, clock).unwrap().unwrap().index,
                    index
                );
                assert_eq!(player.decoded_frames(), index + 1);
                if index == 3 {
                    player.set_paused(true);
                    assert_eq!(
                        player.present_at_cycles(end, clock).unwrap().unwrap().index,
                        index
                    );
                    assert_eq!(player.decoded_frames(), index + 1);
                    player.set_paused(false);
                    assert!(player.present_at_cycles(at - 1, clock).is_err());
                    assert!(player.present_at_cycles(at, clock + 1).is_err());
                    assert!(player.present_at_cycles(at, 0).is_err());
                }
                previous = want;
            }
            assert!(player.present_at_cycles(end - 1, clock).unwrap().is_some());
            assert!(player.present_at_cycles(end, clock).unwrap().is_none());
            assert!(player.is_finished() && player.is_stopped());
            assert!(player.present_at_cycles(end, clock).unwrap().is_none());
            player.stop().unwrap();
            player.stop().unwrap();
            assert!(player.present_at_cycles(0, clock).is_err());
            let mut skip = TimedPlayer::open(&ffmpeg, &ffprobe, &root.join(filename)).unwrap();
            let at = long(48 + (count - 1) * 20 + 8);
            assert_eq!(
                skip.present_at_cycles(at, clock).unwrap().unwrap().index,
                count - 1
            );
            assert_eq!(skip.decoded_frames(), count);
            skip.stop().unwrap();
            assert!(skip.is_stopped());
        }
    }
}
