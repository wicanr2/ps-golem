//! Poster preferences in the existing Go JSON schema, 024 section6.
//! GPL-3.0-or-later. Other settings are merged unchanged at each atomic save.
use serde_json::{json, Map, Value};
use std::{
    env,
    fs::{self, OpenOptions},
    io::{Read, Write},
    path::{Path, PathBuf},
    sync::atomic::{AtomicU64, Ordering},
};
static SERIAL: AtomicU64 = AtomicU64::new(0);
#[derive(Clone, Debug, PartialEq)]
pub struct Choice {
    pub id: String,
    pub soft: bool,
    pub zoom: bool,
}
impl Default for Choice {
    fn default() -> Self {
        Self {
            id: String::new(),
            soft: true,
            zoom: true,
        }
    }
}
pub fn valid_id(s: &str) -> bool {
    !s.is_empty() && s.len() <= 32 && s.bytes().all(|b| b.is_ascii_alphanumeric() || b == b'-')
}
impl Choice {
    pub fn validate(&self) -> Result<(), String> {
        if !self.id.is_empty() && !valid_id(&self.id) {
            Err("無效海報代號".into())
        } else {
            Ok(())
        }
    }
    fn parse(value: Option<&Value>) -> Result<Self, String> {
        let Some(v) = value.filter(|v| !v.is_null()) else {
            return Ok(Self::default());
        };
        let m = v.as_object().ok_or("poster設定須為物件")?;
        let string = |k: &str| -> Result<String, String> {
            match m.get(k) {
                None | Some(Value::Null) => Ok(String::new()),
                Some(Value::String(s)) => Ok(s.clone()),
                _ => Err(format!("poster.{k}須為字串")),
            }
        };
        let boolean = |k: &str| -> Result<bool, String> {
            match m.get(k) {
                None | Some(Value::Null) => Ok(false),
                Some(Value::Bool(b)) => Ok(*b),
                _ => Err(format!("poster.{k}須為布林值")),
            }
        };
        let c = Self {
            id: string("id")?,
            soft: boolean("soft")?,
            zoom: boolean("zoom")?,
        };
        c.validate()?;
        Ok(c)
    }
}
#[derive(Clone)]
pub struct Preferences {
    pub path: PathBuf,
    pub pack: String,
    pub choice: Choice,
    relative: bool,
}
fn read(path: &Path) -> Result<Map<String, Value>, String> {
    let f = match fs::File::open(path) {
        Ok(f) => f,
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => return Ok(Map::new()),
        Err(e) => return Err(e.to_string()),
    };
    let mut bytes = Vec::new();
    f.take(1024 * 1024 + 1)
        .read_to_end(&mut bytes)
        .map_err(|e| e.to_string())?;
    if bytes.len() > 1024 * 1024 {
        return Err("設定檔超過1MiB".into());
    }
    serde_json::from_slice::<Value>(&bytes)
        .map_err(|e| e.to_string())?
        .as_object()
        .cloned()
        .ok_or("設定檔須為JSON物件".into())
}
impl Preferences {
    pub fn load(explicit: Option<PathBuf>) -> Result<Self, String> {
        let (path, relative) = if let Some(p) = explicit {
            (p, true)
        } else {
            let exe = env::current_exe().map_err(|e| e.to_string())?;
            let portable = exe.parent().ok_or("執行檔路徑無效")?.join("settings.json");
            if portable.is_file() {
                (portable, true)
            } else {
                let root = if cfg!(target_os = "windows") {
                    env::var_os("APPDATA").map(PathBuf::from)
                } else if cfg!(target_os = "macos") {
                    env::var_os("HOME")
                        .map(|p| PathBuf::from(p).join("Library/Application Support"))
                } else {
                    env::var_os("XDG_CONFIG_HOME")
                        .map(PathBuf::from)
                        .or_else(|| env::var_os("HOME").map(|p| PathBuf::from(p).join(".config")))
                };
                (
                    root.ok_or("無法決定設定目錄，請指定-settings")?
                        .join("ps-golem/settings.json"),
                    false,
                )
            }
        };
        let path = if path.is_absolute() {
            path
        } else {
            env::current_dir().map_err(|e| e.to_string())?.join(path)
        };
        let m = read(&path)?;
        let choice = Choice::parse(m.get("poster"))?;
        let pack = match m.get("posters") {
            None | Some(Value::Null) => String::new(),
            Some(Value::String(s)) => s.clone(),
            _ => return Err("posters須為字串".into()),
        };
        Ok(Self {
            path,
            pack,
            choice,
            relative,
        })
    }
    pub fn pack_path(&self) -> Option<PathBuf> {
        if self.pack.is_empty() {
            return None;
        }
        let p = PathBuf::from(&self.pack);
        Some(if self.relative && !p.is_absolute() {
            self.path.parent().unwrap().join(p)
        } else {
            p
        })
    }
    pub fn set_pack(&mut self, path: String) -> Result<(), String> {
        if path.is_empty() {
            self.pack.clear();
            return Ok(());
        }
        let p = PathBuf::from(path);
        let p = if p.is_absolute() {
            p
        } else {
            env::current_dir().map_err(|e| e.to_string())?.join(p)
        };
        self.pack = if self.relative {
            p.strip_prefix(self.path.parent().unwrap())
                .unwrap_or(&p)
                .to_string_lossy()
                .into_owned()
        } else {
            p.to_string_lossy().into_owned()
        };
        Ok(())
    }
    pub fn save(&self, next: &Choice) -> Result<(), String> {
        next.validate()?;
        let mut m = read(&self.path)?;
        m.insert("posters".into(), json!(self.pack));
        let v = m.entry("poster").or_insert_with(|| json!({}));
        if !v.is_object() {
            *v = json!({});
        }
        let p = v.as_object_mut().unwrap();
        p.insert("id".into(), json!(next.id));
        p.insert("soft".into(), json!(next.soft));
        p.insert("zoom".into(), json!(next.zoom));
        let mut bytes = serde_json::to_vec_pretty(&m).map_err(|e| e.to_string())?;
        bytes.push(b'\n');
        let parent = self.path.parent().ok_or("設定路徑無效")?;
        fs::create_dir_all(parent).map_err(|e| e.to_string())?;
        let name = self
            .path
            .file_name()
            .ok_or("設定檔名無效")?
            .to_string_lossy();
        let temp = parent.join(format!(
            ".{name}.psgolem-{}-{}.tmp",
            std::process::id(),
            SERIAL.fetch_add(1, Ordering::Relaxed)
        ));
        let result = (|| -> Result<(), String> {
            let mut opts = OpenOptions::new();
            opts.write(true).create_new(true);
            #[cfg(unix)]
            {
                use std::os::unix::fs::OpenOptionsExt;
                opts.mode(0o600);
            }
            let mut f = opts.open(&temp).map_err(|e| e.to_string())?;
            f.write_all(&bytes).map_err(|e| e.to_string())?;
            f.sync_all().map_err(|e| e.to_string())?;
            fs::rename(&temp, &self.path).map_err(|e| e.to_string())?;
            Ok(())
        })();
        if result.is_err() {
            let _ = fs::remove_file(&temp);
        }
        result
    }
}
#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn preserve_other_fields_and_failed_save() {
        let root = env::temp_dir().join(format!(
            "psgolem-preferences-{}-{}",
            std::process::id(),
            SERIAL.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&root).unwrap();
        let file = root.join("settings.json");
        let initial = json!({"hd":4,"bios":"bios/example.bin","textures":"art","gamepad":{"device":"keyboard"},"future":[1,2],"poster":{"id":"a","soft":true,"zoom":true,"futureEffect":77}});
        fs::write(&file, serde_json::to_vec(&initial).unwrap()).unwrap();
        let mut p = Preferences::load(Some(file.clone())).unwrap();
        p.set_pack(root.join("posters").to_string_lossy().into())
            .unwrap();
        assert_eq!(p.pack, "posters");
        let mut next = p.choice.clone();
        next.id = "b".into();
        p.save(&next).unwrap();
        let after = read(&file).unwrap();
        for key in ["hd", "bios", "textures", "gamepad", "future"] {
            assert_eq!(after[key], initial[key]);
        }
        assert_eq!(after["poster"]["futureEffect"], 77);
        assert_eq!(Preferences::load(Some(file.clone())).unwrap().choice, next);
        let saved = fs::read(&file).unwrap();
        fs::rename(&file, root.join("saved.json")).unwrap();
        fs::create_dir(&file).unwrap();
        assert!(p.save(&Choice::default()).is_err());
        assert_eq!(fs::read(root.join("saved.json")).unwrap(), saved);
        assert!(!fs::read_dir(&root).unwrap().any(|e| e
            .unwrap()
            .file_name()
            .to_string_lossy()
            .ends_with(".tmp")));
        fs::remove_dir_all(root).unwrap();
    }
}
