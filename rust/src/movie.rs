//! Bounded CPU H.264 pull player, spec027 section5.
//! GPL-3.0-or-later. Uses explicitly selected FFmpeg tools as subprocesses;
//! no codec source or original game data is embedded. No native-state hook.
use std::{
    collections::BTreeMap,
    fs,
    io::Read,
    path::{Path, PathBuf},
    process::{Child, ChildStdout, Command, Stdio},
};

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct ClipInfo {
    pub width: usize,
    pub height: usize,
    pub fps_num: u32,
    pub fps_den: u32,
    pub frames: usize,
}
#[derive(Debug)]
pub struct Frame {
    pub index: usize,
    pub width: usize,
    pub height: usize,
    pub rgb: Vec<u8>,
}
pub struct Player {
    info: ClipInfo,
    child: Option<Child>,
    stdout: Option<ChildStdout>,
    frame: Option<Frame>,
    decoded: usize,
    paused: bool,
}
impl ClipInfo {
    pub fn probe(ffprobe: &Path, path: &Path) -> Result<Self, String> {
        let meta = fs::metadata(path).map_err(|e| format!("Movie file: {e}"))?;
        if !meta.is_file() || meta.len() == 0 || meta.len() > 256 * 1024 * 1024 {
            return Err("Movie must be a nonempty regular file no larger than 256 MiB".into());
        }
        let output = Command::new(ffprobe)
            .args([
                "-v",
                "error",
                "-threads",
                "2",
                "-count_frames",
                "-select_streams",
                "v:0",
                "-show_entries",
                "stream=codec_name,width,height,r_frame_rate,nb_read_frames",
                "-of",
                "default=noprint_wrappers=1:nokey=0",
            ])
            .arg(path)
            .stdin(Stdio::null())
            .output()
            .map_err(|e| format!("ffprobe: {e}"))?;
        if !output.status.success() {
            return Err(format!(
                "ffprobe {}: {}",
                output.status,
                String::from_utf8_lossy(&output.stderr[..output.stderr.len().min(2048)])
            ));
        }
        if output.stdout.len() > 65536 {
            return Err("Movie metadata exceeds limit".into());
        }
        let text = String::from_utf8(output.stdout).map_err(|_| "Invalid metadata encoding")?;
        let fields: BTreeMap<_, _> = text.lines().filter_map(|s| s.split_once('=')).collect();
        if fields.get("codec_name") != Some(&"h264") {
            return Err("Only H.264 movie input is supported by this slice".into());
        }
        let num = |name: &str| -> Result<usize, String> {
            fields
                .get(name)
                .ok_or_else(|| format!("Missing movie metadata {name}"))?
                .parse()
                .map_err(|_| format!("Invalid movie metadata {name}"))
        };
        let fps = fields.get("r_frame_rate").ok_or("Missing frame rate")?;
        let (n, d) = fps.split_once('/').ok_or("Invalid rational frame rate")?;
        let info = Self {
            width: num("width")?,
            height: num("height")?,
            fps_num: n.parse().map_err(|_| "Invalid fps numerator")?,
            fps_den: d.parse().map_err(|_| "Invalid fps denominator")?,
            frames: num("nb_read_frames")?,
        };
        if info.width == 0
            || info.height == 0
            || info.width > 4096
            || info.height > 2160
            || info.width * info.height > 3840 * 2160
            || info.frames == 0
            || info.frames > 100000
            || info.fps_num == 0
            || info.fps_den == 0
            || info.fps_num as u64 > 120 * info.fps_den as u64
        {
            return Err("Movie dimensions, frame count or frame rate exceed limits".into());
        }
        Ok(info)
    }
}
impl Player {
    /// Preflight and decode the first frame before it can be selected for display.
    pub fn open(ffmpeg: &Path, ffprobe: &Path, path: &Path) -> Result<Self, String> {
        let info = ClipInfo::probe(ffprobe, path)?;
        let mut child = Command::new(ffmpeg)
            .args([
                "-hide_banner",
                "-nostdin",
                "-v",
                "error",
                "-threads",
                "2",
                "-i",
            ])
            .arg(path)
            .args([
                "-map",
                "0:v:0",
                "-an",
                "-filter_threads",
                "1",
                "-pix_fmt",
                "rgb24",
                "-f",
                "rawvideo",
                "pipe:1",
            ])
            .stdin(Stdio::null())
            .stdout(Stdio::piped())
            .stderr(Stdio::null())
            .spawn()
            .map_err(|e| format!("ffmpeg: {e}"))?;
        let stdout = child.stdout.take().expect("Selected piped decoder stdout");
        let frame = Frame {
            index: 0,
            width: info.width,
            height: info.height,
            rgb: vec![0; info.width * info.height * 3],
        };
        let mut player = Self {
            info,
            child: Some(child),
            stdout: Some(stdout),
            frame: Some(frame),
            decoded: 0,
            paused: false,
        };
        player.read_next()?;
        Ok(player)
    }
    pub fn info(&self) -> &ClipInfo {
        &self.info
    }
    pub fn decoded_frames(&self) -> usize {
        self.decoded
    }
    pub fn paused(&self) -> bool {
        self.paused
    }
    pub fn current(&self) -> Option<&Frame> {
        self.frame.as_ref()
    }
    pub fn is_stopped(&self) -> bool {
        self.child.is_none()
    }
    pub fn set_paused(&mut self, paused: bool) {
        self.paused = paused
    }
    fn read_next(&mut self) -> Result<(), String> {
        let result = self
            .stdout
            .as_mut()
            .ok_or("Movie decoder is stopped")?
            .read_exact(&mut self.frame.as_mut().ok_or("Movie frame is absent")?.rgb);
        if let Err(e) = result {
            let index = self.decoded;
            let state = self
                .child
                .as_mut()
                .and_then(|c| c.try_wait().ok())
                .flatten();
            let _ = self.stop();
            return Err(format!(
                "Incomplete RGB frame {index}: {e}; decoder exit {state:?}"
            ));
        }
        self.frame.as_mut().unwrap().index = self.decoded;
        self.decoded += 1;
        Ok(())
    }
    /// The caller supplies a native presentation frame index. No wall clock is read.
    pub fn present(&mut self, index: usize) -> Result<&Frame, String> {
        if self.child.is_none() {
            return Err("Movie decoder is stopped".into());
        }
        if self.paused {
            return Ok(self.frame.as_ref().unwrap());
        }
        if index >= self.info.frames {
            return Err(format!(
                "Movie frame {index} exceeds {} frames",
                self.info.frames
            ));
        }
        let current = self.frame.as_ref().unwrap().index;
        if index < current {
            return Err("Backward frame request requires an explicit new movie session".into());
        }
        while self.decoded <= index {
            self.read_next()?
        }
        Ok(self.frame.as_ref().unwrap())
    }
    /// Own and reap the decoder, including early skip and partially read streams.
    pub fn stop(&mut self) -> Result<(), String> {
        self.stdout.take();
        let mut error = None;
        if let Some(mut child) = self.child.take() {
            match child.try_wait() {
                Ok(Some(_)) => {}
                Ok(None) => {
                    if let Err(e) = child.kill() {
                        error = Some(format!("Stop movie decoder: {e}"))
                    }
                }
                Err(e) => error = Some(format!("Inspect movie decoder: {e}")),
            }
            if let Err(e) = child.wait() {
                error = Some(format!("Wait for movie decoder: {e}"))
            }
        }
        self.frame = None;
        self.paused = false;
        if let Some(e) = error {
            Err(e)
        } else {
            Ok(())
        }
    }
}
impl Drop for Player {
    fn drop(&mut self) {
        let _ = self.stop();
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    fn crc(bytes: &[u8]) -> u32 {
        let table: [u32; 256] = std::array::from_fn(|i| {
            let mut x = i as u32;
            for _ in 0..8 {
                x = if x & 1 != 0 {
                    0xedb88320 ^ (x >> 1)
                } else {
                    x >> 1
                }
            }
            x
        });
        let mut c = !0;
        for &b in bytes {
            c = table[((c ^ b as u32) & 255) as usize] ^ (c >> 8)
        }
        !c
    }
    fn selected() -> Option<(PathBuf, PathBuf, PathBuf)> {
        let Ok(root) = std::env::var("PSGOLEM_RUST_MOVIE_FIXTURES") else {
            eprintln!("SKIP: select genuine retained movie/reference and FFmpeg tools");
            return None;
        };
        Some((
            root.into(),
            std::env::var("PSGOLEM_RUST_FFMPEG")
                .unwrap_or_else(|_| "/usr/bin/ffmpeg".into())
                .into(),
            std::env::var("PSGOLEM_RUST_FFPROBE")
                .unwrap_or_else(|_| "/usr/bin/ffprobe".into())
                .into(),
        ))
    }
    #[test]
    fn native_display_events_drive_all_h264_frames() {
        let Some((root, ffmpeg, ffprobe)) = selected() else {
            return;
        };
        let b = fs::read(root.join("hd-movie-player-reference-v1.bin")).unwrap();
        assert_eq!(&b[..8], b"PSGMOV01");
        let word = |i| u32::from_le_bytes(b[i..i + 4].try_into().unwrap());
        assert_eq!(
            [word(8), word(12), word(16), word(20), word(24)],
            [1440, 1080, 15, 1, 156]
        );
        assert_eq!(b.len(), 28 + 156 * 8);
        let mut p =
            Player::open(&ffmpeg, &ffprobe, &root.join("hd-movie-native-logo-v1.mp4")).unwrap();
        assert_eq!(p.info().frames, 156);
        assert_eq!(p.decoded_frames(), 1);
        let mut prev = 0;
        for i in 0..156 {
            let field = word(28 + i * 8);
            assert!(field > prev);
            prev = field;
            let want = word(32 + i * 8);
            assert_eq!(
                crc(&p.present(i).unwrap().rgb),
                want,
                "display frame {i} at native field {field}"
            );
            let n = p.decoded_frames();
            assert_eq!(p.present(i).unwrap().index, i);
            assert_eq!(p.decoded_frames(), n);
            p.set_paused(true);
            assert_eq!(p.present(usize::MAX).unwrap().index, i);
            assert_eq!(p.decoded_frames(), n);
            p.set_paused(false);
        }
        assert_eq!(p.decoded_frames(), 156);
        assert!(p.present(156).is_err());
        assert!(p.present(0).is_err());
        p.stop().unwrap();
        p.stop().unwrap();
        assert!(p.is_stopped());
        assert!(p.current().is_none());
        assert!(p.present(0).is_err());
        eprintln!("RUST_MOVIE_PLAYER: 156 native display events, all RGB frames match independent CRC, repeat/pause/stop passed; one-frame cache");
    }
    #[test]
    fn h264_forward_skip_and_early_close() {
        let Some((root, ffmpeg, ffprobe)) = selected() else {
            return;
        };
        assert!(ClipInfo::probe(&ffprobe, &root.join("absent-movie.mp4")).is_err());
        let mut p = Player::open(
            &ffmpeg,
            &ffprobe,
            &root.join("hd-movie-h264-transport-v1.mp4"),
        )
        .unwrap();
        assert_eq!(p.info().frames, 45);
        assert_eq!(p.present(20).unwrap().index, 20);
        assert_eq!(p.decoded_frames(), 21);
        p.stop().unwrap();
        assert!(p.is_stopped());
        eprintln!("RUST_MOVIE_SKIP: valid 45-frame H264 forward selection, early stop/reap and missing-file rejection passed");
    }
}
