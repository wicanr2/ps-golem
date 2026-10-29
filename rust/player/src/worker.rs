//! Single owner of the deterministic machine. SDL never changes guest state.
use psgolem_core::{cardfile, input, machine::Machine};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::{
    collections::VecDeque,
    path::PathBuf,
    sync::{
        atomic::{AtomicU64, Ordering},
        Arc, Condvar, Mutex,
    },
    time::{Duration, Instant},
};
pub const CLOCK: u64 = 33_868_800;
pub const AUDIO_CAP: usize = 44_100; // i16 values: 22050 stereo samples (0.5 s).
pub struct Image {
    pub field: u64,
    pub w: u32,
    pub h: u32,
    pub rgba: Vec<u8>,
}
#[derive(Default)]
pub struct State {
    pub buttons: u16,
    pub pause: bool,
    pub paused: bool,
    pub stop: bool,
    pub frame: Option<Image>,
    pub done: bool,
    pub error: Option<String>,
    pub report: Option<Value>,
    pub field: u64,
    pub cycles: u64,
    pub pause_count: u64,
    pub pause_cycles: u64,
}
#[derive(Default)]
pub struct Shared {
    pub state: Mutex<State>,
    pub wake: Condvar,
}
#[derive(Default)]
pub struct Audio {
    pub samples: Mutex<VecDeque<i16>>,
    pub wake: Condvar,
    pub consumed: AtomicU64,
    pub nonzero: AtomicU64,
    pub silence: AtomicU64,
    pub high_water: AtomicU64,
}
pub struct Output(pub Arc<Audio>);
impl sdl2::audio::AudioCallback for Output {
    type Channel = i16;
    fn callback(&mut self, out: &mut [i16]) {
        let mut q = self.0.samples.lock().unwrap();
        let n = out.len().min(q.len());
        let mut nz = 0;
        for v in &mut out[..n] {
            *v = q.pop_front().unwrap();
            if *v != 0 {
                nz += 1;
            }
        }
        out[n..].fill(0);
        self.0.consumed.fetch_add(n as u64, Ordering::Relaxed);
        self.0.nonzero.fetch_add(nz, Ordering::Relaxed);
        self.0
            .silence
            .fetch_add((out.len() - n) as u64, Ordering::Relaxed);
        self.0.wake.notify_all();
    }
}
pub fn hash(b: &[u8]) -> String {
    format!("{:x}", Sha256::digest(b))
}
pub struct Options {
    pub fields: Option<u64>,
    pub script: Option<Vec<input::Press>>,
    pub card: PathBuf,
    pub unpaced: bool,
    pub report: Option<PathBuf>,
}
fn save(m: &mut Machine, p: &std::path::Path) -> Result<(), String> {
    if let Some(c) = m.bus.sio.cards[0].as_mut() {
        cardfile::save_if_dirty(p, c).map_err(|e| format!("記憶卡保存失敗：{e}"))?;
    }
    Ok(())
}
pub fn run(mut m: Machine, shared: Arc<Shared>, audio: Option<Arc<Audio>>, opts: Options) {
    let started = Instant::now();
    let mut clock_base = Instant::now();
    let mut cycle_base = m.cycles();
    let mut pcm = Sha256::new();
    let (mut values, mut nz) = (0u64, 0u64);
    let (mut last_writes, mut changed) = (0, 0);
    let result = (|| -> Result<(), String> {
        loop {
            let mut s = shared.state.lock().unwrap();
            while s.pause && !s.stop {
                if !s.paused {
                    s.pause_count += 1;
                    s.pause_cycles = m.cycles();
                    s.paused = true;
                    shared.wake.notify_all();
                }
                s = shared.wake.wait(s).unwrap();
                clock_base = Instant::now();
                cycle_base = m.cycles();
            }
            s.paused = false;
            if s.stop {
                break;
            }
            let buttons = if let Some(p) = &opts.script {
                input::at(p, m.field())
            } else {
                s.buttons
            };
            drop(s);
            if let Some(p) = m.bus.sio.pads[0].as_mut() {
                p.buttons = buttons;
            }
            let old = m.field();
            let end = m.cycles() + 2_000_000;
            while m.field() == old {
                if m.cycles() >= end {
                    return Err("未在單場週期上限到達 VBlank".into());
                }
                m.step();
            }
            let samples = m.take_audio();
            for &v in &samples {
                pcm.update(v.to_le_bytes());
                if v != 0 {
                    nz += 1;
                }
            }
            values += samples.len() as u64;
            if let Some(a) = &audio {
                if samples.len() > AUDIO_CAP {
                    return Err("單場音訊超出佇列上限".into());
                }
                let mut q = a.samples.lock().unwrap();
                while q.len() + samples.len() > AUDIO_CAP {
                    if shared.state.lock().unwrap().stop {
                        break;
                    }
                    q = a.wake.wait_timeout(q, Duration::from_millis(4)).unwrap().0;
                }
                if !shared.state.lock().unwrap().stop {
                    q.extend(samples);
                    a.high_water.fetch_max(q.len() as u64, Ordering::Relaxed);
                }
            }
            let f = m.bus.gpu.screen();
            {
                let mut s = shared.state.lock().unwrap();
                s.field = m.field();
                s.cycles = m.cycles();
                s.frame = Some(Image {
                    field: m.field(),
                    w: f.width,
                    h: f.height,
                    rgba: f.rgba,
                });
            }
            shared.wake.notify_all();
            if let Some(c) = m.bus.sio.cards[0].as_ref() {
                if c.writes != last_writes {
                    last_writes = c.writes;
                    changed = m.field();
                }
                if c.dirty && m.field().saturating_sub(changed) >= 120 {
                    save(&mut m, &opts.card)?;
                }
            }
            if opts.fields.is_some_and(|v| m.field() >= v) {
                break;
            }
            if !opts.unpaced {
                let due = Duration::from_secs_f64((m.cycles() - cycle_base) as f64 / CLOCK as f64);
                let elapsed = clock_base.elapsed();
                if elapsed > due + Duration::from_millis(100) {
                    clock_base = Instant::now();
                    cycle_base = m.cycles();
                } else if due > elapsed {
                    let s = shared.state.lock().unwrap();
                    if !s.stop && !s.pause {
                        drop(shared.wake.wait_timeout(s, due - elapsed).unwrap());
                    }
                }
            }
        }
        Ok(())
    })();
    let saved = save(&mut m, &opts.card);
    let vram: Vec<u8> = m
        .bus
        .gpu
        .vram()
        .iter()
        .flat_map(|v| v.to_le_bytes())
        .collect();
    let frame = m.bus.gpu.screen();
    let mut report = json!({"scope":"Rust original player window; not HD or physical audio/controller acceptance",
        "field":m.field(),"cycles":m.cycles(),"pc":m.cpu.pc(),"ram_sha256":hash(m.bus.ram()),
        "vram_sha256":hash(&vram),"pcm_sha256":format!("{:x}",pcm.finalize()),"pcm_i16_values":values,
        "pcm_nonzero_values":nz,"rgba_sha256":hash(&frame.rgba),"native_width":frame.width,"native_height":frame.height,
        "elapsed_seconds":started.elapsed().as_secs_f64(),"unpaced":opts.unpaced,
        "card_sha256":m.bus.sio.cards[0].as_ref().map(|c|hash(&c.data[..])),
        "card_writes":m.bus.sio.cards[0].as_ref().map(|c|c.writes),"card_save_ok":saved.is_ok()});
    let screenshot = if let Some(p) = &opts.report {
        let p = p.with_extension("native.png");
        super::display::png(&p, frame.width, frame.height, &frame.rgba)
    } else {
        Ok(())
    };
    let mut s = shared.state.lock().unwrap();
    report["pause_count"] = json!(s.pause_count);
    report["pause_cycle"] = json!(s.pause_cycles);
    s.error = result.err().or(saved.err()).or(screenshot.err());
    s.report = Some(report);
    s.done = true;
    shared.wake.notify_all();
}
