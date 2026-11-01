//! Saved-source HD preview, spec026 §16.7.20–21. No Machine or game loop.
//! GPL-3.0-or-later; posters reuse 024 §6 without changing player geometry.
use super::{apps, display, poster, preferences, worker};
use apps::sotn::scene::Viewport;
use sdl2::{
    event::Event,
    keyboard::Keycode,
    mouse::MouseButton,
    pixels::{Color, PixelFormatEnum},
    rect::Rect,
};
use std::{ffi::OsString, path::PathBuf, sync::Arc, time::Duration};

const PRESETS: [(usize, usize); 4] = [(256, 240), (480, 270), (640, 360), (800, 600)];
struct Controls {
    viewport: Viewport,
    fields: [String; 2],
    focus: Option<usize>,
    replace: bool,
    pinned: bool,
    error: String,
}
impl Controls {
    fn new(viewport: Viewport) -> Self {
        let (w, h) = viewport.size();
        Self {
            viewport,
            fields: [w.to_string(), h.to_string()],
            focus: None,
            replace: false,
            pinned: false,
            error: String::new(),
        }
    }
    fn select(&mut self, n: usize) {
        self.focus = Some(n);
        self.replace = true;
    }
    fn text(&mut self, text: &str) {
        if let Some(i) = self.focus {
            let mut value = if self.replace {
                String::new()
            } else {
                self.fields[i].clone()
            };
            value.push_str(text);
            if value.chars().count() <= 8 {
                self.fields[i] = value;
                self.replace = false;
            }
        }
    }
    fn requested(&mut self) -> Option<Viewport> {
        match Viewport::parse(&format!("{}x{}", self.fields[0], self.fields[1])) {
            Ok(v) => Some(v),
            Err(e) => {
                self.error = e;
                None
            }
        }
    }
    fn accept(&mut self, v: Viewport) {
        self.viewport = v;
        self.fields = [v.size().0.to_string(), v.size().1.to_string()];
        self.focus = None;
        self.error.clear();
    }
}
fn image(c: psgolem_core::scene::Canvas) -> worker::Image {
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
}
fn button(
    c: &mut sdl2::render::Canvas<sdl2::video::Window>,
    font: &display::Font,
    t: &sdl2::render::Texture<'_>,
    x: i32,
    w: u32,
    label: &str,
    active: bool,
) -> Result<(), String> {
    c.set_draw_color(if active {
        Color::RGB(71, 123, 187)
    } else {
        Color::RGB(52, 65, 85)
    });
    c.fill_rect(Rect::new(x, 3, w, 30))?;
    font.draw(c, t, label, x + 6, 5)
}
pub fn run(args: &[OsString]) -> Result<(), String> {
    let source = PathBuf::from(args.first().ok_or("預覽需要來源描述檔")?);
    let (mut posters, mut screenshot, mut frames, mut report_path) = (None, None, None, None);
    let mut initial = Viewport::default();
    let mut i = 1;
    while i < args.len() {
        let key = args[i].to_str().ok_or("預覽參數不符")?;
        i += 1;
        let value = args.get(i).ok_or("預覽參數缺少值")?;
        i += 1;
        match key {
            "-posters" => posters = Some(PathBuf::from(value)),
            "-screenshot" => screenshot = Some(PathBuf::from(value)),
            "-report" => report_path = Some(PathBuf::from(value)),
            "-viewport" => initial = Viewport::parse(value.to_str().ok_or("視野參數不符")?)?,
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
    let (native, hd, mut report) = apps::sotn::view::render_file_at(&source, initial, true)?;
    let (mut original, mut high) = (image(native), image(hd));
    let sdl = sdl2::init()?;
    let video = sdl.video()?;
    let mut window = video
        .window("月下 HD 預覽", 1920, 1080)
        .resizable()
        .position_centered()
        .build()
        .map_err(|e| e.to_string())?;
    window
        .set_minimum_size(640, 360)
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
    let (mut nt, mut ht) = (make_texture(&original)?, make_texture(&high)?);
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
    let font = display::Font::load()?;
    let ft = font.texture(&tc)?;
    video.text_input().start();
    let mut controls = Controls::new(initial);
    let mut pump = sdl.event_pump()?;
    let mut is_hd = true;
    let mut shown = 0;
    let mut screenshots = 0;
    let mut applied = 0;
    let mut last_ui = serde_json::Value::Null;
    'show: loop {
        let (w, h) = canvas.output_size()?;
        let ui_scale = (w as f32 / 1280.).clamp(0.5, 1.5);
        let (ww, wh) = canvas.window().size();
        let mouse = pump.mouse_state();
        let mx = mouse.x() as f32 * w as f32 / ww.max(1) as f32 / ui_scale;
        let my = mouse.y() as f32 * h as f32 / wh.max(1) as f32 / ui_scale;
        let mut visible = controls.pinned || (mx >= 0. && mx < 1280. && (0. ..72.).contains(&my));
        let mut requested = None;
        let mut capture = shown == 0;
        for event in pump.poll_iter() {
            match event {
                Event::Quit { .. } => break 'show,
                Event::KeyDown {
                    keycode: Some(Keycode::Escape),
                    repeat: false,
                    ..
                } => {
                    if controls.focus.take().is_none() {
                        break 'show;
                    }
                }
                Event::KeyDown {
                    keycode: Some(Keycode::F1),
                    repeat: false,
                    ..
                } => {
                    controls.pinned = !controls.pinned;
                    visible =
                        controls.pinned || (mx >= 0. && mx < 1280. && (0. ..72.).contains(&my));
                }
                Event::KeyDown {
                    keycode: Some(Keycode::F12),
                    repeat: false,
                    ..
                } => capture = true,
                Event::TextInput { text, .. } if visible => controls.text(&text),
                Event::KeyDown {
                    keycode: Some(Keycode::Backspace),
                    ..
                } if visible => {
                    if let Some(i) = controls.focus {
                        if controls.replace {
                            controls.fields[i].clear();
                            controls.replace = false;
                        } else {
                            controls.fields[i].pop();
                        }
                    }
                }
                Event::KeyDown {
                    keycode: Some(Keycode::Tab),
                    repeat: false,
                    ..
                } if visible => {
                    controls.select(if controls.focus == Some(0) { 1 } else { 0 });
                }
                Event::KeyDown {
                    keycode: Some(Keycode::Return | Keycode::KpEnter),
                    repeat: false,
                    ..
                } if visible => requested = controls.requested(),
                Event::KeyDown {
                    keycode: Some(Keycode::N),
                    repeat: false,
                    ..
                } if controls.focus.is_none() => is_hd = !is_hd,
                Event::KeyDown {
                    keycode: Some(Keycode::P),
                    repeat: false,
                    ..
                } if controls.focus.is_none() => {
                    if let Some(p) = &art {
                        choice.id = p.next(&choice.id);
                    }
                }
                Event::MouseButtonDown {
                    mouse_btn: MouseButton::Left,
                    x,
                    y,
                    ..
                } => {
                    let x = x as f32 * w as f32 / ww.max(1) as f32 / ui_scale;
                    let y = y as f32 * h as f32 / wh.max(1) as f32 / ui_scale;
                    if visible && (3. ..33.).contains(&y) {
                        controls.focus = None;
                        let mut preset = None;
                        for (i, &(vw, vh)) in PRESETS.iter().enumerate() {
                            if ((12 + i * 132) as f32..(134 + i * 132) as f32).contains(&x) {
                                preset = Some(Viewport::new(vw, vh)?);
                            }
                        }
                        if preset.is_some() {
                            requested = preset;
                        } else if (548. ..632.).contains(&x) {
                            controls.select(0);
                        } else if (650. ..734.).contains(&x) {
                            controls.select(1);
                        } else if (744. ..828.).contains(&x) {
                            requested = controls.requested();
                        } else if (838. ..922.).contains(&x) {
                            requested = Some(Viewport::default());
                        } else if (932. ..1032.).contains(&x) {
                            if let Some(p) = &art {
                                choice.id = p.next(&choice.id);
                            }
                        } else if (1042. ..1182.).contains(&x) {
                            is_hd = !is_hd;
                        }
                    } else {
                        controls.focus = None;
                    }
                }
                _ => (),
            }
        }
        if let Some(next) = requested {
            // Prepare both canvases and both textures before replacing the visible pair.
            let prepared =
                apps::sotn::view::render_file_at(&source, next, false).and_then(|(n, h, r)| {
                    let (n, h) = (image(n), image(h));
                    let (nt, ht) = (make_texture(&n)?, make_texture(&h)?);
                    Ok((n, h, nt, ht, r))
                });
            match prepared {
                Ok((n, h, ntex, htex, r)) => {
                    original = n;
                    high = h;
                    nt = ntex;
                    ht = htex;
                    report = r;
                    controls.accept(next);
                    applied += 1;
                }
                Err(e) => controls.error = e,
            }
        }
        canvas.set_clip_rect(None);
        canvas.set_draw_color(Color::RGB(0, 0, 0));
        canvas.clear();
        let (f, t) = if is_hd {
            (&high, &ht)
        } else {
            (&original, &nt)
        };
        let layout = poster::source_geometry(w, h, f.w, f.h);
        let covered = if let Some(p) = art.as_mut() {
            p.draw_source(&mut canvas, f, t, &choice)?.is_some()
        } else {
            false
        };
        if !covered {
            canvas.copy(t, None, layout.game)?;
        }
        if visible {
            canvas.set_scale(ui_scale, ui_scale)?;
            canvas.set_draw_color(Color::RGB(20, 27, 39));
            canvas.fill_rect(Rect::new(0, 0, 1280, 72))?;
            for (i, &(vw, vh)) in PRESETS.iter().enumerate() {
                button(
                    &mut canvas,
                    &font,
                    &ft,
                    12 + i as i32 * 132,
                    122,
                    &format!("{vw}x{vh}"),
                    controls.viewport.size() == (vw, vh),
                )?;
            }
            button(
                &mut canvas,
                &font,
                &ft,
                548,
                84,
                &format!("寬{}", controls.fields[0]),
                controls.focus == Some(0),
            )?;
            button(
                &mut canvas,
                &font,
                &ft,
                650,
                84,
                &format!("高{}", controls.fields[1]),
                controls.focus == Some(1),
            )?;
            button(&mut canvas, &font, &ft, 744, 84, "套用", false)?;
            button(&mut canvas, &font, &ft, 838, 84, "恢復", false)?;
            button(
                &mut canvas,
                &font,
                &ft,
                932,
                100,
                "海報",
                !choice.id.is_empty(),
            )?;
            button(
                &mut canvas,
                &font,
                &ft,
                1042,
                140,
                if is_hd { "HD素材" } else { "原素材" },
                is_hd,
            )?;
            let status = if controls.error.is_empty() {
                format!(
                    "視野 {}x{}  輸出 {}x{}  F1  F12",
                    controls.viewport.size().0,
                    controls.viewport.size().1,
                    w,
                    h
                )
            } else {
                format!("設定失敗：{}", controls.error)
            };
            font.draw(&mut canvas, &ft, &status, 12, 40)?;
            canvas.set_scale(1., 1.)?;
        }
        if capture {
            if let Some(path) = &screenshot {
                let pixels = canvas.read_pixels(None, PixelFormatEnum::RGBA32)?;
                display::png(path, w, h, &pixels)?;
                screenshots += 1;
            }
        }
        canvas.present();
        shown += 1;
        let ui = serde_json::json!({"scope":"saved-source HD preview window; not gameplay","viewport":report["viewport"],"window_width":w,"window_height":h,"game_rect":[layout.game.x(),layout.game.y(),layout.game.width() as i32,layout.game.height() as i32],"poster_id":choice.id,"HD":is_hd,"toolbar_visible":visible,"pinned":controls.pinned,"custom_fields":controls.fields,"focus":controls.focus,"error":controls.error,"applied_changes":applied,"screenshots":screenshots,"settings_modified":false,"cards_opened":false,"original_CPU_executed":false,"GPU_command_stream_input":false,"HD_canvas":report});
        if ui != last_ui {
            canvas
                .window_mut()
                .set_title(&format!(
                    "月下 HD 預覽 | {}x{} | {}",
                    controls.viewport.size().0,
                    controls.viewport.size().1,
                    if is_hd { "HD" } else { "原素材" }
                ))
                .map_err(|e| e.to_string())?;
            if let Some(path) = &report_path {
                let tmp = path.with_extension("preview-tmp");
                std::fs::write(
                    &tmp,
                    serde_json::to_vec_pretty(&ui).map_err(|e| e.to_string())?,
                )
                .map_err(|e| e.to_string())?;
                std::fs::rename(tmp, path).map_err(|e| e.to_string())?;
            }
            last_ui = ui;
        }
        if frames.is_some_and(|n| shown >= n) {
            break;
        }
        std::thread::sleep(Duration::from_millis(16));
    }
    video.text_input().stop();
    last_ui["presented_frames"] = serde_json::json!(shown);
    println!("{last_ui}");
    Ok(())
}
#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn invalid_draft_preserves_view_and_replacement_works() {
        let mut c = Controls::new(Viewport::default());
        c.select(0);
        c.text("801");
        assert!(c.requested().is_none());
        assert_eq!(c.viewport, Viewport::default());
        c.select(0);
        c.text("799");
        c.select(1);
        c.text("599");
        let v = c.requested().unwrap();
        c.accept(v);
        assert_eq!(c.viewport.size(), (799, 599));
        assert!(c.error.is_empty());
        c.select(0);
        c.text("abc");
        assert!(c.requested().is_none());
        assert_eq!(c.viewport, v);
    }
}
