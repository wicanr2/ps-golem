// 026 §16.4 staged scene components, no normal-player registration.
#[path = "../src/apps/sotn/scene.rs"]
mod sotn;
#[path = "../src/texture_audit.rs"]
mod texture_audit;

#[test]
#[ignore = "requires retained original scene and approved local art; no replacements"]
fn retained_room_direct_rust_hd() {
    use psgolem_core::scene::MaterialKey;
    use serde_json::{json, Value};
    use sha2::{Digest, Sha256};
    use std::{
        fs,
        path::{Path, PathBuf},
    };
    fn read(p: &str) -> Vec<u8> {
        fs::read(p).unwrap()
    }
    fn hex(b: &[u8]) -> String {
        format!("{:x}", Sha256::digest(b))
    }
    fn key(k: MaterialKey) -> Vec<String> {
        [k.upload, k.clut]
            .iter()
            .map(|k| k.iter().map(|b| format!("{b:02x}")).collect())
            .collect()
    }
    fn rgba(p: &Path) -> (usize, usize, Vec<u8>) {
        let mut d = png::Decoder::new(std::io::BufReader::new(fs::File::open(p).unwrap()));
        d.set_transformations(png::Transformations::EXPAND | png::Transformations::STRIP_16);
        let mut r = d.read_info().unwrap();
        let mut b = vec![0; r.output_buffer_size()];
        let f = r.next_frame(&mut b).unwrap();
        r.finish().unwrap();
        b.truncate(f.buffer_size());
        let out = match f.color_type {
            png::ColorType::Rgba => b,
            png::ColorType::Rgb => b
                .chunks_exact(3)
                .flat_map(|p| [p[0], p[1], p[2], 255])
                .collect(),
            _ => panic!("unexpected original PNG"),
        };
        (f.width as usize, f.height as usize, out)
    }
    fn png_out(p: &Path, w: usize, h: usize, b: &[u8]) {
        let mut e = png::Encoder::new(fs::File::create(p).unwrap(), w as u32, h as u32);
        e.set_color(png::ColorType::Rgba);
        e.set_depth(png::BitDepth::Eight);
        e.write_header().unwrap().write_image_data(b).unwrap();
    }
    let root = PathBuf::from(
        std::env::var_os("PSGOLEM_SCENE_FIXTURES").expect("original fixture root required"),
    );
    let evidence: Value =
        serde_json::from_slice(&fs::read(root.join("scene-evidence.json")).unwrap()).unwrap();
    let old = &evidence["old_inputs"];
    for group in ["sources", "image_sources"] {
        for (p, h) in evidence[group].as_object().unwrap() {
            let checked=if p=="/engine/rust/player/src/texture_audit.rs" {root.join("texture-audit-before.rs")} else {PathBuf::from(p)};
            assert_eq!(hex(&fs::read(checked).unwrap()), h.as_str().unwrap());
        }
    }
    let ram = read(old["RAM"].as_str().unwrap());
    let raw_vram = read(old["VRAM"].as_str().unwrap());
    let gfx = read(old["Graphics"].as_str().unwrap());
    let vram: Vec<_> = raw_vram
        .chunks_exact(2)
        .map(|p| u16::from_le_bytes([p[0], p[1]]))
        .collect();
    let reference = evidence["sources"].as_object().unwrap().keys()
        .find(|p| p.ends_with("/room-native-Go-native.png"))
        .expect("retained reference image source required");
    let base = Path::new(reference).parent().expect("reference source directory required");
    let originals = [
        rgba(Path::new(old["NativeBackground"].as_str().unwrap())),
        rgba(Path::new(old["NativeLayer"].as_str().unwrap())),
    ];
    let mut rows = Vec::new();
    for view in evidence["views"].as_array().unwrap() {
        let left = view["left"].as_i64().unwrap() as i32;
        let width = view["width"].as_u64().unwrap() as usize;
        let name = if left == 0 { "native" } else { "wide" };
        let scene = sotn::build(&ram, &vram, &gfx, left, width).unwrap();
        assert_eq!(
            scene
                .materials
                .iter()
                .map(|m| key(m.key))
                .collect::<Vec<_>>(),
            serde_json::from_value::<Vec<Vec<String>>>(view["keys"].clone()).unwrap()
        );
        assert_eq!(scene.sprites.len(), view["tiles"].as_array().unwrap().len());
        for (s, w) in scene.sprites.iter().zip(view["tiles"].as_array().unwrap()) {
            assert_eq!(
                key(scene.materials[s.material].key),
                serde_json::from_value::<Vec<String>>(w["key"].clone()).unwrap()
            );
            assert_eq!(
                [
                    s.x as i64,
                    s.y as i64,
                    s.u as i64,
                    s.v as i64,
                    s.width as i64,
                    s.height as i64
                ],
                ["x", "y", "u", "v", "width", "height"].map(|k| w[k].as_i64().unwrap())
            );
        }
        let native = scene.render(1, None).unwrap();
        let native_rgba = native.rgba();
        assert_eq!(
            native_rgba,
            rgba(&base.join(format!("room-{name}-Go-native.png"))).2
        );
        let mut independent = vec![0; width * 240 * 4];
        for (layer, addr) in originals.iter().zip([0x730d8, 0x73084]) {
            let sx = i16::from_le_bytes(ram[addr + 10..addr + 12].try_into().unwrap()) as i32;
            let sy = i16::from_le_bytes(ram[addr + 14..addr + 16].try_into().unwrap()) as i32;
            for y in 0..240 {
                for x in 0..width {
                    let wx = sx + left + x as i32;
                    let wy = sy + y as i32;
                    if wx < 0 || wy < 0 || wx >= layer.0 as i32 || wy >= layer.1 as i32 {
                        continue;
                    }
                    let at = (wy as usize * layer.0 + wx as usize) * 4;
                    let p = &layer.2[at..at + 4];
                    if p[3] == 0 {
                        continue;
                    }
                    assert_eq!(p[3], 255);
                    independent[(y * width + x) * 4..(y * width + x) * 4 + 4].copy_from_slice(p);
                }
            }
        }
        assert_eq!(native_rgba, independent);
        let images = texture_audit::load_scene_images(
            Path::new(old["Pack"].as_str().unwrap()),
            &scene.materials.iter().map(|m| m.key).collect::<Vec<_>>(),
        )
        .unwrap();
        let hd = scene.render(4, Some(&images)).unwrap();
        let hd_rgba = hd.rgba();
        assert_eq!(
            hd_rgba,
            rgba(&base.join(format!("room-{name}-Go-HD4.png"))).2
        );
        let expected = fs::read(root.join(format!("reference-{name}.words"))).unwrap();
        let actual: Vec<u8> = hd.words.iter().flat_map(|w| w.to_le_bytes()).collect();
        assert_eq!(actual, expected);
        png_out(
            &root.join(format!("Rust-{name}-HD4.png")),
            hd.width,
            hd.height,
            &hd_rgba,
        );
        rows.push(json!({"view":name,"tiles":scene.sprites.len(),"materials":scene.materials.len(),"typed_data_equal":true,"native_independent_PNG_equal":true,"HD4_RGBA_and_all_words_equal":true,"HD_words":hd.words.len(),"HD_sha256":hex(&actual),"width":hd.width,"height":hd.height}));
    }
    let mut rejected = 0;
    for case in 0..6 {
        let mut r = ram.clone();
        let mut v = vram.clone();
        let mut g = gfx.clone();
        match case {
            0 => r[0x730f4..0x730f8].copy_from_slice(&2u32.to_le_bytes()),
            1 => r[0x730d8..0x730dc].copy_from_slice(&0x80180001u32.to_le_bytes()),
            2 => r[0x730f8..0x730fc].copy_from_slice(&u32::MAX.to_le_bytes()),
            3 => r[0x180000] ^= 1,
            4 => g[0] ^= 1,
            _ => v.fill(0),
        };
        assert!(sotn::build(&r, &v, &g, 0, 256).is_err());
        rejected += 1;
    }
    for group in ["sources", "image_sources"] {
        for (p, h) in evidence[group].as_object().unwrap() {
            let checked=if p=="/engine/rust/player/src/texture_audit.rs" {root.join("texture-audit-before.rs")} else {PathBuf::from(p)};
            assert_eq!(hex(&fs::read(checked).unwrap()), h.as_str().unwrap());
        }
    }
    fs::write(root.join("rust-verification.json"),serde_json::to_vec_pretty(&json!({"passed":true,"rows":rows,"source_guard_cases":rejected,"normal_game_runs":0,"art_reviews":0,"Machine_constructed":false,"MIPS_executions":0,"GP0_submissions":0,"full_HD_complete":false,"GPU_tested":false,"formal_player_registered":false,"speed_pass":false})).unwrap()).unwrap();
}
