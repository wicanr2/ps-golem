//! Rust native player window, spec026 section14. GPL-3.0-or-later.
//! SDL2 is a platform binding; CPU/GTE/GPU/SPU/CD/MDEC remain our Rust core.
mod display;
mod input;
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
}
fn options() -> Result<Option<Options>> {
    let mut o = Options {
        present: "auto".into(),
        ..Options::default()
    };
    let mut args = env::args().skip(1);
    while let Some(a) = args.next() {
        if a == "-h" || a == "--help" {
            println!("Rust 原版玩家視窗（HD、海報與完整設定面板尚未遷移）\npsgolem-player -native -bios FILE -cue FILE [-card FILE] [-resolution 1920x1080] [-mute] [-present auto|cpu|gpu]\n鍵盤同 Go 版；已連線標準手把；F1 暫停／續播；Esc 關窗保存原卡。\n診斷：-fields N [-press SCRIPT | -press-file FILE] [-report PREFIX] [-screenshot FILE] [-unpaced（限靜音有界診斷）]");
            return Ok(None);
        }
        if a == "-version" {
            println!("psgolem-player Rust original preview / spec026 section14");
            return Ok(None);
        }
        match a.as_str() {
            "-native" => o.native = true,
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
    if !o.native {
        return Err("這是原版視窗切片，請明確使用 -native；HD 偏好不會被改寫".into());
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
    m.insert_disc(disc);
    let script = o
        .script
        .as_ref()
        .map(|s| psgolem_core::input::parse(s))
        .transpose()?;
    let sdl = sdl2::init()?;
    let video = sdl.video()?;
    let controllers = sdl.game_controller()?;
    let (w, h) = o.size.unwrap_or((1280, 960));
    let window = video
        .window("ps-golem Rust 原版", w, h)
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
        "Rust 原版視窗：SDL呈現={}；PS1繪圖=原生Rust軟體GPU；記憶卡={}",
        canvas.info().name,
        card.display()
    );
    let font = display::Font::load()?;
    let tc = canvas.texture_creator();
    let font_texture = font.texture(&tc)?;
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
                        let mut s = shared.state.lock().unwrap();
                        s.pause = !s.pause;
                        shared.wake.notify_all();
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
                canvas.copy(&t, None, display::fit(size_w, size_h))?;
                shown = f.field;
            }
            if pause {
                canvas.set_draw_color(Color::RGBA(20, 20, 30, 235));
                canvas.set_blend_mode(sdl2::render::BlendMode::Blend);
                canvas.fill_rect(sdl2::rect::Rect::new(12, 12, (size_w - 24).min(900), 92))?;
                font.draw(
                    &mut canvas,
                    &font_texture,
                    if paused {
                        "暫停  F1 繼續  Esc 離開"
                    } else {
                        "等待場邊界暫停"
                    },
                    26,
                    24,
                )?;
                font.draw(
                    &mut canvas,
                    &font_texture,
                    &format!("field {field}  cycles {cycles}  input {mask:04X}"),
                    26,
                    55,
                )?;
            }
            if o.screenshot.is_some() && (done || pause) {
                screenshot = Some(canvas.read_pixels(None, PixelFormatEnum::RGBA32)?);
                screenshot_size = (size_w, size_h);
            }
            canvas.present();
            draws += 1;
            if last_title.elapsed() > Duration::from_millis(500) {
                canvas
                    .window_mut()
                    .set_title(&format!("ps-golem Rust 原版 | field {field} | F1 暫停"))
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
