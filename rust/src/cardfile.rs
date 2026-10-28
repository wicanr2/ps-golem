//! Host card file persistence, 010 / 012 / 026 section12. GPL-3.0-or-later.
//! Keeps native 128 KiB images; no game-specific or hardware state changes.
use crate::sio::Memcard;
use std::{
    fs::{self, OpenOptions},
    io::{self, Write},
    path::Path,
    sync::atomic::{AtomicU64, Ordering},
};
static SERIAL: AtomicU64 = AtomicU64::new(0);
pub fn load_or_new(path: &Path) -> io::Result<Memcard> {
    let mut card = Memcard::new();
    match fs::read(path) {
        Ok(bytes) => {
            if !card.load(&bytes) {
                return Err(io::Error::new(
                    io::ErrorKind::InvalidData,
                    "Memory card must contain exactly 131072 bytes",
                ));
            }
        }
        Err(e) if e.kind() == io::ErrorKind::NotFound => {}
        Err(e) => return Err(e),
    }
    Ok(card)
}
pub fn save_if_dirty(path: &Path, card: &mut Memcard) -> io::Result<bool> {
    if !card.dirty {
        return Ok(false);
    }
    let parent = path
        .parent()
        .filter(|p| !p.as_os_str().is_empty())
        .unwrap_or(Path::new("."));
    let name = path.file_name().ok_or_else(|| {
        io::Error::new(
            io::ErrorKind::InvalidInput,
            "Memory card requires a filename",
        )
    })?;
    for _ in 0..16 {
        let mut temp_name = std::ffi::OsString::from(".");
        temp_name.push(name);
        temp_name.push(format!(
            ".psgolem-{}-{}.tmp",
            std::process::id(),
            SERIAL.fetch_add(1, Ordering::Relaxed)
        ));
        let temp = parent.join(temp_name);
        let mut options = OpenOptions::new();
        options.write(true).create_new(true);
        #[cfg(unix)]
        {
            use std::os::unix::fs::OpenOptionsExt;
            options.mode(0o600);
        }
        let mut file = match options.open(&temp) {
            Ok(f) => f,
            Err(e) if e.kind() == io::ErrorKind::AlreadyExists => continue,
            Err(e) => return Err(e),
        };
        let result = (|| {
            file.write_all(&card.data[..])?;
            file.sync_all()?;
            drop(file);
            fs::rename(&temp, path)
        })();
        if let Err(e) = result {
            let _ = fs::remove_file(&temp);
            return Err(e);
        }
        card.dirty = false;
        return Ok(true);
    }
    Err(io::Error::new(
        io::ErrorKind::AlreadyExists,
        "Cannot allocate a private card staging file",
    ))
}
#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn preserve_native_cards_and_failed_atomic_save() {
        let Ok(root) = std::env::var("PSGOLEM_RUST_CARD_TEST_DIR") else {
            eprintln!("SKIP: select a disposable card output directory");
            return;
        };
        let root = Path::new(&root);
        assert!(root.is_dir());
        let dir = root.join("rust-player-card-tests-v1");
        fs::create_dir(&dir).unwrap();
        let path = dir.join("new-card.mcd");
        let mut card = load_or_new(&path).unwrap();
        assert_eq!(&card.data[..2], b"MC");
        assert!(!save_if_dirty(&path, &mut card).unwrap());
        assert!(!path.exists());
        card.data[0x1f80..0x2000].fill(0x71);
        card.dirty = true;
        assert!(save_if_dirty(&path, &mut card).unwrap());
        assert!(!card.dirty);
        assert_eq!(fs::read(&path).unwrap(), &card.data[..]);
        let mut loaded = load_or_new(&path).unwrap();
        assert_eq!(loaded.data, card.data);
        loaded.data[0x1f80] = 0x33;
        loaded.dirty = true;
        assert!(save_if_dirty(&path, &mut loaded).unwrap());
        assert_eq!(load_or_new(&path).unwrap().data[0x1f80], 0x33);
        let bad = dir.join("bad.mcd");
        fs::write(&bad, b"bad-length").unwrap();
        assert!(load_or_new(&bad).is_err());
        assert_eq!(fs::read(&bad).unwrap(), b"bad-length");
        let blocked = dir.join("blocked.mcd");
        fs::create_dir(&blocked).unwrap();
        fs::write(blocked.join("keep"), b"original").unwrap();
        loaded.dirty = true;
        assert!(save_if_dirty(&blocked, &mut loaded).is_err());
        assert!(loaded.dirty);
        assert_eq!(fs::read(blocked.join("keep")).unwrap(), b"original");
        assert!(fs::read_dir(&dir).unwrap().all(|p| !p
            .unwrap()
            .file_name()
            .to_string_lossy()
            .ends_with(".tmp")));
        assert!(save_if_dirty(&dir.join("missing/child.mcd"), &mut loaded).is_err());
        assert!(loaded.dirty);
        eprintln!("RUST_CARD_FILE: original format/load, no-dirty preservation, atomic replace, wrong-length and failed-save preservation/staging cleanup passed");
    }
}
