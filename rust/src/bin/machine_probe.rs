//! Machine/disc validation CLI, 026 sections 7-9. No player/HD frontend.
use psgolem_core::machine::{Machine, SHELL_ENTRY};
use std::{env, fs};
fn main() -> Result<(), Box<dyn std::error::Error>> {
    let (mut bios, mut exe, mut tty, mut state) = (None, None, None, None);
    let (mut cue, mut stop_pc) = (None, None);
    let mut wav = None;
    let (mut press, mut press_file, mut card) = (None, None, None);
    let mut fields: Option<u64> = None;
    let mut cycles_explicit = false;
    let mut cycles = 100000000u64;
    let mut args = env::args().skip(1);
    while let Some(arg) = args.next() {
        match arg.as_str() {
            "-press" => press = Some(args.next().ok_or("Missing press script")?),
            "-press-file" => press_file = Some(args.next().ok_or("Missing press script file")?),
            "-card" => card = Some(args.next().ok_or("Missing card path")?),
            "-fields" => fields = Some(args.next().ok_or("Missing field count")?.parse()?),
            "-wav" => wav = args.next(),
            "-cue" => cue = args.next(),
            "-stop-pc" => {
                stop_pc = Some(u32::from_str_radix(
                    args.next().ok_or("Missing PC")?.trim_start_matches("0x"),
                    16,
                )?)
            }
            "-bios" => bios = args.next(),
            "-exe" => exe = args.next(),
            "-tty" => tty = args.next(),
            "-state" => state = args.next(),
            "-cycles" => {
                cycles = args.next().ok_or("Missing cycle count")?.parse()?;
                cycles_explicit = true;
            }
            "-h" | "--help" => {
                println!("Rust validation: -bios file [-exe public-test.exe | -cue disc.cue] [-stop-pc hex] [-cycles N] [-tty file] [-state prefix] [-wav output.wav] [-fields N] [-press script | -press-file file] [-card file]\nCD loading supported; PCM/WAV output supported; native MDEC/DMA0-1 supported; no player/HD frontend.");
                return Ok(());
            }
            _ => return Err(format!("Unknown option {arg}").into()),
        }
    }
    if cue.is_some() && exe.is_some() {
        return Err("-cue and -exe are mutually exclusive".into());
    }
    if press.is_some() && press_file.is_some() {
        return Err("-press and -press-file are mutually exclusive".into());
    }
    if fields.is_some() && stop_pc.is_some() {
        return Err("-fields and -stop-pc are mutually exclusive".into());
    }
    if fields == Some(0) {
        return Err("-fields must be positive".into());
    }
    if let Some(path) = press_file {
        press = Some(fs::read_to_string(path)?);
    }
    let presses = psgolem_core::input::parse(press.as_deref().unwrap_or(""))?;
    if !cycles_explicit {
        if let Some(n) = fields {
            cycles = n
                .checked_mul(1000000)
                .ok_or("Field cycle budget overflow")?;
        }
    }
    let bios = bios.ok_or("-bios is required")?;
    let mut m = Machine::new(&fs::read(bios)?)?;
    if let Some(path) = &card {
        m.bus.sio.cards[0] = Some(psgolem_core::cardfile::load_or_new(std::path::Path::new(
            path,
        ))?);
    }
    if let Some(path) = cue {
        let disc = psgolem_core::disc::Disc::open(path)?;
        if let Some(message) = &disc.fallback {
            eprintln!("{message}");
        }
        m.insert_disc(disc);
    } else {
        m.run_until_pc(SHELL_ENTRY, 400000000)?;
        if let Some(path) = exe {
            m.sideload_exe(&fs::read(path)?)?;
        }
    }
    let mut samples = Vec::new();
    let end = m.cycles().saturating_add(cycles);
    let mut field = m.field();
    if let Some(pad) = m.bus.sio.pads[0].as_mut() {
        pad.buttons = psgolem_core::input::at(&presses, field);
    }
    while m.cycles() < end
        && fields.is_none_or(|target| m.field() < target)
        && stop_pc.is_none_or(|pc| {
            psgolem_core::machine::physical(m.cpu.pc()) != psgolem_core::machine::physical(pc)
        })
    {
        m.step();
        if m.field() != field {
            if wav.is_some() {
                samples.extend(m.take_audio());
            }
            field = m.field();
            if let Some(pad) = m.bus.sio.pads[0].as_mut() {
                pad.buttons = psgolem_core::input::at(&presses, field);
            }
            if fields.is_some() && field % 600 == 0 {
                eprintln!("Rust replay field={field} cycles={}", m.cycles());
            }
        }
    }
    if fields.is_some_and(|target| m.field() < target) {
        return Err("Requested field was not reached within cycle limit".into());
    }
    if stop_pc.is_some_and(|pc| {
        psgolem_core::machine::physical(m.cpu.pc()) != psgolem_core::machine::physical(pc)
    }) {
        return Err("Requested PC was not reached within cycle limit".into());
    }
    if let Some(path) = wav {
        samples.extend(m.take_audio());
        let bytes = u32::try_from(samples.len().checked_mul(2).ok_or("WAV size overflow")?)?;
        let mut out = Vec::new();
        out.extend_from_slice(b"RIFF");
        out.extend_from_slice(
            &bytes
                .checked_add(36)
                .ok_or("WAV RIFF overflow")?
                .to_le_bytes(),
        );
        out.extend_from_slice(b"WAVEfmt ");
        out.extend_from_slice(&16u32.to_le_bytes());
        out.extend_from_slice(&1u16.to_le_bytes());
        out.extend_from_slice(&2u16.to_le_bytes());
        out.extend_from_slice(&44100u32.to_le_bytes());
        out.extend_from_slice(&176400u32.to_le_bytes());
        out.extend_from_slice(&4u16.to_le_bytes());
        out.extend_from_slice(&16u16.to_le_bytes());
        out.extend_from_slice(b"data");
        out.extend_from_slice(&bytes.to_le_bytes());
        for sample in samples {
            out.extend_from_slice(&sample.to_le_bytes());
        }
        fs::write(path, out)?;
    }
    eprintln!(
        "Rust validation PC={:08x} cycles={} field={}",
        m.cpu.pc(),
        m.cycles(),
        m.field()
    );
    if let Some(path) = tty {
        fs::write(path, m.tty())?;
    } else {
        print!("{}", String::from_utf8_lossy(m.tty()));
    }
    if let Some(prefix) = state {
        fs::write(format!("{prefix}.ram"), m.bus.ram())?;
        let raw: Vec<u8> = m
            .bus
            .gpu
            .vram()
            .iter()
            .flat_map(|v| v.to_le_bytes())
            .collect();
        fs::write(format!("{prefix}.vram"), raw)?;
        let frame = m.bus.gpu.screen();
        fs::write(
            format!("{prefix}-{}x{}.rgba", frame.width, frame.height),
            frame.rgba,
        )?;
        let s = &m.cpu.state;
        let gte: Vec<u32> = (0..64).map(|i| m.cpu.gte.read(i)).collect();
        let cache: Vec<[u32; 2]> = s.icache.iter().map(|l| [l.tag, l.valid as u32]).collect();
        fs::write(format!("{prefix}.json"),format!("{{\"scope\":\"Rust machine validation, not gameplay\",\"cycles\":{},\"field\":{},\"gpr\":{:?},\"hi\":{},\"lo\":{},\"pc\":{},\"next_pc\":{},\"bdbt\":{},\"ld_which\":{},\"ld_value\":{},\"cop0\":{:?},\"now\":{},\"cost\":{},\"muldiv_done\":{},\"gte_done\":{},\"icache\":{:?},\"gte_read\":{:?}}}\n",m.cycles(),m.field(),s.gpr,s.hi,s.lo,s.pc,s.next_pc,s.bdbt,s.ld_which,s.ld_value,s.cop0,s.now,s.cost,s.muldiv_done,s.gte_done,cache,gte))?;
    }
    if let Some(path) = card {
        let c = m.bus.sio.cards[0].as_mut().ok_or("Card disconnected")?;
        let saved = psgolem_core::cardfile::save_if_dirty(std::path::Path::new(&path), c)?;
        eprintln!("Rust card: writes={} saved={saved}", c.writes);
    }
    Ok(())
}
