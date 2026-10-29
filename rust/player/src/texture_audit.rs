//! 026 §15: bounded, read-only image audit. This is not a renderable pack.
//! Format/quantization: ps-golem render/hd/texture_pack.go at
//! 4778ac7d88e3873ccc5ead50ebcd8cf53a621854 (GPL-3.0-or-later).
use serde::{
    de::{self, MapAccess, SeqAccess, Visitor},
    Deserialize, Deserializer,
};
use serde_json::{json, Map, Value};
use sha2::{Digest, Sha256};
use std::{
    collections::{HashMap, HashSet},
    fmt, fs,
    io::{BufReader, Read},
    path::{Path, PathBuf},
};
type Result<T> = std::result::Result<T, String>;
const MANIFEST_LIMIT: u64 = 4 << 20;
const FILE_LIMIT: u64 = 64 << 20;
const PIXELS: u64 = 16 << 20;

// Value alone silently overwrites duplicate object members. Reject them during
// parsing, before schema checks, and reject null even in forbidden fields.
struct Strict(Value);
impl<'de> Deserialize<'de> for Strict {
    fn deserialize<D: Deserializer<'de>>(d: D) -> std::result::Result<Self, D::Error> {
        struct V;
        impl<'de> Visitor<'de> for V {
            type Value = Strict;
            fn expecting(&self, f: &mut fmt::Formatter) -> fmt::Result {
                f.write_str("non-null JSON without duplicate members")
            }
            fn visit_bool<E: de::Error>(self, v: bool) -> std::result::Result<Strict, E> {
                Ok(Strict(v.into()))
            }
            fn visit_i64<E: de::Error>(self, v: i64) -> std::result::Result<Strict, E> {
                Ok(Strict(v.into()))
            }
            fn visit_u64<E: de::Error>(self, v: u64) -> std::result::Result<Strict, E> {
                Ok(Strict(v.into()))
            }
            fn visit_f64<E: de::Error>(self, v: f64) -> std::result::Result<Strict, E> {
                Ok(Strict(v.into()))
            }
            fn visit_str<E: de::Error>(self, v: &str) -> std::result::Result<Strict, E> {
                Ok(Strict(v.into()))
            }
            fn visit_string<E: de::Error>(self, v: String) -> std::result::Result<Strict, E> {
                Ok(Strict(v.into()))
            }
            fn visit_seq<A: SeqAccess<'de>>(
                self,
                mut a: A,
            ) -> std::result::Result<Strict, A::Error> {
                let mut v = Vec::new();
                while let Some(Strict(x)) = a.next_element()? {
                    v.push(x);
                }
                Ok(Strict(v.into()))
            }
            fn visit_map<A: MapAccess<'de>>(
                self,
                mut a: A,
            ) -> std::result::Result<Strict, A::Error> {
                let mut v = Map::new();
                while let Some(k) = a.next_key::<String>()? {
                    if v.contains_key(&k) {
                        return Err(de::Error::custom(format!("重複 JSON 欄位 {k}")));
                    }
                    let Strict(x) = a.next_value()?;
                    v.insert(k, x);
                }
                Ok(Strict(v.into()))
            }
        }
        d.deserialize_any(V)
    }
}
fn object<'a>(v: &'a Value, allowed: &[&str]) -> Result<&'a Map<String, Value>> {
    let m = v.as_object().ok_or("需要 JSON 物件")?;
    for k in m.keys() {
        if !allowed.contains(&k.as_str()) {
            return Err(format!("未知欄位 {k}"));
        }
    }
    Ok(m)
}
fn string<'a>(m: &'a Map<String, Value>, k: &str) -> Result<&'a str> {
    m.get(k)
        .and_then(Value::as_str)
        .ok_or_else(|| format!("{k} 須為字串"))
}
fn number(m: &Map<String, Value>, k: &str) -> Result<u64> {
    m.get(k)
        .and_then(Value::as_u64)
        .ok_or_else(|| format!("{k} 須為非負整數"))
}
fn digest(s: &str) -> Result<()> {
    if s.len() != 64
        || !s
            .bytes()
            .all(|c| c.is_ascii_digit() || (b'a'..=b'f').contains(&c))
    {
        return Err("SHA-256 須為 64 位小寫十六進位".into());
    }
    Ok(())
}
fn file(root: &Path, name: &str, limit: u64) -> Result<PathBuf> {
    if name.is_empty()
        || name.contains(['\\', ':'])
        || name
            .split('/')
            .any(|s| s.is_empty() || s == "." || s == "..")
    {
        return Err(format!("不合法的包內相對路徑 {name:?}"));
    }
    let p = root
        .join(name)
        .canonicalize()
        .map_err(|e| format!("{name}: {e}"))?;
    if !p.starts_with(root) {
        return Err(format!("{name} 指向包外"));
    }
    let m = p.metadata().map_err(|e| e.to_string())?;
    if !m.is_file() || m.len() > limit {
        return Err(format!("{name} 非一般檔案或超過 {limit} bytes"));
    }
    Ok(p)
}
struct Png {
    data: Vec<u8>,
    color: png::ColorType,
    depth: png::BitDepth,
}
fn png_read(root: &Path, name: &str, w: u32, h: u32, indexed: bool) -> Result<Png> {
    let path = file(root, name, FILE_LIMIT)?;
    let mut d = png::Decoder::new(BufReader::new(
        fs::File::open(path).map_err(|e| e.to_string())?,
    ));
    // The single-image ceiling is 16 Mi pixels, at most 128 MiB for RGBA16.
    d.set_limits(png::Limits { bytes: 128 << 20 });
    if !indexed {
        d.set_transformations(png::Transformations::EXPAND);
    }
    let mut r = d.read_info().map_err(|e| format!("{name}: {e}"))?;
    if (r.info().width, r.info().height) != (w, h) {
        return Err(format!("{name} 尺寸不符，需要 {w}×{h}"));
    }
    if indexed && r.info().color_type != png::ColorType::Indexed {
        return Err(format!("{name} 需要索引 PNG"));
    }
    if indexed && r.info().palette.as_ref().is_none_or(|p| p.is_empty()) {
        return Err(format!("{name} 缺少調色盤"));
    }
    let palette_len = r.info().palette.as_ref().map_or(0, |p| p.len() / 3);
    let mut data = vec![0; r.output_buffer_size()];
    let o = r
        .next_frame(&mut data)
        .map_err(|e| format!("{name}: {e}"))?;
    data.truncate(o.buffer_size());
    // Consume remaining chunks as well: late CRC/truncation is not success.
    r.finish().map_err(|e| format!("{name}: {e}"))?;
    if indexed {
        let bits = o.bit_depth as usize;
        if ![1, 2, 4, 8].contains(&bits) {
            return Err(format!("{name} 索引位深錯誤"));
        }
        let row = (w as usize * bits).div_ceil(8);
        let mask = (1usize << bits) - 1;
        let mut unpacked = Vec::with_capacity(w as usize * h as usize);
        for y in 0..h as usize {
            for x in 0..w as usize {
                let bit = x * bits;
                let v = (data[y * row + bit / 8] as usize >> (8 - bits - bit % 8)) & mask;
                if v >= palette_len {
                    return Err(format!("{name} 索引超出 PNG 調色盤"));
                }
                unpacked.push(v as u8);
            }
        }
        return Ok(Png {
            data: unpacked,
            color: o.color_type,
            depth: o.bit_depth,
        });
    }
    if ![png::BitDepth::Eight, png::BitDepth::Sixteen].contains(&o.bit_depth) {
        return Err(format!("{name} 未能解碼為 8／16 位通道"));
    }
    Ok(Png {
        data,
        color: o.color_type,
        depth: o.bit_depth,
    })
}
impl Png {
    fn rgba(&self, i: usize) -> [u8; 4] {
        if self.depth == png::BitDepth::Sixteen {
            // 019 specifies Go NRGBAModel conversion, including its integer
            // round trip for NRGBA64 with non-full alpha. Reference: Go 1.26.7,
            // src/image/color/color.go, nrgbaModel/NRGBA64.RGBA, BSD-3-Clause.
            let channels = match self.color {
                png::ColorType::Rgba => 4,
                png::ColorType::Rgb => 3,
                png::ColorType::GrayscaleAlpha => 2,
                png::ColorType::Grayscale => 1,
                png::ColorType::Indexed => unreachable!(),
            };
            let at = i * channels * 2;
            let get =
                |c: usize| u16::from_be_bytes([self.data[at + c * 2], self.data[at + c * 2 + 1]]);
            let (r, g, b, a) = match self.color {
                png::ColorType::Rgba => (get(0), get(1), get(2), get(3)),
                png::ColorType::Rgb => (get(0), get(1), get(2), 65535),
                png::ColorType::GrayscaleAlpha => (get(0), get(0), get(0), get(1)),
                png::ColorType::Grayscale => (get(0), get(0), get(0), 65535),
                png::ColorType::Indexed => unreachable!(),
            };
            let channel = |v: u16| -> u8 {
                if a == 0 {
                    0
                } else if a == 65535 {
                    (v >> 8) as u8
                } else {
                    (((u32::from(v) * u32::from(a) / 65535) * 65535 / u32::from(a)) >> 8) as u8
                }
            };
            return [channel(r), channel(g), channel(b), (a >> 8) as u8];
        }
        match self.color {
            png::ColorType::Rgba => self.data[i * 4..i * 4 + 4].try_into().unwrap(),
            png::ColorType::Rgb => [
                self.data[i * 3],
                self.data[i * 3 + 1],
                self.data[i * 3 + 2],
                255,
            ],
            png::ColorType::Grayscale => [self.data[i], self.data[i], self.data[i], 255],
            png::ColorType::GrayscaleAlpha => [
                self.data[i * 2],
                self.data[i * 2],
                self.data[i * 2],
                self.data[i * 2 + 1],
            ],
            png::ColorType::Indexed => unreachable!("indexed must be expanded for RGBA"),
        }
    }
}
fn color_word(c: [u8; 4], m: [u8; 4]) -> Result<u16> {
    if m[3] != 255 || m[0] != m[1] || m[0] != m[2] || ![0, 255].contains(&m[0]) {
        return Err("STP 須為不透明純黑／純白".into());
    }
    if ![0, 255].contains(&c[3]) {
        return Err("alpha 須為 0 或 255".into());
    }
    if c[3] == 0 {
        return Ok(0);
    }
    let v = u16::from(c[0] >> 3)
        | (u16::from(c[1] >> 3) << 5)
        | (u16::from(c[2] >> 3) << 10)
        | (u16::from(m[0] == 255) << 15);
    if v == 0 {
        return Err("不能表達 RGB5=0、STP=0 的不透明黑".into());
    }
    Ok(v)
}
pub fn run(directory: &Path) -> Result<Value> {
    let root = directory.canonicalize().map_err(|e| e.to_string())?;
    if !root.is_dir() {
        return Err("素材根路徑須為目錄".into());
    }
    let p = file(&root, "manifest.json", MANIFEST_LIMIT)?;
    let mut b = Vec::new();
    fs::File::open(p)
        .map_err(|e| e.to_string())?
        .take(MANIFEST_LIMIT + 1)
        .read_to_end(&mut b)
        .map_err(|e| e.to_string())?;
    if b.len() as u64 > MANIFEST_LIMIT {
        return Err("manifest 超過 4 MiB".into());
    }
    let Strict(v) = serde_json::from_slice::<Strict>(&b).map_err(|e| e.to_string())?;
    let m = object(&v, &["version", "entries"])?;
    if number(m, "version")? != 1 {
        return Err("只接受 manifest v1".into());
    }
    let es = m
        .get("entries")
        .and_then(Value::as_array)
        .ok_or("entries 須為陣列")?;
    if es.len() > 8192 {
        return Err("entries 超過 8192".into());
    }
    let mut keys = HashSet::new();
    let mut sizes = HashMap::new();
    let mut entries = Vec::new();
    let (mut pixels, mut bytes) = (0u64, 0u64);
    for (i, v) in es.iter().enumerate() {
        let row=(|| -> Result<Value> {
            let s=object(v,&["upload_sha256","width_words","height","depth","scale","kind","clut_sha256","image","stp"])?;
            let upload=string(s,"upload_sha256")?; digest(upload)?;
            let (ww,hh,depth,scale)=(number(s,"width_words")?,number(s,"height")?,number(s,"depth")?,number(s,"scale")?);
            if !(1..=1024).contains(&ww) || !(1..=512).contains(&hh) || ![4,8,15].contains(&depth) || ![1,2,4,8].contains(&scale) { return Err("原尺寸／depth／scale 超界".into()); }
            let kind=string(s,"kind")?; let indexed=kind=="indexed";
            if !indexed && kind!="color" { return Err("kind 須為 indexed 或 color".into()); }
            let clut=if indexed || depth==15 {
                if s.contains_key("clut_sha256") || (indexed && (s.contains_key("stp") || depth==15)) { return Err("禁止的 CLUT／STP／depth 組合".into()); } ""
            } else { let c=string(s,"clut_sha256")?; digest(c)?; c };
            if !keys.insert((upload.to_owned(),depth,kind.to_owned(),clut.to_owned())) { return Err("重複素材鍵".into()); }
            if sizes.insert(upload.to_owned(),(ww,hh)).is_some_and(|old|old!=(ww,hh)) { return Err("同上傳鍵尺寸不同".into()); }
            let (w,h)=(ww*(if depth==4 {4} else if depth==8 {2} else {1})*scale,hh*scale);
            if w*h>PIXELS { return Err("PNG 超過 16 Mi 貼素".into()); }
            let name=string(s,"image")?;
            let image=png_read(&root,name,w as u32,h as u32,indexed)?;
            let mut hash=Sha256::new();
            if indexed {
                debug_assert_ne!(image.depth,png::BitDepth::Sixteen);
                if image.data.iter().any(|&v|u64::from(v)>=(1u64<<depth)) { return Err(format!("{name} 索引超過 {depth}bpp")); }
                hash.update(&image.data);
            } else {
                let stp_name=string(s,"stp")?;
                let stp=png_read(&root,stp_name,w as u32,h as u32,false)?;
                // Batch hashing avoids a SHA call for every two-byte word.
                let mut words=Vec::with_capacity(w as usize*h as usize*2);
                for n in 0..(w*h) as usize {
                    let word=color_word(image.rgba(n),stp.rgba(n)).map_err(|e|format!("{name} / {stp_name} ({},{}): {e}",n%w as usize,n/w as usize))?;
                    words.extend_from_slice(&word.to_le_bytes());
                }
                hash.update(words);
            }
            pixels+=w*h; bytes+=w*h*(if indexed {1} else {2});
            Ok(json!({"upload_sha256":upload,"depth":depth,"kind":kind,"clut_sha256":clut,"width":w,"height":h,"scale":scale,"decoded_sha256":format!("{:x}",hash.finalize())}))
        })().map_err(|e|format!("entries[{i}]: {e}"))?;
        entries.push(row);
    }
    Ok(
        json!({"scope":"019 image data only; not a renderable pack","manifest_sha256":format!("{:x}",Sha256::digest(&b)),"entry_count":entries.len(),"upload_keys":sizes.len(),"decoded_pixels":pixels,"decoded_bytes_total_not_resident":bytes,"sprite_runs_present":root.join("sprite-runs.json").try_exists().map_err(|e|e.to_string())?,"sprite_runs_validated":false,"gameplay_HD_enabled":false,"entries":entries}),
    )
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::atomic::{AtomicU64, Ordering};
    static N: AtomicU64 = AtomicU64::new(0);
    struct Dir(PathBuf);
    impl Dir {
        fn new() -> Self {
            let p = std::env::temp_dir().join(format!(
                "psgolem-texture-audit-{}-{}",
                std::process::id(),
                N.fetch_add(1, Ordering::Relaxed)
            ));
            fs::create_dir(&p).unwrap();
            Self(p)
        }
        fn manifest(&self, v: &Value) {
            fs::write(self.0.join("manifest.json"), serde_json::to_vec(v).unwrap()).unwrap();
        }
        fn png(
            &self,
            name: &str,
            c: png::ColorType,
            d: png::BitDepth,
            w: u32,
            h: u32,
            data: &[u8],
        ) {
            let mut e = png::Encoder::new(fs::File::create(self.0.join(name)).unwrap(), w, h);
            e.set_color(c);
            e.set_depth(d);
            if c == png::ColorType::Indexed {
                e.set_palette(vec![80; (1usize << d as usize) * 3]);
            }
            e.write_header().unwrap().write_image_data(data).unwrap();
        }
    }
    impl Drop for Dir {
        fn drop(&mut self) {
            let _ = fs::remove_dir_all(&self.0);
        }
    }
    fn spec() -> Value {
        json!({"upload_sha256":"0".repeat(64),"width_words":1,"height":1,"depth":4,"scale":1,"kind":"color","clut_sha256":"1".repeat(64),"image":"i.png","stp":"s.png"})
    }
    fn manifest(s: Value) -> Value {
        json!({"version":1,"entries":[s]})
    }
    fn valid(d: &Dir) {
        d.png(
            "i.png",
            png::ColorType::Rgba,
            png::BitDepth::Eight,
            4,
            1,
            &[248, 0, 0, 255, 0, 0, 0, 0, 0, 248, 0, 255, 0, 0, 0, 255],
        );
        d.png(
            "s.png",
            png::ColorType::Grayscale,
            png::BitDepth::Eight,
            4,
            1,
            &[0, 255, 255, 255],
        );
        d.manifest(&manifest(spec()));
    }
    #[test]
    fn rgba_and_stp_words() {
        let d = Dir::new();
        valid(&d);
        let r = run(&d.0).unwrap();
        let raw: Vec<u8> = [31u16, 0, 0x83e0, 0x8000]
            .iter()
            .flat_map(|v| v.to_le_bytes())
            .collect();
        assert_eq!(
            r["entries"][0]["decoded_sha256"],
            format!("{:x}", Sha256::digest(raw))
        );
        assert_eq!(r["decoded_bytes_total_not_resident"], 8);
        assert_eq!(r["gameplay_HD_enabled"], false);
    }
    #[test]
    fn indexed_preserves_equal_color_indices_all_bit_depths() {
        for (depth, data, expected) in [
            (png::BitDepth::One, vec![0b01010000], vec![0, 1, 0, 1]),
            (png::BitDepth::Two, vec![0b00011011], vec![0, 1, 2, 3]),
            (png::BitDepth::Four, vec![0x01, 0x23], vec![0, 1, 2, 3]),
            (png::BitDepth::Eight, vec![0, 1, 2, 15], vec![0, 1, 2, 15]),
        ] {
            let d = Dir::new();
            d.png("i.png", png::ColorType::Indexed, depth, 4, 1, &data);
            let mut s = spec();
            s["kind"] = "indexed".into();
            s.as_object_mut().unwrap().remove("clut_sha256");
            s.as_object_mut().unwrap().remove("stp");
            d.manifest(&manifest(s));
            assert_eq!(
                run(&d.0).unwrap()["entries"][0]["decoded_sha256"],
                format!("{:x}", Sha256::digest(expected))
            );
        }
    }
    #[test]
    fn rgb_and_16bit_and_grayscale_alpha() {
        let d = Dir::new();
        valid(&d);
        d.png(
            "i.png",
            png::ColorType::Rgba,
            png::BitDepth::Sixteen,
            4,
            1,
            &[
                248, 1, 0, 0, 0, 0, 255, 255, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 248, 1, 0, 0, 255, 255,
                0, 0, 0, 0, 0, 0, 255, 255,
            ],
        );
        assert!(run(&d.0).is_ok());
        // Go NRGBAModel turns 0x0800/0xF800 at alpha 0xFF00 into
        // 7/247 before RGB555 quantization; stripping 16-bit channels is wrong.
        d.png(
            "s.png",
            png::ColorType::Grayscale,
            png::BitDepth::Eight,
            4,
            1,
            &[255; 4],
        );
        d.png(
            "i.png",
            png::ColorType::Rgba,
            png::BitDepth::Sixteen,
            4,
            1,
            &[
                8, 0, 0, 0, 0, 0, 255, 0, 248, 0, 0, 0, 0, 0, 255, 0, 0, 0, 248, 0, 0, 0, 255, 255,
                0, 0, 0, 0, 0, 0, 0, 255,
            ],
        );
        let raw: Vec<u8> = [0x8000u16, 0x801e, 0x83e0, 0]
            .iter()
            .flat_map(|v| v.to_le_bytes())
            .collect();
        assert_eq!(
            run(&d.0).unwrap()["entries"][0]["decoded_sha256"],
            format!("{:x}", Sha256::digest(raw))
        );
        d.png(
            "i.png",
            png::ColorType::Rgb,
            png::BitDepth::Eight,
            4,
            1,
            &[248; 12],
        );
        assert!(run(&d.0).is_ok());
        d.png(
            "i.png",
            png::ColorType::GrayscaleAlpha,
            png::BitDepth::Eight,
            4,
            1,
            &[248, 255, 248, 255, 248, 255, 248, 255],
        );
        assert!(run(&d.0).is_ok());
    }
    #[test]
    fn transparency_failures_are_not_approximated() {
        assert!(color_word([0, 0, 0, 255], [0, 0, 0, 255]).is_err());
        assert!(color_word([255, 0, 0, 128], [0, 0, 0, 255]).is_err());
        for m in [[128, 128, 128, 255], [255, 0, 0, 255], [0, 0, 0, 0]] {
            assert!(color_word([0, 0, 0, 0], m).is_err());
        }
        let d = Dir::new();
        valid(&d);
        d.png(
            "i.png",
            png::ColorType::Rgba,
            png::BitDepth::Eight,
            4,
            1,
            &[248, 0, 0, 128, 0, 0, 0, 0, 0, 248, 0, 255, 0, 0, 0, 255],
        );
        assert!(run(&d.0).unwrap_err().contains("(0,0)"));
    }
    #[test]
    fn strict_schema_and_late_failure() {
        let d = Dir::new();
        valid(&d);
        for raw in [
            r#"{"version":1,"version":1,"entries":[]}"#,
            r#"{"version":1,"entries":null}"#,
            r#"{"version":1,"Entries":[]}"#,
            r#"{"version":1.0,"entries":[]}"#,
            r#"{"version":1,"entries":[]}{}"#,
        ] {
            fs::write(d.0.join("manifest.json"), raw).unwrap();
            assert!(run(&d.0).is_err());
        }
        let mut s = spec();
        s["kind"] = "indexed".into();
        s["clut_sha256"] = "".into();
        d.manifest(&manifest(s));
        assert!(run(&d.0).is_err());
        let mut s = spec();
        s["upload_sha256"] = "2".repeat(64).into();
        s["image"] = "missing.png".into();
        d.manifest(&json!({"version":1,"entries":[spec(),s]}));
        assert!(run(&d.0).unwrap_err().contains("entries[1]"));
        d.manifest(&json!({"version":1,"entries":[spec(),spec()]}));
        assert!(run(&d.0).unwrap_err().contains("重複素材鍵"));
    }
    #[test]
    fn dimensions_and_index_bounds() {
        let d = Dir::new();
        valid(&d);
        let mut s = spec();
        s["scale"] = 3.into();
        d.manifest(&manifest(s));
        assert!(run(&d.0).is_err());
        valid(&d);
        d.png(
            "i.png",
            png::ColorType::Rgba,
            png::BitDepth::Eight,
            1,
            1,
            &[248, 0, 0, 255],
        );
        assert!(run(&d.0).unwrap_err().contains("尺寸"));
        d.png(
            "i.png",
            png::ColorType::Indexed,
            png::BitDepth::Eight,
            4,
            1,
            &[0, 1, 2, 16],
        );
        let mut s = spec();
        s["kind"] = "indexed".into();
        s.as_object_mut().unwrap().remove("clut_sha256");
        s.as_object_mut().unwrap().remove("stp");
        d.manifest(&manifest(s));
        assert!(run(&d.0).unwrap_err().contains("索引超過"));
    }
    #[test]
    fn canonical_paths_and_sprite_scope() {
        let d = Dir::new();
        valid(&d);
        let root = d.0.canonicalize().unwrap();
        for p in [
            "",
            ".",
            "../i.png",
            "/i.png",
            "a//i.png",
            "a/./i.png",
            "a\\i.png",
            "C:i.png",
        ] {
            assert!(file(&root, p, FILE_LIMIT).is_err());
        }
        fs::write(
            d.0.join("sprite-runs.json"),
            b"not validated by this image-only audit",
        )
        .unwrap();
        let r = run(&d.0).unwrap();
        assert_eq!(r["sprite_runs_present"], true);
        assert_eq!(r["sprite_runs_validated"], false);
        #[cfg(unix)]
        {
            let other = Dir::new();
            fs::write(other.0.join("outside"), b"x").unwrap();
            std::os::unix::fs::symlink(other.0.join("outside"), d.0.join("escape")).unwrap();
            assert!(file(&root, "escape", FILE_LIMIT).is_err());
            std::os::unix::fs::symlink(d.0.join("i.png"), d.0.join("inside")).unwrap();
            assert!(file(&root, "inside", FILE_LIMIT).is_ok());
        }
    }
}
