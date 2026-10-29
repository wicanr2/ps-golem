//! Rust native player window, spec026 section14. GPL-3.0-or-later.
//! SDL2 is a platform binding; CPU/GTE/GPU/SPU/CD/MDEC remain our Rust core.
mod apps;
mod display;
mod input;
mod poster;
mod preferences;
mod worker;
use psgolem_core::{cardfile, disc::Disc, machine::Machine};
use sdl2::{
    event::{Event, WindowEvent},
    keyboard::Keycode,
    pixels::{Color, PixelFormatEnum},
};
use serde_json::json;
use std::{
    env, fs,
    path::PathBuf,
    sync::{atomic::Ordering, Arc},
    time::{Duration, Instant},
};
type Result<T> = std::result::Result<T, String>;
#[derive(Default)]
struct Options {
    bios: PathBuf,
    cue: PathBuf,
    card: Option<PathBuf>,
    fields: Option<u64>,
    script: Option<String>,
    report: Option<PathBuf>,
    screenshot: Option<PathBuf>,
    mute: bool,
    unpaced: bool,
    present: String,
    size: Option<(u32, u32)>,
    native: bool,
    movies: Option<PathBuf>,
    ffmpeg: PathBuf,
    ffprobe: PathBuf,
    captures: Vec<u64>,
    settings: Option<PathBuf>,
    posters: Option<String>,
    poster: Option<String>,
    poster_soft: Option<bool>,
    poster_zoom: Option<bool>,
}
fn options() -> Result<Option<Options>> {
    let mut o = Options {
        present: "auto".into(),
        ffmpeg: "/usr/bin/ffmpeg".into(),
        ffprobe: "/usr/bin/ffprobe".into(),
        ..Options::default()
    };
    let mut args = env::args().skip(1);
    while let Some(a) = args.next() {
        if a == "-h" || a == "--help" {
            println!("Rust 玩家視窗（原版／LOGO與OPN的HD影片；可切換海報；完整手把設定面板仍待遷移）\npsgolem-player -native -bios FILE -cue FILE [-card FILE] [-resolution 1920x1080] [-mute] [-present auto|cpu|gpu]\nHD影片：以-sotn-hd-movies DIR取代-native；-ffmpeg FILE -ffprobe FILE指定工具。\n鍵盤同 Go 版；已連線標準手把；F1 顯示工具列；F2 暫停／續播；Esc 關窗保存原卡。\n海報：-posters DIR -poster ID [-poster-soft true|false] [-poster-zoom true|false] [-settings FILE]。\n診斷：-fields N [-press SCRIPT | -press-file FILE] [-report PREFIX] [-screenshot FILE] [-capture-fields N,N（需-report）] [-unpaced（限靜音有界診斷）]");
            return Ok(None);
        }
        if a == "-version" {
            println!("psgolem-player Rust preview / specs024,026,027");
            return Ok(None);
        }
        match a.as_str() {
            "-native" => o.native = true,
            a if a.starts_with("-poster-soft=") => {
                o.poster_soft = Some(a[13..].parse().map_err(|_| "poster-soft須為true或false")?)
            }
            a if a.starts_with("-poster-zoom=") => {
                o.poster_zoom = Some(a[13..].parse().map_err(|_| "poster-zoom須為true或false")?)
            }
            "-mute" => o.mute = true,
            "-unpaced" => o.unpaced = true,
            _ => {
                let v = args.next().ok_or_else(|| format!("{a} 缺少參數"))?;
                match a.as_str() {
                    "-bios" => o.bios = v.into(),
                    "-cue" => o.cue = v.into(),
                    "-card" => o.card = Some(v.into()),
                    "-fields" => o.fields = Some(v.parse().map_err(|_| "fields 格式錯誤")?),
                    "-press" | "-press-file" => {
                        if o.script.is_some() {
                            return Err("按鍵腳本只能指定一次".into());
                        }
                        o.script = Some(if a == "-press-file" {
                            fs::read_to_string(v).map_err(|e| e.to_string())?
                        } else {
                            v
                        });
                    }
                    "-report" => o.report = Some(v.into()),
                    "-screenshot" => o.screenshot = Some(v.into()),
                    "-present" => o.present = v,
                    "-settings" => o.settings = Some(v.into()),
                    "-posters" => o.posters = Some(v),
                    "-poster" => o.poster = Some(v),
                    "-poster-soft" => {
                        o.poster_soft = Some(v.parse().map_err(|_| "poster-soft須為true或false")?)
                    }
                    "-poster-zoom" => {
                        o.poster_zoom = Some(v.parse().map_err(|_| "poster-zoom須為true或false")?)
                    }
                    "-sotn-hd-movies" => o.movies = Some(v.into()),
                    "-ffmpeg" => o.ffmpeg = v.into(),
                    "-ffprobe" => o.ffprobe = v.into(),
                    "-capture-fields" => {
                        o.captures = v
                            .split(',')
                            .map(|n| {
                                n.parse::<u64>()
                                    .map_err(|_| "capture-fields格式錯誤".to_string())
                            })
                            .collect::<Result<Vec<_>>>()?;
                    }
                    "-resolution" => {
                        let (w, h) = v.split_once('x').ok_or("解析度須為 WIDTHxHEIGHT")?;
                        let (w, h) = (
                            w.parse::<u32>().map_err(|_| "寬度錯誤")?,
                            h.parse::<u32>().map_err(|_| "高度錯誤")?,
                        );
                        if !(320..=7680).contains(&w) || !(240..=4320).contains(&h) {
                            return Err("解析度超出範圍".into());
                        }
                        o.size = Some((w, h));
                    }
                    _ => return Err(format!("未知選項 {a}")),
                }
            }
        }
    }
    if o.native == o.movies.is_some() {
        return Err("請指定-native或-sotn-hd-movies DIR，兩者互斥；既有HD偏好不會被改寫".into());
    }
    if !o.captures.is_empty()
        && (o.report.is_none()
            || o.captures.len() > 16
            || o.captures
                .iter()
                .any(|&n| n == 0 || o.fields.is_some_and(|f| n > f)))
    {
        return Err("capture-fields需-report，限16個有效正整數場數且不得超過-fields".into());
    }
    if o.bios.as_os_str().is_empty() || o.cue.as_os_str().is_empty() {
        return Err("請指定 -bios 與 -cue".into());
    }
    if !["auto", "cpu", "gpu"].contains(&o.present.as_str()) {
        return Err("present 必須為 auto、cpu 或 gpu".into());
    }
    if o.fields == Some(0) {
        return Err("fields 必須大於零".into());
    }
    if o.unpaced && (!o.mute || o.fields.is_none()) {
        return Err("unpaced 需要 -mute 及明確 -fields".into());
    }
    Ok(Some(o))
}
fn default_card(boot: &str) -> Result<PathBuf> {
    let root = if cfg!(target_os = "windows") {
        env::var_os("APPDATA").map(PathBuf::from)
    } else if cfg!(target_os = "macos") {
        env::var_os("HOME").map(|s| PathBuf::from(s).join("Library/Application Support"))
    } else {
        env::var_os("XDG_CONFIG_HOME")
            .map(PathBuf::from)
            .or_else(|| env::var_os("HOME").map(|s| PathBuf::from(s).join(".config")))
    };
    if boot.is_empty()
        || !boot
            .bytes()
            .all(|b| b.is_ascii_alphanumeric() || b == b'_' || b == b'.')
    {
        return Err("無法決定原卡檔名，請使用 -card".into());
    }
    let root = root
        .ok_or("無法決定使用者設定目錄，請使用 -card")?
        .join("ps-golem");
    fs::create_dir_all(&root).map_err(|e| e.to_string())?;
    Ok(root.join(format!("{boot}.mcd")))
}
fn main() {
    if let Err(e) = entry() {
        eprintln!("psgolem-player：{e}");
        std::process::exit(1)
    }
}
fn entry() -> Result<()> {
    let Some(o) = options()? else { return Ok(()) };
    let disc = Disc::open(&o.cue)?;
    let card = match &o.card {
        Some(p) => p.clone(),
        None => default_card(disc.boot_file())?,
    };
    let mut m = Machine::new(&fs::read(&o.bios).map_err(|e| e.to_string())?)?;
    m.bus.sio.cards[0] = Some(cardfile::load_or_new(&card).map_err(|e| e.to_string())?);
    let movie = o
        .movies
        .as_ref()
        .map(|root| {
            apps::sotn::Overlay::load(
                &disc,
                root,
                o.ffmpeg.clone(),
                o.ffprobe.clone(),
                o.report.is_some(),
            )
        })
        .transpose()?;
    if movie.is_some() {
        m.bus.cd.enable_sector_reads(true);
    }
    m.insert_disc(disc);
    let script = o
        .script
        .as_ref()
        .map(|s| psgolem_core::input::parse(s))
        .transpose()?;
    let (mut prefs, pack, mut poster_error) = poster_setup(&o)?;
    let sdl = sdl2::init()?;
    let video = sdl.video()?;
    let controllers = sdl.game_controller()?;
    let (w, h) = o.size.unwrap_or((1280, 960));
    let mode = if o.movies.is_some() {
        "HD影片"
    } else {
        "原版"
    };
    let window = video
        .window(&format!("ps-golem Rust {mode}"), w, h)
        .resizable()
        .position_centered()
        .build()
        .map_err(|e| e.to_string())?;
    sdl2::hint::set("SDL_RENDER_SCALE_QUALITY", "nearest");
    let mut builder = window.into_canvas();
    if o.present == "cpu" {
        builder = builder.software()
    } else if o.present == "gpu" {
        builder = builder.accelerated()
    }
    let mut canvas = builder.build().map_err(|e| e.to_string())?;
    eprintln!(
        "Rust {mode}視窗：SDL呈現={}；PS1繪圖=原生Rust軟體GPU；記憶卡={}",
        canvas.info().name,
        card.display()
    );
    let font = display::Font::load()?;
    let tc = canvas.texture_creator();
    let font_texture = font.texture(&tc)?;
    let mut posters = pack
        .map(|pack| poster::Renderer::new(&tc, pack))
        .transpose()?;
    let audio = Arc::new(worker::Audio::default());
    let audio_system = if o.mute {
        None
    } else {
        sdl.audio().map_err(|e| eprintln!("音訊改為靜音：{e}")).ok()
    };
    let device = if let Some(a) = &audio_system {
        let desired = sdl2::audio::AudioSpecDesired {
            freq: Some(44100),
            channels: Some(2),
            samples: Some(1024),
        };
        let q = audio.clone();
        a.open_playback(None, &desired, move |_| worker::Output(q))
            .map_err(|e| eprintln!("音訊改為靜音：{e}"))
            .ok()
    } else {
        None
    };
    let mut events = sdl.event_pump()?;
    canvas.set_draw_color(Color::BLACK);
    canvas.clear();
    canvas.present(); // 012: first visible Draw before machine.
    if let Some(d) = &device {
        d.resume()
    }
    let shared = Arc::new(worker::Shared::default());
    let worker_opts = worker::Options {
        fields: o.fields,
        script,
        card,
        unpaced: o.unpaced,
        report: o.report.clone(),
        movie,
        captures: o.captures.clone(),
    };
    let producer = shared.clone();
    let sound = device.as_ref().map(|_| audio.clone());
    let thread = std::thread::spawn(move || worker::run(m, producer, sound, worker_opts));
    let mut pads = Vec::new();
    let mut rescan = true;
    let (mut focused, mut last_pause) = (true, false);
    let (mut shown, mut draws) = (0, 0u64);
    let mut latest = None::<worker::Image>;
    let mut last_title = Instant::now();
    let mut screenshot = None::<Vec<u8>>;
    let mut screenshot_size = (w, h);
    let mut freeze = None::<(u64, u64, u64)>;
    let mut freeze_checks = 0u64;
    let mut captures = std::collections::BTreeMap::<u64, serde_json::Value>::new();
    let mut pinned = false;
    let mut poster_events = Vec::<serde_json::Value>::new();
    let mut pause_sources = Vec::<serde_json::Value>::new();
    let mut poster_layout = None::<poster::Layout>;
    let mut observed_ui = None::<serde_json::Value>;
    // Always reach cleanup after the worker has started, including renderer errors.
    let gui_result = (|| -> Result<()> {
        'ui: loop {
            for ev in events.poll_iter() {
                match ev {
                    Event::Quit { .. }
                    | Event::KeyDown {
                        keycode: Some(Keycode::Escape),
                        repeat: false,
                        ..
                    } => break 'ui,
                    Event::KeyDown {
                        keycode: Some(Keycode::F1),
                        repeat: false,
                        ..
                    } => {
                        pinned = !pinned;
                    }
                    Event::KeyDown {
                        keycode: Some(Keycode::F2),
                        repeat: false,
                        ..
                    } => {
                        let mut s = shared.state.lock().unwrap();
                        s.pause = !s.pause;
                        shared.wake.notify_all();
                    }
                    Event::MouseButtonDown {
                        mouse_btn: sdl2::mouse::MouseButton::Left,
                        x,
                        y,
                        ..
                    } => {
                        let width = canvas.output_size()?.0;
                        if toolbar_visible(pinned, x, y, width) && (2..30).contains(&y) {
                            if (116..222).contains(&x) {
                                let mut st = shared.state.lock().unwrap();
                                st.pause = !st.pause;
                                shared.wake.notify_all();
                            } else if let Some(renderer) = posters.as_ref() {
                                let mut next = prefs.choice.clone();
                                let mut change = true;
                                if (230..324).contains(&x) {
                                    next.id = renderer.next(&next.id);
                                } else if (332..402).contains(&x) {
                                    next.soft = !next.soft;
                                } else if (410..480).contains(&x) {
                                    next.zoom = !next.zoom;
                                } else {
                                    change = false;
                                }
                                if change {
                                    let result = prefs.save(&next);
                                    if let Err(e) = &result {
                                        poster_error = format!("儲存失敗：{e}");
                                    } else {
                                        prefs.choice = next;
                                        poster_error.clear();
                                    }
                                    let st = shared.state.lock().unwrap();
                                    poster_events.push(json!({"field":st.field,"cycles":st.cycles,"ok":result.is_ok(),
                                        "id":prefs.choice.id,"soft":prefs.choice.soft,"zoom":prefs.choice.zoom,"error":poster_error}));
                                }
                            }
                        }
                    }
                    Event::ControllerDeviceAdded { .. } | Event::ControllerDeviceRemoved { .. } => {
                        rescan = true
                    }
                    Event::Window {
                        win_event: WindowEvent::FocusLost,
                        ..
                    } => focused = false,
                    Event::Window {
                        win_event: WindowEvent::FocusGained,
                        ..
                    } => focused = true,
                    _ => {}
                }
            }
            if rescan {
                pads.clear();
                for i in 0..controllers.num_joysticks()? {
                    if controllers.is_game_controller(i) {
                        if let Ok(p) = controllers.open(i) {
                            eprintln!("標準手把：{}", p.name());
                            pads.push(p);
                            break;
                        }
                    }
                }
                rescan = false;
            }
            let mask = if focused {
                input::keyboard(|k| events.keyboard_state().is_scancode_pressed(k))
                    | pads.first().map(input::controller).unwrap_or(0)
            } else {
                0
            };
            let (done, paused, pause, field, cycles) = {
                let mut s = shared.state.lock().unwrap();
                s.buttons = mask;
                if let Some(f) = s.frame.take() {
                    latest = Some(f);
                }
                (s.done, s.paused, s.pause, s.field, s.cycles)
            };
            if paused != last_pause {
                if let Some(d) = &device {
                    if paused {
                        d.pause()
                    } else {
                        d.resume()
                    }
                }
                if paused {
                    freeze = Some((field, cycles, audio.consumed.load(Ordering::Relaxed)));
                    eprintln!("暫停確認 field={field} cycles={cycles}");
                    if let (Some(prefix), Some(f)) = (&o.report, &latest) {
                        let path = prefix
                            .with_extension(format!("pause{}-source.png", pause_sources.len() + 1));
                        display::png(&path, f.w, f.h, &f.rgba)?;
                        pause_sources.push(json!({"path":path,"field":field,"cycles":cycles,"width":f.w,"height":f.h,"rgba_sha256":worker::hash(&f.rgba)}));
                    }
                } else {
                    freeze = None;
                    eprintln!("續播 field={field} cycles={cycles}");
                }
                last_pause = paused;
            }
            if let Some(f) = freeze {
                if (field, cycles, audio.consumed.load(Ordering::Relaxed)) != f {
                    return Err("暫停期間機器或音訊仍在推進".into());
                }
                freeze_checks += 1;
            }
            canvas.set_draw_color(Color::BLACK);
            canvas.clear();
            let (size_w, size_h) = canvas.output_size()?;
            if let Some(f) = &latest {
                let mut t = tc
                    .create_texture_streaming(PixelFormatEnum::RGBA32, f.w, f.h)
                    .map_err(|e| e.to_string())?;
                t.update(None, &f.rgba, f.w as usize * 4)
                    .map_err(|e| e.to_string())?;
                poster_layout = if let Some(renderer) = posters.as_mut() {
                    renderer.draw(&mut canvas, f, &t, &prefs.choice)?
                } else {
                    None
                };
                if poster_layout.is_none() {
                    canvas.copy(&t, None, display::fit(size_w, size_h))?;
                }
                shown = f.field;
            }
            let mouse = events.mouse_state();
            if toolbar_visible(pinned, mouse.x(), mouse.y(), size_w) {
                canvas.set_draw_color(Color::RGB(37, 42, 53));
                canvas.fill_rect(sdl2::rect::Rect::new(0, 0, size_w, 32))?;
                font.draw(&mut canvas, &font_texture, "ps-golem", 14, 4)?;
                toolbar_button(
                    &mut canvas,
                    &font,
                    &font_texture,
                    116,
                    106,
                    if pause { "繼續" } else { "暫停" },
                    pause,
                )?;
                if let Some(renderer) = posters.as_ref() {
                    let label = if renderer.selected(&prefs.choice.id) {
                        format!("海報 {}", prefs.choice.id)
                    } else {
                        "海報 關閉".into()
                    };
                    toolbar_button(
                        &mut canvas,
                        &font,
                        &font_texture,
                        230,
                        94,
                        &label,
                        renderer.selected(&prefs.choice.id),
                    )?;
                    toolbar_button(
                        &mut canvas,
                        &font,
                        &font_texture,
                        332,
                        70,
                        "柔和",
                        prefs.choice.soft,
                    )?;
                    toolbar_button(
                        &mut canvas,
                        &font,
                        &font_texture,
                        410,
                        70,
                        "放大",
                        prefs.choice.zoom,
                    )?;
                }
                if !poster_error.is_empty() {
                    font.draw(&mut canvas, &font_texture, &poster_error, 14, 36)?;
                }
            }
            if o.screenshot.is_some() && (done || pause) {
                screenshot = Some(canvas.read_pixels(None, PixelFormatEnum::RGBA32)?);
                screenshot_size = (size_w, size_h);
            }
            let capture = if o.captures.contains(&shown) && !captures.contains_key(&shown) {
                Some(canvas.read_pixels(None, PixelFormatEnum::RGBA32)?)
            } else {
                None
            };
            canvas.present();
            if let Some(bytes) = capture {
                let path = o
                    .report
                    .as_ref()
                    .unwrap()
                    .with_extension(format!("field{shown}.png"));
                display::png(&path, size_w, size_h, &bytes)?;
                captures.insert(
                    shown,
                    json!({"path":path,"width":size_w,"height":size_h,
                    "movie":latest.as_ref().and_then(|f|f.movie.clone())}),
                );
            }
            {
                let mut s = shared.state.lock().unwrap();
                s.presented_field = shown;
                shared.wake.notify_all();
            }
            if let Some(prefix) = &o.report {
                let layout=poster_layout.map(|l|json!({"canvas":[l.canvas.x(),l.canvas.y(),l.canvas.width(),l.canvas.height()],"game":[l.game.x(),l.game.y(),l.game.width(),l.game.height()],"zoomed":l.zoomed}));
                let mouse = events.mouse_state();
                let key = json!({"id":prefs.choice.id,"soft":prefs.choice.soft,"zoom":prefs.choice.zoom,"error":poster_error,
                    "pinned":pinned,"visible":toolbar_visible(pinned,mouse.x(),mouse.y(),size_w),"paused":paused,"pause_requested":pause,
                    "width":size_w,"height":size_h,"layout":layout});
                if observed_ui.as_ref() != Some(&key) {
                    let mut state = key.clone();
                    state["field"] = json!(field);
                    state["cycles"] = json!(cycles);
                    let temporary = prefix.with_extension("ui.tmp");
                    let target = prefix.with_extension("ui.json");
                    fs::write(
                        &temporary,
                        serde_json::to_vec(&state).map_err(|e| e.to_string())?,
                    )
                    .map_err(|e| e.to_string())?;
                    fs::rename(temporary, target).map_err(|e| e.to_string())?;
                    observed_ui = Some(key);
                }
            }
            draws += 1;
            if last_title.elapsed() > Duration::from_millis(500) {
                canvas
                    .window_mut()
                    .set_title(&format!(
                        "ps-golem Rust {mode} | field {field} | F1 工具列 | F2 暫停"
                    ))
                    .map_err(|e| e.to_string())?;
                last_title = Instant::now();
            }
            if done && latest.as_ref().is_none_or(|f| shown == f.field) {
                break;
            }
            std::thread::sleep(Duration::from_millis(16));
        }
        Ok(())
    })();
    {
        let mut s = shared.state.lock().unwrap();
        s.stop = true;
        s.pause = false;
        shared.wake.notify_all();
    }
    audio.wake.notify_all();
    let joined = thread.join().map_err(|_| "機器執行緒意外停止".to_string());
    if let Some(d) = &device {
        d.pause()
    }
    joined?;
    let mut state = shared.state.lock().unwrap();
    let mut report = state.report.take().unwrap_or(json!({}));
    report["presented_field"] = json!(shown);
    report["draws"] = json!(draws);
    report["sdl_renderer"] = json!(canvas.info().name);
    report["audio_open"] = json!(device.is_some());
    report["audio_consumed_i16"] = json!(audio.consumed.load(Ordering::Relaxed));
    report["audio_consumed_nonzero_i16"] = json!(audio.nonzero.load(Ordering::Relaxed));
    report["audio_underrun_i16"] = json!(audio.silence.load(Ordering::Relaxed));
    report["audio_queue_high_water_i16"] = json!(audio.high_water.load(Ordering::Relaxed));
    report["pause_frozen_gui_checks"] = json!(freeze_checks);
    report["captures"] = json!(captures);
    report["poster"] = json!({"id":prefs.choice.id,"soft":prefs.choice.soft,"zoom":prefs.choice.zoom,
        "path":prefs.pack,"settings":prefs.path,"pack_loaded":posters.is_some(),"error":poster_error,
        "name":posters.as_ref().map(|p|p.name(&prefs.choice.id))});
    report["poster_events"] = json!(poster_events);
    report["pause_sources"] = json!(pause_sources);
    report["poster_layout"] = poster_layout
        .map(|l| {
            json!({"canvas":[l.canvas.x(),l.canvas.y(),l.canvas.width(),l.canvas.height()],
        "game":[l.game.x(),l.game.y(),l.game.width(),l.game.height()],"zoomed":l.zoomed})
        })
        .unwrap_or(json!(null));
    report["worker_error"] = json!(state.error);
    report["gui_error"] = json!(gui_result.as_ref().err());
    if let Some(p) = &o.report {
        fs::write(
            p.with_extension("json"),
            serde_json::to_vec_pretty(&report).map_err(|e| e.to_string())?,
        )
        .map_err(|e| e.to_string())?;
    }
    if let Some(p) = &o.screenshot {
        if let Some(bytes) = screenshot {
            display::png(p, screenshot_size.0, screenshot_size.1, &bytes)?;
        }
    }
    eprintln!(
        "Rust 視窗結束：field={}，已呈現={}，原卡保存={}",
        report["field"], shown, report["card_save_ok"]
    );
    gui_result?;
    if let Some(e) = state.error.take() {
        return Err(e);
    }
    Ok(())
}

