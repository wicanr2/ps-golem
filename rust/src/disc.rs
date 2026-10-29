//! Read-only CUE/BIN and ISO tools. 009 §§1,7 / 026 §9.
//! Port of internal/disc/disc.go at f00311b; GPL-3.0-or-later.
use std::{
    fs::{self, File},
    io::{Read, Seek, SeekFrom},
    path::Path,
    sync::{Arc, Mutex},
};
pub const SECTOR_SIZE: usize = 2352;
#[derive(Clone, Debug)]
pub struct Track {
    pub number: i32,
    pub audio: bool,
    pub index0: i32,
    pub index1: i32,
}
#[derive(Debug)]
pub struct Disc {
    file: Mutex<File>,
    sectors: i32,
    pub tracks: Vec<Track>,
    region: String,
    boot: String,
    pub fallback: Option<String>,
}
fn err(s: impl Into<String>) -> String {
    s.into()
}
pub fn bcd(v: i32) -> u8 {
    ((v / 10) << 4 | v % 10) as u8
}
pub fn unbcd(v: u8) -> i32 {
    (v >> 4) as i32 * 10 + (v & 15) as i32
}
fn msf(v: i32) -> [u8; 3] {
    let v = v.max(0);
    [bcd(v / 4500), bcd(v / 75 % 60), bcd(v % 75)]
}
impl Disc {
    pub fn open(path: impl AsRef<Path>) -> Result<Arc<Self>, String> {
        let path = path.as_ref();
        let data = fs::read(path).map_err(|e| e.to_string())?;
        // Preserve ASCII syntax in Big5 CUEs; a non-UTF-8 filename deliberately
        // cannot match and uses the same sole-BIN fallback as the Go source.
        let text = String::from_utf8_lossy(&data);
        let mut name = None;
        let mut tracks: Vec<Track> = Vec::new();
        for line in text.lines() {
            let f: Vec<_> = line.split_whitespace().collect();
            if f.is_empty() {
                continue;
            }
            match f[0].to_ascii_uppercase().as_str() {
                "FILE" => {
                    if name.is_some() {
                        return Err(err("Only one FILE supported"));
                    }
                    let i = line.find('"').ok_or("Invalid FILE")?;
                    let j = line.rfind('"').ok_or("Invalid FILE")?;
                    if j <= i {
                        return Err(err("Invalid FILE"));
                    }
                    name = Some(line[i + 1..j].to_owned());
                }
                "TRACK" => {
                    if f.len() < 3 {
                        return Err(err("Invalid TRACK"));
                    }
                    let mode = f[2].to_ascii_uppercase();
                    if !["AUDIO", "MODE2/2352", "MODE1/2352"].contains(&mode.as_str()) {
                        return Err(err("Unsupported track format"));
                    }
                    tracks.push(Track {
                        number: f[1].parse().unwrap_or(0),
                        audio: mode == "AUDIO",
                        index0: -1,
                        index1: -1,
                    });
                }
                "INDEX" => {
                    if f.len() < 3 || tracks.is_empty() {
                        return Err(err("Invalid INDEX"));
                    }
                    let m: Vec<_> = f[2].split(':').map(str::parse::<i32>).collect();
                    if m.len() != 3 || m.iter().any(Result::is_err) {
                        return Err(err("Invalid MSF"));
                    }
                    let v = (m[0].as_ref().unwrap() * 60 + m[1].as_ref().unwrap()) * 75
                        + m[2].as_ref().unwrap();
                    let t = tracks.last_mut().unwrap();
                    match f[1] {
                        "00" => t.index0 = v,
                        "01" => t.index1 = v,
                        _ => {}
                    }
                }
                _ => {}
            }
        }
        let name = name.ok_or("No FILE")?;
        if tracks.is_empty() {
            return Err(err("No TRACK"));
        }
        for t in &mut tracks {
            if t.index1 < 0 {
                return Err(err("Missing INDEX 01"));
            }
            if t.index0 < 0 {
                t.index0 = t.index1
            }
        }
        let dir = path.parent().unwrap_or(Path::new("."));
        let mut bin = dir.join(&name);
        let mut fallback = None;
        if !bin.exists() {
            let mut bins = Vec::new();
            for e in fs::read_dir(dir).map_err(|e| e.to_string())? {
                let p = e.map_err(|e| e.to_string())?.path();
                if p.extension()
                    .is_some_and(|x| x.to_string_lossy().eq_ignore_ascii_case("bin"))
                {
                    bins.push(p)
                }
            }
            if bins.len() != 1 {
                return Err(format!(
                    "CUE filename unavailable; {} BIN candidates",
                    bins.len()
                ));
            }
            bin = bins.remove(0);
            fallback = Some(format!(
                "CUE filename {name:?} unavailable; using {}",
                bin.file_name().unwrap().to_string_lossy()
            ));
        }
        let file = File::open(bin).map_err(|e| e.to_string())?;
        let sectors =
            (file.metadata().map_err(|e| e.to_string())?.len() / SECTOR_SIZE as u64) as i32;
        let mut d = Self {
            file: Mutex::new(file),
            sectors,
            tracks,
            region: String::new(),
            boot: String::new(),
            fallback,
        };
        if let Ok(cnf) = d.read_file_limit("SYSTEM.CNF", u32::MAX) {
            for line in String::from_utf8_lossy(&cnf).lines() {
                if let Some((k, v)) = line.split_once('=') {
                    if k.trim().eq_ignore_ascii_case("BOOT") {
                        let v = v.trim().rsplit([':', '\\', '/']).next().unwrap_or("");
                        d.boot = v.split(';').next().unwrap_or("").to_owned();
                        break;
                    }
                }
            }
        }
        d.region = match d.boot.as_bytes().get(2).map(u8::to_ascii_uppercase) {
            Some(b'P') => "SCEI",
            Some(b'U') => "SCEA",
            Some(b'E') => "SCEE",
            _ => "",
        }
        .to_owned();
        Ok(Arc::new(d))
    }
    pub fn lead_out(&self) -> i32 {
        self.sectors
    }
    pub fn region(&self) -> &str {
        &self.region
    }
    pub fn boot_file(&self) -> &str {
        &self.boot
    }
    pub fn read_sector(&self, lba: i32, out: &mut [u8; SECTOR_SIZE]) {
        out.fill(0);
        if lba < 0 || lba >= self.sectors {
            return;
        }
        let mut f = self.file.lock().unwrap();
        if f.seek(SeekFrom::Start(lba as u64 * SECTOR_SIZE as u64))
            .is_ok()
        {
            let _ = f.read_exact(out);
        }
    }
    pub fn track_at(&self, lba: i32) -> Option<&Track> {
        if lba >= self.sectors {
            return None;
        }
        let mut t = None;
        for (i, x) in self.tracks.iter().enumerate() {
            if i == 0 || lba >= x.index0 {
                t = Some(x)
            }
        }
        t
    }
    pub fn subq(&self, lba: i32) -> [u8; 12] {
        let mut q = [0; 12];
        if let Some(t) = self.track_at(lba) {
            q[0] = if t.audio { 1 } else { 0x41 };
            q[1] = bcd(t.number);
            q[2] = if lba < t.index1 { 0 } else { 1 };
            q[3..6].copy_from_slice(&msf((lba - t.index1).abs()));
        } else {
            q[0] = 0x41;
            q[1] = 0xaa;
            q[2] = 1;
            q[3..6].copy_from_slice(&msf(lba - self.sectors));
        }
        q[7..10].copy_from_slice(&msf(lba + 150));
        let mut crc = 0u16;
        for &b in &q[..10] {
            crc ^= (b as u16) << 8;
            for _ in 0..8 {
                crc = if crc & 0x8000 != 0 {
                    (crc << 1) ^ 0x1021
                } else {
                    crc << 1
                }
            }
        }
        let crc = !crc;
        q[10] = (crc >> 8) as u8;
        q[11] = crc as u8;
        q
    }
    fn user_data(&self, lba: u32) -> [u8; 2048] {
        let mut raw = [0; SECTOR_SIZE];
        self.read_sector(lba as i32, &mut raw);
        raw[24..2072].try_into().unwrap()
    }
    fn extent(&self, r: &[u8]) -> Result<(u32, u32), String> {
        if r.len() < 34 {
            return Err(err("Invalid ISO extent"));
        }
        let l = u32::from_le_bytes(r[2..6].try_into().unwrap());
        let s = u32::from_le_bytes(r[10..14].try_into().unwrap());
        if l != u32::from_be_bytes(r[6..10].try_into().unwrap())
            || s != u32::from_be_bytes(r[14..18].try_into().unwrap())
        {
            return Err(err("Inconsistent ISO extent"));
        }
        if l as u64 + (s as u64).div_ceil(2048) > self.sectors as u64 {
            return Err(err("ISO extent outside image"));
        }
        Ok((l, s))
    }
    pub fn read_file(&self, name: &str) -> Result<Vec<u8>, String> {
        self.read_file_limit(name, u32::MAX)
    }
    pub fn file_extent(&self, name: &str) -> Result<(u32, u32), String> {
        let name = name.replace('\\', "/");
        let parts: Vec<_> = name
            .split('/')
            .map(|p| p.split(';').next().unwrap())
            .collect();
        if parts.len() > 16 || parts.iter().any(|s| ["", ".", ".."].contains(s)) {
            return Err(err("Invalid ISO path"));
        }
        let pvd = self.user_data(16);
        if pvd[0] != 1 || &pvd[1..6] != b"CD001" || pvd[6] != 1 {
            return Err(err("Not ISO9660"));
        }
        let (mut lba, mut size) = self.extent(&pvd[156..190])?;
        for (level, part) in parts.iter().enumerate() {
            if size > 16 << 20 {
                return Err(err("ISO directory too large"));
            }
            let mut found = None;
            for off in (0..size).step_by(2048) {
                let sec = self.user_data(lba + off / 2048);
                let end = 2048.min((size - off) as usize);
                let mut pos = 0;
                while pos < end {
                    let n = sec[pos] as usize;
                    if n == 0 {
                        break;
                    }
                    if n < 34 || n > end - pos {
                        return Err(err("Invalid ISO directory record"));
                    }
                    let rec = &sec[pos..pos + n];
                    let nl = rec[32] as usize;
                    if 33 + nl > n {
                        return Err(err("Invalid ISO filename length"));
                    }
                    let fname = String::from_utf8_lossy(&rec[33..33 + nl]);
                    let fname = fname.split(';').next().unwrap();
                    if fname.eq_ignore_ascii_case(part) {
                        if rec[25] & 0x80 != 0 {
                            return Err(err("Multi-extent ISO unsupported"));
                        }
                        if level < parts.len() - 1 && rec[25] & 2 == 0 {
                            return Err(err("ISO entry not found"));
                        }
                        found = Some(self.extent(rec)?);
                        break;
                    }
                    pos += n;
                }
                if found.is_some() {
                    break;
                }
            }
            (lba, size) = found.ok_or("ISO entry not found")?;
        }
        Ok((lba, size))
    }
    pub fn read_file_limit(&self, name: &str, max: u32) -> Result<Vec<u8>, String> {
        let (lba, size) = self.file_extent(name)?;
        if size > max || size as u64 > usize::MAX as u64 {
            return Err(err("ISO file exceeds read limit"));
        }
        let mut out = vec![0; size as usize];
        for off in (0..size).step_by(2048) {
            let n = 2048.min((size - off) as usize);
            out[off as usize..off as usize + n]
                .copy_from_slice(&self.user_data(lba + off / 2048)[..n]);
        }
        Ok(out)
    }
}
