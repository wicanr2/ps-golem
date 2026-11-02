//! Direct source-to-HD scene+actor connection, spec026 §16.7.20.
//! GPL-3.0-or-later. No GP0 packets, native framebuffer or CPU execution.
use super::{actor::ActorPlan, actorview, scene};
use crate::texture_audit;
use psgolem_core::{
    paint::{self, Draw, Point},
    scene::{Canvas, Image, Material, MaterialKey, Scene},
};
use sha2::{Digest, Sha256};
use std::{collections::HashMap, path::Path};
type Result<T> = std::result::Result<T, String>;
fn span(r: &[u8], a: u32, n: usize) -> Result<usize> {
    let p = (a & 0x1fffffff) as usize;
    if p >= r.len() || n > r.len() - p {
        Err("HD來源越界".into())
    } else {
        Ok(p)
    }
}
fn half(r: &[u8], a: u32) -> Result<u16> {
    let p = span(r, a, 2)?;
    Ok(u16::from_le_bytes(r[p..p + 2].try_into().unwrap()))
}
fn word(r: &[u8], a: u32) -> Result<u32> {
    let p = span(r, a, 4)?;
    Ok(u32::from_le_bytes(r[p..p + 4].try_into().unwrap()))
}
pub struct View {
    pub background: Scene,
    pub materials: Vec<Material>,
    pub draws: Vec<Draw>,
    pub images: Vec<Option<Image>>,
    pub background_images: Vec<Image>,
    pub hd_actor_quads: usize,
    pub native_actor_quads: usize,
    pub actor_entities: Vec<usize>,
}
impl View {
    pub fn prepare(
        r: &[u8],
        vram: &[u16],
        graphics: &[u8],
        actors: &ActorPlan,
        pack: &Path,
    ) -> Result<Self> {
        Self::prepare_at(r, vram, graphics, actors, pack, scene::Viewport::default())
    }
    pub fn prepare_at(
        r: &[u8],
        vram: &[u16],
        graphics: &[u8],
        actors: &ActorPlan,
        pack: &Path,
        viewport: scene::Viewport,
    ) -> Result<Self> {
        let (left, top) = viewport.origin();
        let (width, height) = viewport.size();
        let background = scene::build_area(r, vram, graphics, left, top, width, height)?;
        let actorview::ActorView {
            materials,
            draws,
            images,
            hd_actor_quads,
            native_actor_quads,
            actor_entities,
            ..
        } = actorview::ActorView::prepare_at(r, vram, actors, pack, left, top)?;

        let background_images = texture_audit::load_scene_images(
            pack,
            &background
                .materials
                .iter()
                .map(|m| m.key)
                .collect::<Vec<_>>(),
        )?;

        Ok(Self {
            background,
            materials,
            draws,
            images,
            background_images,
            hd_actor_quads,
            native_actor_quads,
            actor_entities,
        })
    }
    pub fn render(&self, scale: usize) -> Result<Canvas> {
        let mut canvas = self.background.render(
            scale,
            if scale == 4 {
                Some(&self.background_images)
            } else {
                None
            },
        )?;
        paint::draw(
            &mut canvas,
            scale,
            &self.materials,
            &self.images,
            &self.draws,
        )?;
        Ok(canvas)
    }
}
// Read-only saved-source diagnostic, 026 §16.7.20. Never initializes a Machine.
pub fn render_file(path: &std::path::Path) -> Result<(Canvas, Canvas, serde_json::Value)> {
    render_file_at(path, scene::Viewport::default(), true)
}
pub fn render_file_at(
    path: &std::path::Path,
    viewport: scene::Viewport,
    write_outputs: bool,
) -> Result<(Canvas, Canvas, serde_json::Value)> {
    use serde_json::{json, Value};
    use std::{fs, io::Read, path::PathBuf};
    let mut b = Vec::new();
    fs::File::open(path)
        .map_err(|e| e.to_string())?
        .take(16385)
        .read_to_end(&mut b)
        .map_err(|e| e.to_string())?;
    if b.len() > 16384 {
        return Err("預覽描述超過16KiB".into());
    }
    let v: Value = serde_json::from_slice(&b).map_err(|e| e.to_string())?;
    let obj = v.as_object().ok_or("預覽描述須為物件")?;
    let has_hud = obj.contains_key("hud_source");
    let allowed = [
        "version", "ram", "vram", "scratch", "graphics", "textures", "output", "sha256",
    ];
    if obj.len() != allowed.len() + usize::from(has_hud)
        || obj
            .keys()
            .any(|k| !allowed.contains(&k.as_str()) && k != "hud_source")
        || v["version"] != 1
    {
        return Err("預覽描述欄位或版本不符".into());
    }
    let root = path.parent().unwrap_or(std::path::Path::new("."));
    let get = |key: &str| -> Result<PathBuf> {
        let p = PathBuf::from(v[key].as_str().ok_or("預覽路徑須為字串")?);
        Ok(if p.is_absolute() { p } else { root.join(p) })
    };
    let hashes = v["sha256"].as_object().ok_or("預覽來源需要sha256")?;
    if hashes.len() != 5 + usize::from(has_hud)
        || hashes.keys().any(|k| {
            !["ram", "vram", "scratch", "graphics", "manifest"].contains(&k.as_str())
                && !(has_hud && k == "hud_source")
        })
    {
        return Err("預覽來源SHA欄位不符".into());
    }
    let mut loaded = HashMap::new();
    for (key, len) in [
        ("ram", 2 << 20),
        ("vram", 1024 * 512 * 2),
        ("scratch", 1024),
        ("graphics", 0x40000),
    ] {
        let mut bytes = Vec::new();
        fs::File::open(get(key)?)
            .map_err(|e| e.to_string())?
            .take(len as u64 + 1)
            .read_to_end(&mut bytes)
            .map_err(|e| e.to_string())?;
        let digest = format!("{:x}", Sha256::digest(&bytes));
        if bytes.len() != len || hashes[key].as_str() != Some(digest.as_str()) {
            return Err(format!("預覽來源大小或SHA不符：{key}"));
        }
        loaded.insert(key, bytes);
    }
    let pack = get("textures")?;
    let manifest = pack.join("manifest.json");
    let metadata = fs::metadata(&manifest).map_err(|e| e.to_string())?;
    if metadata.len() > 4 << 20 {
        return Err("預覽manifest過大".into());
    }
    let digest = format!(
        "{:x}",
        Sha256::digest(fs::read(&manifest).map_err(|e| e.to_string())?)
    );
    if hashes["manifest"].as_str() != Some(digest.as_str()) {
        return Err("預覽manifest SHA不符".into());
    }
    let vram: Vec<_> = loaded["vram"]
        .chunks_exact(2)
        .map(|b| u16::from_le_bytes([b[0], b[1]]))
        .collect();
    let actor = ActorPlan::prepare(&loaded["ram"], &loaded["scratch"])?;
    let view = View::prepare_at(
        &loaded["ram"],
        &vram,
        &loaded["graphics"],
        &actor,
        &pack,
        viewport,
    )?;
    let hud = if has_hud {
        let mut bytes = Vec::new();
        fs::File::open(get("hud_source")?)
            .map_err(|e| e.to_string())?
            .take(270337)
            .read_to_end(&mut bytes)
            .map_err(|e| e.to_string())?;
        let digest = format!("{:x}", Sha256::digest(&bytes));
        if hashes["hud_source"].as_str() != Some(digest.as_str()) {
            return Err("HUD來源SHA不符".into());
        }
        let source = super::hud::Hud::source(&bytes)?;
        if actor.quads.iter().any(|q| q.order >= 496) {
            return Err("HUD與人物跨層順序尚未支援".into());
        }
        Some(super::hud::Hud::prepare(
            &loaded["ram"],
            &vram,
            source,
            &pack,
        )?)
    } else {
        None
    };
    let mut native = view.render(1)?;
    let mut hd = view.render(4)?;
    if let Some(hud) = &hud {
        hud.paint(&mut native, 1)?;
        hud.paint(&mut hd, 4)?;
    }
    let prefix = get("output")?;
    if !prefix.parent().is_some_and(|p| p.is_dir()) {
        return Err("預覽輸出目錄不存在".into());
    }
    if write_outputs {
        for (suffix, c) in [("native.png", &native), ("HD.png", &hd)] {
            let mut rgba = c.rgba();
            for p in rgba.chunks_exact_mut(4) {
                p[3] = 255;
            }
            let mut e = png::Encoder::new(
                fs::File::create(prefix.with_extension(suffix)).map_err(|e| e.to_string())?,
                c.width as u32,
                c.height as u32,
            );
            e.set_color(png::ColorType::Rgba);
            e.set_depth(png::BitDepth::Eight);
            e.write_header()
                .map_err(|e| e.to_string())?
                .write_image_data(&rgba)
                .map_err(|e| e.to_string())?;
        }
    }
    let report = json!({"viewport":{"width":viewport.size().0,"height":viewport.size().1,"left":viewport.origin().0,"top":viewport.origin().1,"background_tiles":view.background.sprites.len(),"actor_world_scale":1,"resident_layer_data_only":true,"new_actor_generation":false,"HUD_anchor":"screen top-left"},"scope":"saved-source direct HD preview; not gameplay","native_width":native.width,"native_height":native.height,"HD_width":hd.width,"HD_height":hd.height,"background_HD_materials":view.background_images.len(),"typed_actor_quads":view.draws.len(),"HD_actor_quads":view.hd_actor_quads,"native_actor_fallback_quads":view.native_actor_quads,"original_CPU_executed":false,"GP0_submitted":false,"native_framebuffer_input":false,"HUD_text_effects_complete":false,"HD_HUD_quads":hud.as_ref().map(|h|h.draws.len()).unwrap_or(0),"HUD_clear_texels":hud.as_ref().map(|h|h.clear_texels).unwrap_or(0),"deferred_environment_records":hud.as_ref().map(|h|h.deferred_records),"full_HD_complete":false,"speed_pass":false});
    if write_outputs {
        fs::write(
            prefix.with_extension("json"),
            serde_json::to_vec_pretty(&report).map_err(|e| e.to_string())?,
        )
        .map_err(|e| e.to_string())?;
    }
    Ok((native, hd, report))
}