fn toolbar_visible(pinned: bool, x: i32, y: i32, w: u32) -> bool {
    pinned || (x >= 0 && (x as u32) < w && (0..32).contains(&y))
}
fn toolbar_button(
    c: &mut sdl2::render::Canvas<sdl2::video::Window>,
    font: &display::Font,
    t: &sdl2::render::Texture<'_>,
    x: i32,
    w: u32,
    label: &str,
    active: bool,
) -> Result<()> {
    c.set_draw_color(if active {
        Color::RGB(71, 123, 187)
    } else {
        Color::RGB(52, 65, 85)
    });
    c.fill_rect(sdl2::rect::Rect::new(x, 2, w, 28))?;
    font.draw(c, t, label, x + 8, 4)
}
fn poster_setup(o: &Options) -> Result<(preferences::Preferences, Option<poster::Pack>, String)> {
    let saved = preferences::Preferences::load(o.settings.clone())?;
    let mut next = saved.clone();
    if let Some(p) = &o.posters {
        next.set_pack(p.clone())?;
    }
    if let Some(id) = &o.poster {
        next.choice.id = id.clone();
    }
    if let Some(v) = o.poster_soft {
        next.choice.soft = v;
    }
    if let Some(v) = o.poster_zoom {
        next.choice.zoom = v;
    }
    next.choice.validate()?;
    let mut error = String::new();
    let pack = match next.pack_path().map(|p| poster::Pack::load(&p)).transpose() {
        Ok(p) => p,
        Err(e) => {
            error = format!("海報載入失敗：{e}");
            eprintln!("{error}");
            return Ok((saved, None, error));
        }
    };
    if o.posters.is_some()
        || o.poster.is_some()
        || o.poster_soft.is_some()
        || o.poster_zoom.is_some()
    {
        if let Err(e) = next.save(&next.choice) {
            error = format!("儲存失敗：{e}");
            eprintln!("{error}");
            drop(pack);
            let pack = saved.pack_path().and_then(|p| poster::Pack::load(&p).ok());
            return Ok((saved, pack, error));
        }
    }
    Ok((next, pack, error))
}
