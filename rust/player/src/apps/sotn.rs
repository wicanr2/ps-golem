//! SotN-specific HD movie bindings, spec027 section15, GPL-3.0-or-later.
//! Native clock, sources and lifetime use the tested generic movie::Session.
//! Only fingerprints and paths are embedded, never original or HD media bytes.
use psgolem_core::{
    disc::{Disc, SECTOR_SIZE},
    machine::Machine,
    movie::{
        source::{Selection, SourceProfile},
        Session, TimedPlayer,
    },
};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::{
    collections::BTreeMap,
    fs,
    io::Read,
    path::{Path, PathBuf},
    sync::Arc,
    time::Instant,
};

struct Binding {
    path: PathBuf,
    sha: &'static str,
    frames: usize,
}
pub struct Overlay {
    session: Session,
    bindings: BTreeMap<String, Binding>,
    ffmpeg: PathBuf,
    ffprobe: PathBuf,
    was_movie: bool,
    cache: Option<(String, usize, Arc<Vec<u8>>, u32)>,
    pub hd_fields: u64,
    pub opens: u64,
    pub events: Vec<Value>,
    pub errors: Vec<String>,
    pub last_index: Option<usize>,
    pub last_crc: Option<u32>,
    pub last_source: String,
    diagnostics: bool,
}
const CLIPS: [(&str, &str, &str, &str, u32, usize); 2] = [
    (
        "LOGO15XA.STR",
        "hd-movie-LOGO-full-HD-v2.mp4",
        "5799bdb50503fdccdb5c6742c3c0123ad7795f938bc11fa0379a8929cf9c8c94",
        "02a28c5ac00b112a7bccfd0dc22e2190c388d2f49bb44767bcfbcd2e41abe6a6",
        1680,
        93,
    ),
    (
        "OPN_WS.STR",
        "hd-movie-opn-hd-v5/hd-movie-OPN-native-HD-v5.mp4",
        "26ebb4c8f37af8b60fc358a98f5f5f09e9a0741b95d930805e85ba3cadadc56c",
        "fae19b9c8718a2a87a416ba9fcd024158b7c6f1e8bdd547bbdd16a623df96232",
        9240,
        921,
    ),
];
fn file_sha(path: &Path) -> Result<String, String> {
    let meta = fs::metadata(path).map_err(|e| e.to_string())?;
    if !meta.is_file() || meta.len() == 0 || meta.len() > 256 * 1024 * 1024 {
        return Err("影片必須為1至256MiB的一般檔案".into());
    }
    let mut f = fs::File::open(path).map_err(|e| e.to_string())?;
    let mut h = Sha256::new();
    let mut b = vec![0; 1024 * 1024];
    loop {
        let n = f.read(&mut b).map_err(|e| e.to_string())?;
        if n == 0 {
            break;
        }
        h.update(&b[..n]);
    }
    Ok(format!("{:x}", h.finalize()))
}
fn adler(bytes: &[u8]) -> u32 {
    let (mut a, mut b) = (1u64, 0u64);
    for chunk in bytes.chunks(5552) {
        for &v in chunk {
            a += v as u64;
            b += a;
        }
        a %= 65521;
        b %= 65521;
    }
    ((b << 16) | a) as u32
}
impl Overlay {
    pub fn load(
        disc: &Disc,
        root: &Path,
        ffmpeg: PathBuf,
        ffprobe: PathBuf,
        diagnostics: bool,
    ) -> Result<Self, String> {
        if disc.boot_file() != "SLPM_860.23" {
            return Err("這個影片綁定只支援SLPM_860.23；其他光碟請用-native".into());
        }
        if !root.is_dir() {
            return Err("HD影片目錄不存在".into());
        }
        let (mut profiles, mut bindings, mut errors) = (Vec::new(), BTreeMap::new(), Vec::new());
        for (key, relative, raw_sha, hd_sha, sectors, frames) in CLIPS {
            let source = (|| -> Result<SourceProfile, String> {
                let (lba, bytes) = disc.file_extent(key)?;
                if bytes.div_ceil(2048) != sectors
                    || lba as u64 + sectors as u64 > i32::MAX as u64 + 1
                {
                    return Err("原片extent不符".into());
                }
                let mut h = Sha256::new();
                let mut raw = [0; SECTOR_SIZE];
                for i in 0..sectors {
                    disc.read_sector((lba + i) as i32, &mut raw);
                    h.update(raw);
                }
                if format!("{:x}", h.finalize()) != raw_sha {
                    return Err("原片SHA256不符".into());
                }
                Ok(SourceProfile {
                    key: key.into(),
                    lba,
                    sectors,
                    width: 320,
                    height: 240,
                })
            })();
            let profile = match source {
                Ok(p) => p,
                Err(e) => {
                    errors.push(format!("{key}：{e}，該段原生"));
                    continue;
                }
            };
            profiles.push(profile);
            let path = root.join(relative);
            match file_sha(&path) {
                Ok(sha) if sha == hd_sha => {
                    bindings.insert(
                        key.into(),
                        Binding {
                            path,
                            sha: hd_sha,
                            frames,
                        },
                    );
                }
                value => errors.push(format!(
                    "{key}：HD素材缺少或SHA256不符({value:?})，該段原生"
                )),
            }
        }
        for e in &errors {
            eprintln!("HD影片：{e}")
        }
        Ok(Self {
            session: Session::new(profiles, super::super::worker::CLOCK as u32)?,
            bindings,
            ffmpeg,
            ffprobe,
            was_movie: false,
            cache: None,
            hd_fields: 0,
            opens: 0,
            events: Vec::new(),
            errors,
            last_index: None,
            last_crc: None,
            last_source: "native".into(),
            diagnostics,
        })
    }
    pub fn pause(&mut self, value: bool) {
        self.session.set_paused(value);
    }
    pub fn update(
        &mut self,
        m: &Machine,
        batch: &psgolem_core::cdrom::SectorReads,
    ) -> Result<Option<super::super::worker::Image>, String> {
        self.session.observe_batch(batch)?;
        let movie = m.bus.gpu.status() & (1 << 21) != 0;
        self.last_index = None;
        self.last_crc = None;
        self.last_source = "native".into();
        if movie && !self.was_movie {
            // Prepare before this complete field is published. Reads alone never
            // launch decoders, and native video is never published then upgraded.
            let key = self.session.needs_prepare().map(str::to_owned);
            if let Some(key) = &key {
                if let Some(binding) = self.bindings.get(key) {
                    let start = Instant::now();
                    let result = (|| -> Result<(), String> {
                        if file_sha(&binding.path)? != binding.sha {
                            return Err("準備前素材SHA256改變".into());
                        }
                        self.opens += 1;
                        let p = TimedPlayer::open(&self.ffmpeg, &self.ffprobe, &binding.path)?;
                        if (p.info().width, p.info().height, p.info().frames)
                            != (1440, 1080, binding.frames)
                        {
                            return Err("HD影片尺寸或格數不符".into());
                        }
                        self.session.prepare(key, p)
                    })();
                    self.events.push(
                        json!({"event":"prepare","field":m.field(),"cycles":m.cycles(),"key":key,
                    "wall_seconds":start.elapsed().as_secs_f64(),"ok":result.is_ok()}),
                    );
                    if let Err(e) = result {
                        eprintln!("HD影片準備失敗：{e}");
                        self.errors.push(e);
                    }
                }
            }
            let selection = self.session.begin(m.cycles());
            self.events.push(json!({"event":"begin","field":m.field(),"cycles":m.cycles(),"pending":key,
                "selection":match selection{Selection::Native=>"native",Selection::Hd(k)=>k.as_str()}}));
        }
        if !movie && self.was_movie {
            self.events
                .push(json!({"event":"end","field":m.field(),"cycles":m.cycles()}));
            self.session.end()?;
            self.cache = None;
        }
        self.was_movie = movie;
        if !movie {
            return Ok(None);
        }
        let key = match self.session.selection() {
            Selection::Hd(k) => k.clone(),
            _ => return Ok(None),
        };
        match self.session.present_at_cycles(m.cycles()) {
            Ok(Some(f)) => {
                let same = self
                    .cache
                    .as_ref()
                    .is_some_and(|(k, i, _, _)| k == &key && *i == f.index);
                if !same {
                    let rgba = f
                        .rgb
                        .chunks_exact(3)
                        .flat_map(|v| [v[0], v[1], v[2], 255])
                        .collect();
                    self.cache = Some((
                        key.clone(),
                        f.index,
                        Arc::new(rgba),
                        if self.diagnostics { adler(&f.rgb) } else { 0 },
                    ));
                }
                let (_, index, rgba, crc) = self.cache.as_ref().unwrap();
                self.hd_fields += 1;
                self.last_index = Some(*index);
                self.last_crc = Some(*crc);
                self.last_source = key.clone();
                Ok(Some(super::super::worker::Image {
                    field: m.field(),
                    w: f.width as u32,
                    h: f.height as u32,
                    rgba: rgba.clone(),
                    movie: Some((key, *index)),
                }))
            }
            Ok(None) => Ok(None),
            Err(e) => {
                eprintln!("HD影片回退原生：{e}");
                self.errors.push(e);
                self.cache = None;
                Ok(None)
            }
        }
    }
    pub fn finish(&mut self, field: u64, cycles: u64) -> Result<(), String> {
        if self.session.active() {
            self.events
                .push(json!({"event":"close","field":field,"cycles":cycles}));
        }
        self.session.end()?;
        self.cache = None;
        Ok(())
    }
    pub fn report(&self) -> Value {
        json!({"hd_fields":self.hd_fields,"decoder_opens":self.opens,"events":self.events,
        "errors":self.errors,"decoder_alive":self.session.has_decoder(),"source_bindings":self.bindings.len()})
    }
}
#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn checksum_reference_and_block_boundary() {
        assert_eq!(adler(b"Wikipedia"), 0x11e60398);
        let bytes = vec![255u8; 12000];
        let mut a = 1u32;
        let mut b = 0u32;
        for v in &bytes {
            a = (a + *v as u32) % 65521;
            b = (b + a) % 65521;
        }
        assert_eq!(adler(&bytes), (b << 16) | a);
    }
    #[test]
    fn fixed_bindings_are_unique_and_bounded() {
        assert_ne!(CLIPS[0].0, CLIPS[1].0);
        for (_, path, raw, hd, sectors, frames) in CLIPS {
            assert!(!Path::new(path).is_absolute());
            assert!(!path.contains(".."));
            for sha in [raw, hd] {
                assert_eq!(sha.len(), 64);
                assert!(sha.bytes().all(|c| c.is_ascii_hexdigit()));
            }
            assert!(sectors > 0 && frames > 0);
        }
    }
}
