//! Source-driven HD canvas preview. No Machine, BIOS, disc, card or game loop.
//! GPL-3.0-or-later; 026 §16.7.20, poster presentation reuses 024 §6.
use super::{apps, display, poster, preferences, worker};
use sdl2::{
    event::Event,
    keyboard::Keycode,
    pixels::{Color, PixelFormatEnum},
};
use std::{ffi::OsString, path::PathBuf, sync::Arc, time::Duration};
pub fn run(args: &[OsString]) -> Result<(), String> {
    let source = PathBuf::from(args.first().ok_or("預覽需要來源描述檔")?);
    let (mut posters, mut screenshot, mut frames) = (None, None, None);
    let mut i = 1;
    while i < args.len() {
        let key = args[i].to_str().ok_or("預覽參數不符")?;
        i += 1;
        let value = args.get(i).ok_or("預覽參數缺少值")?;
        i += 1;
        match key {
            "-posters" => posters = Some(PathBuf::from(value)),
            "-screenshot" => screenshot = Some(PathBuf::from(value)),
            "-frames" => {
                let n: usize = value
                    .to_str()
                    .ok_or("frames不符")?
                    .parse()
                    .map_err(|_| "frames不符")?;
                if !(1..=100000).contains(&n) {
                    return Err("frames超界".into());
                }
                frames = Some(n);
            }
            _ => return Err("預覽參數不符".into()),
        }
    }
    let (native, hd, report) = apps::sotn::view::render_file(&source)?;
    let make = |c: psgolem_core::scene::Canvas| {
        let mut rgba = c.rgba();
        for p in rgba.chunks_exact_mut(4) {
            p[3] = 255;
        }
        worker::Image {
            field: 0,
            w: c.width as u32,
            h: c.height as u32,
            rgba: Arc::new(rgba),
            movie: None,
        }
    };
    let original = make(native);
    let high = make(hd);
    let sdl = sdl2::init()?;
    let video = sdl.video()?;
    let window = video
        .window(
            "月下 HD 畫面預覽｜N 原素材／HD，P 海報，Esc 關閉",
            1920,
            1080,
        )
        .resizable()
        .position_centered()
        .build()
        .map_err(|e| e.to_string())?;
    sdl2::hint::set("SDL_RENDER_SCALE_QUALITY", "nearest");
    let mut canvas = window.into_canvas().build().map_err(|e| e.to_string())?;
    let tc = canvas.texture_creator();
    let make_texture = |f: &worker::Image| -> Result<_, String> {
        let mut t = tc
            .create_texture_streaming(PixelFormatEnum::RGBA32, f.w, f.h)
            .map_err(|e| e.to_string())?;
        t.update(None, &f.rgba, f.w as usize * 4)
            .map_err(|e| e.to_string())?;
        Ok(t)
    };
    let nt = make_texture(&original)?;
    let ht = make_texture(&high)?;
    let mut art = posters
        .map(|p| poster::Pack::load(&p).and_then(|p| poster::Renderer::new(&tc, p)))
        .transpose()?;
    let mut choice = preferences::Choice {
        zoom: false,
        ..preferences::Choice::default()
    };
    if let Some(p) = &art {
        choice.id = p.next("");
    }
    let mut pump = sdl.event_pump()?;
    let mut is_hd = true;
    let mut shown = 0;
    'show: loop {
        for event in pump.poll_iter() {
            match event {
                Event::Quit { .. }
                | Event::KeyDown {
                    keycode: Some(Keycode::Escape),
                    ..
                } => break 'show,
                Event::KeyDown {
                    keycode: Some(Keycode::N),
                    repeat: false,
                    ..
                } => is_hd = !is_hd,
                Event::KeyDown {
                    keycode: Some(Keycode::P),
                    repeat: false,
                    ..
                } => {
                    if let Some(p) = &art {
                        choice.id = p.next(&choice.id);
                    }
                }
                _ => (),
            }
        }
        canvas.set_clip_rect(None);
        canvas.set_draw_color(Color::RGB(0, 0, 0));
        canvas.clear();
        let f = if is_hd { &high } else { &original };
        let t = if is_hd { &ht } else { &nt };
        let covered = if let Some(p) = art.as_mut() {
            p.draw(&mut canvas, f, t, &choice)?.is_some()
        } else {
            false
        };
        if !covered {
            let (w, h) = canvas.output_size()?;
            canvas.copy(t, None, display::fit(w, h))?;
        }
        if shown == 0 {
            if let Some(path) = &screenshot {
                let (w, h) = canvas.output_size()?;
                let pixels = canvas.read_pixels(None, PixelFormatEnum::RGBA32)?;
                display::png(path, w, h, &pixels)?;
            }
        }
        canvas.present();
        shown += 1;
        if frames.is_some_and(|n| shown >= n) {
            break;
        }
        std::thread::sleep(Duration::from_millis(16));
    }
    println!(
        "{}",
        serde_json::json!({"scope":"saved-source HD preview window; not gameplay","window_width":canvas.output_size()?.0,"window_height":canvas.output_size()?.1,"presented_frames":shown,"poster_id":choice.id,"original_CPU_executed":false,"GPU_command_stream_input":false,"HD_canvas":report,"settings_modified":false,"cards_opened":false})
    );
    Ok(())
}
