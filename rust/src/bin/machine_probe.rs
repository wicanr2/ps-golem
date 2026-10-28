//! Minimal-machine validation CLI, 026 §7. Not a disc/game frontend.
use psgolem_core::machine::{Machine, SHELL_ENTRY};
use std::{env, fs};
fn main() -> Result<(), Box<dyn std::error::Error>> {
    let (mut bios, mut exe, mut tty, mut state) = (None, None, None, None);
    let mut cycles = 100000000u64;
    let mut args = env::args().skip(1);
    while let Some(arg) = args.next() {
        match arg.as_str() {
            "-bios" => bios = args.next(),
            "-exe" => exe = args.next(),
            "-tty" => tty = args.next(),
            "-state" => state = args.next(),
            "-cycles" => cycles = args.next().ok_or("Missing cycle count")?.parse()?,
            "-h" | "--help" => {
                println!("Rust M2 validation: -bios file [-exe public-test.exe] [-cycles N] [-tty file] [-state prefix]\nNo disc loading, audio output or player/HD frontend in this validation tool.");
                return Ok(());
            }
            _ => return Err(format!("Unknown option {arg}").into()),
        }
    }
    let bios = bios.ok_or("-bios is required")?;
    let mut m = Machine::new(&fs::read(bios)?)?;
    m.run_until_pc(SHELL_ENTRY, 400000000)?;
    eprintln!(
        "BIOS shell PC={:08x} cycles={} field={}",
        m.cpu.pc(),
        m.cycles(),
        m.field()
    );
    if let Some(path) = exe {
        m.sideload_exe(&fs::read(path)?)?;
    }
    m.run(cycles);
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
        fs::write(format!("{prefix}.json"),format!("{{\"scope\":\"Rust M2 validation, not gameplay\",\"cycles\":{},\"field\":{},\"gpr\":{:?},\"hi\":{},\"lo\":{},\"pc\":{},\"next_pc\":{},\"bdbt\":{},\"ld_which\":{},\"ld_value\":{},\"cop0\":{:?},\"now\":{},\"cost\":{},\"muldiv_done\":{},\"gte_done\":{},\"icache\":{:?},\"gte_read\":{:?}}}\n",m.cycles(),m.field(),s.gpr,s.hi,s.lo,s.pc,s.next_pc,s.bdbt,s.ld_which,s.ld_value,s.cop0,s.now,s.cost,s.muldiv_done,s.gte_done,cache,gte))?;
    }
    Ok(())
}
